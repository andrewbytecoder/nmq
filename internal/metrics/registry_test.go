package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryGinMiddlewareAndHandler(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	registry := New(Config{
		Prefix:                 "demo",
		HeaderLabels:           map[string]string{"tenant": "X-Tenant"},
		RegisterGoMetrics:      false,
		RegisterProcessMetrics: false,
	})

	engine := gin.New()
	engine.Use(registry.GinMiddleware(GinMiddlewareOptions{
		ShouldObserve: func(c *gin.Context) bool {
			return c.FullPath() != "/metrics"
		},
	}))
	engine.GET("/users/:id", func(c *gin.Context) {
		c.String(http.StatusCreated, "created")
	})
	engine.GET("/metrics", registry.GinHandler())
	registry.SyncGinRoutes(engine)

	request := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	request.Header.Set("X-Tenant", "team-a")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)

	metricsRequest := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRecorder := httptest.NewRecorder()
	engine.ServeHTTP(metricsRecorder, metricsRequest)

	require.Equal(t, http.StatusOK, metricsRecorder.Code)

	body, err := io.ReadAll(metricsRecorder.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "demo_http_requests_total")

	families, err := registry.PrometheusRegistry().Gather()
	require.NoError(t, err)

	family := findMetricFamily(t, families, "demo_http_requests_total")
	metric := findMetricByLabels(t, family,
		"code", "201",
		"method", http.MethodGet,
		"protocol", "http",
		"route", "/users/:id",
		"tenant", "team-a",
	)

	require.NotNil(t, metric.Counter)
	assert.Equal(t, 1.0, metric.Counter.GetValue())
}

func TestRegistryDeletesStaleSeriesAfterScrape(t *testing.T) {
	registry := New(Config{
		Prefix:                 "cleanup",
		RegisterGoMetrics:      false,
		RegisterProcessMetrics: false,
	})

	registry.UpdateResources(ResourceSnapshot{
		Routes:   []string{"/old"},
		Services: []string{"orders"},
		BackendURLs: map[string][]string{
			"orders": {"http://127.0.0.1:9000"},
		},
	})

	registry.ObserveServerRequest(ServerRequestObservation{
		Route:         "/old",
		Method:        http.MethodGet,
		Protocol:      "http",
		StatusCode:    http.StatusOK,
		Duration:      50 * time.Millisecond,
		RequestBytes:  100,
		ResponseBytes: 200,
	})
	registry.ObserveBackendRequest(BackendRequestObservation{
		Service:       "orders",
		Method:        http.MethodGet,
		Protocol:      "http",
		StatusCode:    http.StatusOK,
		Duration:      10 * time.Millisecond,
		RequestBytes:  50,
		ResponseBytes: 80,
	})
	registry.SetBackendServerUp("orders", "http://127.0.0.1:9000", true)

	registry.UpdateResources(ResourceSnapshot{})

	firstFamilies, err := registry.PrometheusRegistry().Gather()
	require.NoError(t, err)

	require.NotNil(t, findMetricByLabels(t, findMetricFamily(t, firstFamilies, "cleanup_http_requests_total"),
		"code", "200",
		"method", http.MethodGet,
		"protocol", "http",
		"route", "/old",
	))
	require.NotNil(t, findMetricByLabels(t, findMetricFamily(t, firstFamilies, "cleanup_backend_server_up"),
		"service", "orders",
		"url", "http://127.0.0.1:9000",
	))

	secondFamilies, err := registry.PrometheusRegistry().Gather()
	require.NoError(t, err)

	httpRequestsFamily := findMetricFamilyLoose(secondFamilies, "cleanup_http_requests_total")
	if httpRequestsFamily != nil {
		assert.Nil(t, findMetricByLabelsLoose(httpRequestsFamily,
			"code", "200",
			"method", http.MethodGet,
			"protocol", "http",
			"route", "/old",
		))
	}

	backendUpFamily := findMetricFamilyLoose(secondFamilies, "cleanup_backend_server_up")
	if backendUpFamily != nil {
		assert.Nil(t, findMetricByLabelsLoose(backendUpFamily,
			"service", "orders",
			"url", "http://127.0.0.1:9000",
		))
	}
}

func TestRegistryAllowsRuntimeCustomMetrics(t *testing.T) {
	registry := New(Config{
		Prefix:                 "runtime",
		RegisterGoMetrics:      false,
		RegisterProcessMetrics: false,
	})

	queueJobs := registry.NewCounterVec(prometheus.CounterOpts{
		Name: "queue_jobs_total",
		Help: "Queued jobs.",
	}, []string{"queue"})
	queueJobs.WithLabelValues("default").Add(2)

	customWorkers := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: registry.MetricName("workers_inflight"),
		Help: "Inflight workers.",
	}, []string{"worker"})
	registry.MustRegisterCollectors(customWorkers)
	customWorkers.WithLabelValues("sync").Set(3)

	families, err := registry.PrometheusRegistry().Gather()
	require.NoError(t, err)

	queueFamily := findMetricFamily(t, families, "runtime_queue_jobs_total")
	queueMetric := findMetricByLabels(t, queueFamily, "queue", "default")
	require.NotNil(t, queueMetric.Counter)
	assert.Equal(t, 2.0, queueMetric.Counter.GetValue())

	workerFamily := findMetricFamily(t, families, "runtime_workers_inflight")
	workerMetric := findMetricByLabels(t, workerFamily, "worker", "sync")
	require.NotNil(t, workerMetric.Gauge)
	assert.Equal(t, 3.0, workerMetric.Gauge.GetValue())
}

func TestRegistryAllowsRuntimeCustomNonVecMetrics(t *testing.T) {
	registry := New(Config{
		Prefix:                 "plain",
		RegisterGoMetrics:      false,
		RegisterProcessMetrics: false,
	})

	totalJobs := registry.NewCounter(prometheus.CounterOpts{
		Name: "jobs_total",
		Help: "Processed jobs.",
	})
	totalJobs.Add(5)

	inFlight := registry.NewGauge(prometheus.GaugeOpts{
		Name: "workers_inflight",
		Help: "Inflight workers.",
	})
	inFlight.Set(3)

	latency := registry.NewHistogram(prometheus.HistogramOpts{
		Name: "job_duration_seconds",
		Help: "Job duration.",
	})
	latency.Observe(0.2)

	summary := registry.NewSummary(prometheus.SummaryOpts{
		Name: "job_payload_bytes",
		Help: "Payload size.",
	})
	summary.Observe(128)

	families, err := registry.PrometheusRegistry().Gather()
	require.NoError(t, err)

	counterFamily := findMetricFamily(t, families, "plain_jobs_total")
	require.Len(t, counterFamily.Metric, 1)
	require.NotNil(t, counterFamily.Metric[0].Counter)
	assert.Equal(t, 5.0, counterFamily.Metric[0].Counter.GetValue())

	gaugeFamily := findMetricFamily(t, families, "plain_workers_inflight")
	require.Len(t, gaugeFamily.Metric, 1)
	require.NotNil(t, gaugeFamily.Metric[0].Gauge)
	assert.Equal(t, 3.0, gaugeFamily.Metric[0].Gauge.GetValue())

	histogramFamily := findMetricFamily(t, families, "plain_job_duration_seconds")
	require.Len(t, histogramFamily.Metric, 1)
	require.NotNil(t, histogramFamily.Metric[0].Histogram)
	assert.Equal(t, uint64(1), histogramFamily.Metric[0].Histogram.GetSampleCount())

	summaryFamily := findMetricFamily(t, families, "plain_job_payload_bytes")
	require.Len(t, summaryFamily.Metric, 1)
	require.NotNil(t, summaryFamily.Metric[0].Summary)
	assert.Equal(t, uint64(1), summaryFamily.Metric[0].Summary.GetSampleCount())
}

func TestRegistryManagedCustomMetricsParticipateInResourceCleanup(t *testing.T) {
	registry := New(Config{
		Prefix:                 "managed",
		RegisterGoMetrics:      false,
		RegisterProcessMetrics: false,
	})

	routeGauge := registry.NewManagedGaugeVec(prometheus.GaugeOpts{
		Name: "route_workers",
		Help: "Workers per route.",
	}, []string{"endpoint", "pool"}, ResourceLabelNames{
		Route: "endpoint",
	})
	backendGauge := registry.NewManagedGaugeVec(prometheus.GaugeOpts{
		Name: "backend_workers",
		Help: "Workers per backend URL.",
	}, []string{"service_name", "backend_url"}, ResourceLabelNames{
		Service:    "service_name",
		BackendURL: "backend_url",
	})

	registry.UpdateResources(ResourceSnapshot{
		Routes:   []string{"/jobs"},
		Services: []string{"inventory"},
		BackendURLs: map[string][]string{
			"inventory": {"http://inventory.default.svc.cluster.local"},
		},
	})

	routeGauge.WithLabelValues("/jobs", "fast").Set(2)
	backendGauge.WithLabelValues("inventory", "http://inventory.default.svc.cluster.local").Set(4)

	firstFamilies, err := registry.PrometheusRegistry().Gather()
	require.NoError(t, err)

	require.NotNil(t, findMetricByLabels(t, findMetricFamily(t, firstFamilies, "managed_route_workers"),
		"endpoint", "/jobs",
		"pool", "fast",
	))
	require.NotNil(t, findMetricByLabels(t, findMetricFamily(t, firstFamilies, "managed_backend_workers"),
		"service_name", "inventory",
		"backend_url", "http://inventory.default.svc.cluster.local",
	))

	registry.UpdateResources(ResourceSnapshot{})

	secondFamilies, err := registry.PrometheusRegistry().Gather()
	require.NoError(t, err)

	require.NotNil(t, findMetricByLabels(t, findMetricFamily(t, secondFamilies, "managed_route_workers"),
		"endpoint", "/jobs",
		"pool", "fast",
	))
	require.NotNil(t, findMetricByLabels(t, findMetricFamily(t, secondFamilies, "managed_backend_workers"),
		"service_name", "inventory",
		"backend_url", "http://inventory.default.svc.cluster.local",
	))

	thirdFamilies, err := registry.PrometheusRegistry().Gather()
	require.NoError(t, err)

	routeFamily := findMetricFamilyLoose(thirdFamilies, "managed_route_workers")
	if routeFamily != nil {
		assert.Nil(t, findMetricByLabelsLoose(routeFamily,
			"endpoint", "/jobs",
			"pool", "fast",
		))
	}

	backendFamily := findMetricFamilyLoose(thirdFamilies, "managed_backend_workers")
	if backendFamily != nil {
		assert.Nil(t, findMetricByLabelsLoose(backendFamily,
			"service_name", "inventory",
			"backend_url", "http://inventory.default.svc.cluster.local",
		))
	}
}

func TestRegisterManagedCollectorRequiresServiceLabelForBackendURLCleanup(t *testing.T) {
	registry := New(Config{
		Prefix:                 "validate",
		RegisterGoMetrics:      false,
		RegisterProcessMetrics: false,
	})

	collector := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: registry.MetricName("backend_only"),
		Help: "Invalid cleanup mapping.",
	}, []string{"backend_url"})

	err := registry.RegisterManagedCollector(collector, ResourceLabelNames{
		BackendURL: "backend_url",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service label")
}

func findMetricFamily(t *testing.T, families []*dto.MetricFamily, name string) *dto.MetricFamily {
	t.Helper()

	family := findMetricFamilyLoose(families, name)
	if family != nil {
		return family
	}

	t.Fatalf("metric family %s not found", name)
	return nil
}

func findMetricFamilyLoose(families []*dto.MetricFamily, name string) *dto.MetricFamily {
	for _, family := range families {
		if strings.EqualFold(family.GetName(), name) {
			return family
		}
	}

	return nil
}

func findMetricByLabels(t *testing.T, family *dto.MetricFamily, labelNamesValues ...string) *dto.Metric {
	t.Helper()

	metric := findMetricByLabelsLoose(family, labelNamesValues...)
	require.NotNil(t, metric)
	return metric
}

func findMetricByLabelsLoose(family *dto.MetricFamily, labelNamesValues ...string) *dto.Metric {
	for _, metric := range family.Metric {
		if hasAllLabels(metric, labelNamesValues...) {
			return metric
		}
	}

	return nil
}

func hasAllLabels(metric *dto.Metric, labelNamesValues ...string) bool {
	for i := 0; i < len(labelNamesValues); i += 2 {
		if !hasLabel(metric, labelNamesValues[i], labelNamesValues[i+1]) {
			return false
		}
	}

	return true
}

func hasLabel(metric *dto.Metric, name, value string) bool {
	for _, label := range metric.Label {
		if label.GetName() == name && label.GetValue() == value {
			return true
		}
	}

	return false
}
