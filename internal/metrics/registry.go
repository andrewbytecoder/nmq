package metrics

import (
	"crypto/tls"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const unknownRoute = "unmatched"

// ServerRequestObservation describes one inbound HTTP request that should be
// recorded in the built-in server-side metrics.
type ServerRequestObservation struct {
	// Headers is used to resolve configured HeaderLabels.
	Headers http.Header
	// Route is the logical route identifier, usually the Gin full path.
	Route string
	// Method is the HTTP method, such as GET or POST.
	Method string
	// Protocol is an application-level protocol label, typically http or https.
	Protocol string
	// StatusCode is the final HTTP response status code.
	StatusCode int
	// Duration is the total request handling latency.
	Duration time.Duration
	// RequestBytes is the inbound request size in bytes.
	RequestBytes int64
	// ResponseBytes is the outbound response size in bytes.
	ResponseBytes int64
	// TLSVersion is an optional TLS version label for TLS-only counters.
	TLSVersion string
	// TLSCipher is an optional TLS cipher label for TLS-only counters.
	TLSCipher string
}

// BackendRequestObservation describes one outbound request to a backend service
// that should be recorded in the built-in backend metrics.
type BackendRequestObservation struct {
	// Service is the logical backend service name.
	Service string
	// Method is the outbound HTTP method.
	Method string
	// Protocol is an application-level protocol label, typically http or https.
	Protocol string
	// StatusCode is the backend response status code.
	StatusCode int
	// Duration is the backend request latency.
	Duration time.Duration
	// RequestBytes is the number of bytes sent to the backend.
	RequestBytes int64
	// ResponseBytes is the number of bytes received from the backend.
	ResponseBytes int64
	// TLSVersion is an optional TLS version label for TLS-only counters.
	TLSVersion string
	// TLSCipher is an optional TLS cipher label for TLS-only counters.
	TLSCipher string
}

// GinMiddlewareOptions customizes how Gin requests are turned into metric label
// values.
type GinMiddlewareOptions struct {
	// RouteLabel overrides the default route label resolution. When nil, the Gin
	// full path is used and falls back to "unmatched".
	RouteLabel func(*gin.Context) string
	// ProtocolLabel overrides the default protocol label resolution. When nil,
	// requests are labeled as https when TLS is present and http otherwise.
	ProtocolLabel func(*gin.Context) string
	// ShouldObserve decides whether the current request should be recorded. It is
	// commonly used to exclude the /metrics endpoint from self-observation.
	ShouldObserve func(*gin.Context) bool
}

type vector interface {
	prometheus.Collector
	DeletePartialMatch(prometheus.Labels) int
}

type trackedVector struct {
	collector      vector
	resourceLabels ResourceLabelNames
}

// deleteListener removes listener-scoped time series for this collector only
// when the caller mapped a listener label for it.
func (t trackedVector) deleteListener(listener string) int {
	if t.resourceLabels.Listener == "" {
		return 0
	}

	return t.collector.DeletePartialMatch(prometheus.Labels{t.resourceLabels.Listener: listener})
}

// deleteRoute removes route-scoped time series for this collector only when the
// caller mapped a route label for it.
func (t trackedVector) deleteRoute(route string) int {
	if t.resourceLabels.Route == "" {
		return 0
	}

	return t.collector.DeletePartialMatch(prometheus.Labels{t.resourceLabels.Route: route})
}

// deleteService removes service-scoped time series for this collector only
// when the caller mapped a service label for it.
func (t trackedVector) deleteService(service string) int {
	if t.resourceLabels.Service == "" {
		return 0
	}

	return t.collector.DeletePartialMatch(prometheus.Labels{t.resourceLabels.Service: service})
}

// deleteBackendURL removes backend URL-scoped time series. Service and backend
// URL must both be present so that URL reuse across services does not delete the
// wrong series.
func (t trackedVector) deleteBackendURL(service, url string) int {
	if t.resourceLabels.Service == "" || t.resourceLabels.BackendURL == "" {
		return 0
	}

	return t.collector.DeletePartialMatch(prometheus.Labels{
		t.resourceLabels.Service:    service,
		t.resourceLabels.BackendURL: url,
	})
}

// Registry is the reusable Prometheus-backed metrics facade exposed by this
// package.
//
// It owns one isolated Prometheus registry, the built-in Gin-oriented metrics,
// helper constructors for runtime metrics, and the topology-aware cleanup state
// used by UpdateResources.
type Registry struct {
	config       Config
	promRegistry *prometheus.Registry
	state        *collectorState

	configReloads           *prometheus.CounterVec
	lastConfigReloadSuccess *prometheus.GaugeVec
	openConnections         *prometheus.GaugeVec
	tlsCertsNotAfter        *prometheus.GaugeVec

	serverRequests         *prometheus.CounterVec
	serverRequestsTLS      *prometheus.CounterVec
	serverRequestDuration  *prometheus.HistogramVec
	serverRequestsBytes    *prometheus.CounterVec
	serverResponsesBytes   *prometheus.CounterVec
	backendRequests        *prometheus.CounterVec
	backendRequestsTLS     *prometheus.CounterVec
	backendRequestDuration *prometheus.HistogramVec
	backendRetries         *prometheus.CounterVec
	backendServerUp        *prometheus.GaugeVec
	backendRequestsBytes   *prometheus.CounterVec
	backendResponsesBytes  *prometheus.CounterVec
}

// New creates a standalone Registry with built-in HTTP/server/backend metrics.
//
// The returned Registry is ready to expose /metrics, record Gin traffic, accept
// runtime custom collectors, and clean stale series when UpdateResources is
// called with the latest topology snapshot.
func New(config Config) *Registry {
	config = config.withDefaults()

	registry := &Registry{
		config:       config,
		promRegistry: prometheus.NewRegistry(),
		state:        newCollectorState(),
	}

	headerLabels := sortedHeaderLabels(config.HeaderLabels)

	registry.configReloads = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: metricName(config.Prefix, "config_reloads_total"),
		Help: "Total number of successful and failed config reload attempts.",
	}, nil)
	registry.lastConfigReloadSuccess = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricName(config.Prefix, "config_last_reload_success"),
		Help: "Whether the last config reload succeeded.",
	}, nil)

	registry.state.addTrackedVectors(
		trackedVector{collector: registry.configReloads},
		trackedVector{collector: registry.lastConfigReloadSuccess},
	)

	registry.configReloads.WithLabelValues().Add(0)
	registry.lastConfigReloadSuccess.WithLabelValues().Set(0)

	// 基础服务信息注册
	if config.RegisterServerMetrics {
		registry.openConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: metricName(config.Prefix, "open_connections"),
			Help: "Number of currently open connections grouped by listener and protocol.",
		}, []string{"listener", "protocol"})
		registry.tlsCertsNotAfter = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: metricName(config.Prefix, "tls_certs_not_after"),
			Help: "TLS certificate expiration timestamps.",
		}, []string{"cn", "serial", "sans"})

		serverRequestLabels := append([]string{"code", "method", "protocol", "route"}, headerLabels...)
		registry.serverRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricName(config.Prefix, "http_requests_total"),
			Help: "Total number of HTTP requests handled by the server.",
		}, serverRequestLabels)
		registry.serverRequestsTLS = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricName(config.Prefix, "http_requests_tls_total"),
			Help: "Total number of HTTP requests served over TLS.",
		}, []string{"route", "tls_version", "tls_cipher"})
		registry.serverRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    metricName(config.Prefix, "http_request_duration_seconds"),
			Help:    "Duration of HTTP requests handled by the server.",
			Buckets: config.Buckets,
		}, []string{"code", "method", "protocol", "route"})
		registry.serverRequestsBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricName(config.Prefix, "http_requests_bytes_total"),
			Help: "Total number of request bytes handled by the server.",
		}, []string{"code", "method", "protocol", "route"})
		registry.serverResponsesBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricName(config.Prefix, "http_responses_bytes_total"),
			Help: "Total number of response bytes handled by the server.",
		}, []string{"code", "method", "protocol", "route"})

		registry.backendRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricName(config.Prefix, "backend_requests_total"),
			Help: "Total number of outbound backend requests.",
		}, []string{"code", "method", "protocol", "service"})
		registry.backendRequestsTLS = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricName(config.Prefix, "backend_requests_tls_total"),
			Help: "Total number of outbound backend requests served over TLS.",
		}, []string{"service", "tls_version", "tls_cipher"})
		registry.backendRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    metricName(config.Prefix, "backend_request_duration_seconds"),
			Help:    "Duration of outbound backend requests.",
			Buckets: config.Buckets,
		}, []string{"code", "method", "protocol", "service"})
		registry.backendRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricName(config.Prefix, "backend_retries_total"),
			Help: "Total number of backend retries.",
		}, []string{"service"})
		registry.backendServerUp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: metricName(config.Prefix, "backend_server_up"),
			Help: "Whether a backend server is considered up.",
		}, []string{"service", "url"})
		registry.backendRequestsBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricName(config.Prefix, "backend_requests_bytes_total"),
			Help: "Total number of request bytes sent to backends.",
		}, []string{"code", "method", "protocol", "service"})
		registry.backendResponsesBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricName(config.Prefix, "backend_responses_bytes_total"),
			Help: "Total number of response bytes received from backends.",
		}, []string{"code", "method", "protocol", "service"})

		registry.state.addTrackedVectors(
			trackedVector{collector: registry.openConnections, resourceLabels: ResourceLabelNames{Listener: "listener"}},
			trackedVector{collector: registry.tlsCertsNotAfter},
			trackedVector{collector: registry.serverRequests, resourceLabels: ResourceLabelNames{Route: "route"}},
			trackedVector{collector: registry.serverRequestsTLS, resourceLabels: ResourceLabelNames{Route: "route"}},
			trackedVector{collector: registry.serverRequestDuration, resourceLabels: ResourceLabelNames{Route: "route"}},
			trackedVector{collector: registry.serverRequestsBytes, resourceLabels: ResourceLabelNames{Route: "route"}},
			trackedVector{collector: registry.serverResponsesBytes, resourceLabels: ResourceLabelNames{Route: "route"}},
			trackedVector{collector: registry.backendRequests, resourceLabels: ResourceLabelNames{Service: "service"}},
			trackedVector{collector: registry.backendRequestsTLS, resourceLabels: ResourceLabelNames{Service: "service"}},
			trackedVector{collector: registry.backendRequestDuration, resourceLabels: ResourceLabelNames{Service: "service"}},
			trackedVector{collector: registry.backendRetries, resourceLabels: ResourceLabelNames{Service: "service"}},
			trackedVector{collector: registry.backendServerUp, resourceLabels: ResourceLabelNames{Service: "service", BackendURL: "url"}},
			trackedVector{collector: registry.backendRequestsBytes, resourceLabels: ResourceLabelNames{Service: "service"}},
			trackedVector{collector: registry.backendResponsesBytes, resourceLabels: ResourceLabelNames{Service: "service"}},
		)
	}

	registry.promRegistry.MustRegister(registry.state)

	if config.RegisterProcessMetrics {
		registry.promRegistry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}

	if config.RegisterGoMetrics {
		registry.promRegistry.MustRegister(collectors.NewGoCollector())
	}

	return registry
}

// Handler returns a standard net/http Prometheus handler backed by this
// registry's isolated collector set.
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.promRegistry, promhttp.HandlerOpts{})
}

// GinHandler adapts Handler into a Gin handler function and aborts the Gin
// chain after serving the metrics response.
func (r *Registry) GinHandler() gin.HandlerFunc {
	handler := r.Handler()

	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

// PrometheusRegistry exposes the underlying Prometheus registry for advanced
// integrations, tests, and manual gather operations.
func (r *Registry) PrometheusRegistry() *prometheus.Registry {
	return r.promRegistry
}

// MarkConfigReload records a configuration reload attempt and whether the last
// attempt succeeded.
func (r *Registry) MarkConfigReload(success bool) {
	r.configReloads.WithLabelValues().Inc()
	if success {
		r.lastConfigReloadSuccess.WithLabelValues().Set(1)
		return
	}

	r.lastConfigReloadSuccess.WithLabelValues().Set(0)
}

// AddOpenConnections adjusts the open-connections gauge for a listener and
// protocol by delta.
func (r *Registry) AddOpenConnections(listener, protocol string, delta float64) {
	r.openConnections.WithLabelValues(listener, normalizeProtocol(protocol)).Add(delta)
}

// SetOpenConnections sets the open-connections gauge for a listener and
// protocol to an absolute value.
func (r *Registry) SetOpenConnections(listener, protocol string, value float64) {
	r.openConnections.WithLabelValues(listener, normalizeProtocol(protocol)).Set(value)
}

// SetTLSCertNotAfter records the expiration timestamp of a TLS certificate.
func (r *Registry) SetTLSCertNotAfter(cn, serial, sans string, notAfter time.Time) {
	r.tlsCertsNotAfter.WithLabelValues(cn, serial, sans).Set(float64(notAfter.Unix()))
}

// AddBackendRetry increments the backend retry counter for the given service.
func (r *Registry) AddBackendRetry(service string, delta float64) {
	r.backendRetries.WithLabelValues(normalizeService(service)).Add(delta)
}

// SetBackendServerUp updates the health gauge for one backend target.
func (r *Registry) SetBackendServerUp(service, url string, up bool) {
	value := 0.0
	if up {
		value = 1
	}

	r.backendServerUp.WithLabelValues(normalizeService(service), url).Set(value)
}

// ObserveServerRequest records one inbound request into the built-in
// server-facing metrics.
func (r *Registry) ObserveServerRequest(observation ServerRequestObservation) {
	route := normalizeRoute(observation.Route)
	labels := prometheus.Labels{
		"code":     normalizeStatusCode(observation.StatusCode),
		"method":   normalizeMethod(observation.Method),
		"protocol": normalizeProtocol(observation.Protocol),
		"route":    route,
	}

	for labelName, headerName := range r.config.HeaderLabels {
		labels[labelName] = ""
		if observation.Headers != nil {
			labels[labelName] = observation.Headers.Get(headerName)
		}
	}

	r.serverRequests.With(labels).Inc()
	r.serverRequestDuration.With(baseRequestLabels(labels)).Observe(observation.Duration.Seconds())
	r.serverRequestsBytes.With(baseRequestLabels(labels)).Add(nonNegativeFloat(observation.RequestBytes))
	r.serverResponsesBytes.With(baseRequestLabels(labels)).Add(nonNegativeFloat(observation.ResponseBytes))

	if observation.TLSVersion == "" && observation.TLSCipher == "" {
		return
	}

	r.serverRequestsTLS.WithLabelValues(route, normalizeValue(observation.TLSVersion), normalizeValue(observation.TLSCipher)).Inc()
}

// ObserveBackendRequest records one outbound request into the built-in
// backend-facing metrics.
func (r *Registry) ObserveBackendRequest(observation BackendRequestObservation) {
	service := normalizeService(observation.Service)
	labels := prometheus.Labels{
		"code":     normalizeStatusCode(observation.StatusCode),
		"method":   normalizeMethod(observation.Method),
		"protocol": normalizeProtocol(observation.Protocol),
		"service":  service,
	}

	r.backendRequests.With(labels).Inc()
	r.backendRequestDuration.With(labels).Observe(observation.Duration.Seconds())
	r.backendRequestsBytes.With(labels).Add(nonNegativeFloat(observation.RequestBytes))
	r.backendResponsesBytes.With(labels).Add(nonNegativeFloat(observation.ResponseBytes))

	if observation.TLSVersion == "" && observation.TLSCipher == "" {
		return
	}

	r.backendRequestsTLS.WithLabelValues(service, normalizeValue(observation.TLSVersion), normalizeValue(observation.TLSCipher)).Inc()
}

// UpdateResources publishes the latest active topology to the cleanup engine.
//
// Time series for removed resources are not deleted immediately. They remain
// visible for the next scrape and are then removed, matching the same scrape-
// safe cleanup strategy used by Traefik.
func (r *Registry) UpdateResources(snapshot ResourceSnapshot) {
	r.state.SetResources(snapshot)
}

// SyncGinRoutes snapshots the currently registered Gin routes into the cleanup
// topology so route-scoped metrics can be deleted when routes disappear.
func (r *Registry) SyncGinRoutes(engine *gin.Engine) {
	snapshot := r.state.Snapshot()
	snapshot.Routes = snapshot.Routes[:0]

	for _, route := range engine.Routes() {
		if route.Path == "" {
			continue
		}
		snapshot.Routes = append(snapshot.Routes, route.Path)
	}

	r.UpdateResources(snapshot)
}

// GinMiddleware records built-in HTTP request metrics for Gin handlers.
//
// It captures request count, latency, request bytes, response bytes, and
// optional TLS labels. The middleware is intentionally generic so callers can
// decide how routes and protocols should be labeled.
func (r *Registry) GinMiddleware(options GinMiddlewareOptions) gin.HandlerFunc {
	routeLabel := options.RouteLabel
	if routeLabel == nil {
		routeLabel = func(c *gin.Context) string {
			if path := c.FullPath(); path != "" {
				return path
			}
			return unknownRoute
		}
	}

	protocolLabel := options.ProtocolLabel
	if protocolLabel == nil {
		protocolLabel = func(c *gin.Context) string {
			if c.Request.TLS != nil {
				return "https"
			}
			return "http"
		}
	}

	shouldObserve := options.ShouldObserve
	if shouldObserve == nil {
		shouldObserve = func(*gin.Context) bool { return true }
	}

	return func(c *gin.Context) {
		if !shouldObserve(c) {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		r.ObserveServerRequest(ServerRequestObservation{
			Headers:       c.Request.Header,
			Route:         routeLabel(c),
			Method:        c.Request.Method,
			Protocol:      protocolLabel(c),
			StatusCode:    c.Writer.Status(),
			Duration:      time.Since(start),
			RequestBytes:  requestSize(c.Request),
			ResponseBytes: int64(max(c.Writer.Size(), 0)),
			TLSVersion:    tlsVersion(c.Request.TLS),
			TLSCipher:     tlsCipher(c.Request.TLS),
		})
	}
}

// collectorState is the cleanup-aware collector multiplexer registered in the
// underlying Prometheus registry.
//
// It owns the tracked collectors, exposes them as a single Prometheus
// Collector, and performs post-scrape deletion of stale series once updated
// topology information has been published through UpdateResources.
type collectorState struct {
	vectors []trackedVector

	mu              sync.Mutex
	resources       *resourceState
	deletedListener []string
	deletedRoute    []string
	deletedService  []string
	deletedURLs     map[string][]string
}

// newCollectorState initializes an empty cleanup state.
func newCollectorState() *collectorState {
	return &collectorState{
		resources:   newResourceState(),
		deletedURLs: make(map[string][]string),
	}
}

// Snapshot returns the current topology view known to the cleanup engine.
func (s *collectorState) Snapshot() ResourceSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.resources.snapshot()
}

// addVectors adds cleanup-unaware tracked collectors. This is used for normal
// registrations where the collector participates in the scrape pipeline but not
// in resource-based stale-series deletion.
func (s *collectorState) addVectors(vectors ...vector) {
	tracked := make([]trackedVector, 0, len(vectors))
	for _, collector := range vectors {
		tracked = append(tracked, trackedVector{collector: collector})
	}

	s.addTrackedVectors(tracked...)
}

// addTrackedVectors adds collectors together with their resource cleanup
// mappings.
func (s *collectorState) addTrackedVectors(vectors ...trackedVector) {
	if len(vectors) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.vectors = append(s.vectors, vectors...)
}

// SetResources compares the new topology against the previous one and records
// which resources disappeared. The actual metric deletion is deferred until the
// next scrape to avoid losing the final visible sample prematurely.
func (s *collectorState) SetResources(snapshot ResourceSnapshot) {
	next := newResourceStateFromSnapshot(snapshot)

	s.mu.Lock()
	defer s.mu.Unlock()

	for listener := range s.resources.listeners {
		if !next.hasListener(listener) {
			s.deletedListener = append(s.deletedListener, listener)
		}
	}

	for route := range s.resources.routes {
		if !next.hasRoute(route) {
			s.deletedRoute = append(s.deletedRoute, route)
		}
	}

	for service, urls := range s.resources.services {
		if !next.hasService(service) {
			s.deletedService = append(s.deletedService, service)
		}

		for url := range urls {
			if !next.hasBackendURL(service, url) {
				s.deletedURLs[service] = append(s.deletedURLs[service], url)
			}
		}
	}

	s.resources = next
}

// Describe forwards Describe to all tracked collectors.
func (s *collectorState) Describe(ch chan<- *prometheus.Desc) {
	for _, v := range s.snapshotVectors() {
		v.collector.Describe(ch)
	}
}

// Collect forwards Collect to all tracked collectors and then deletes stale
// series marked by SetResources.
func (s *collectorState) Collect(ch chan<- prometheus.Metric) {
	for _, v := range s.snapshotVectors() {
		v.collector.Collect(ch)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, listener := range s.deletedListener {
		if !s.resources.hasListener(listener) {
			s.deleteListenerLocked(listener)
		}
	}

	for _, route := range s.deletedRoute {
		if !s.resources.hasRoute(route) {
			s.deleteRouteLocked(route)
		}
	}

	for _, service := range s.deletedService {
		if !s.resources.hasService(service) {
			s.deleteServiceLocked(service)
		}
	}

	for service, urls := range s.deletedURLs {
		for _, url := range urls {
			if !s.resources.hasBackendURL(service, url) {
				s.deleteBackendURLLocked(service, url)
			}
		}
	}

	s.deletedListener = nil
	s.deletedRoute = nil
	s.deletedService = nil
	s.deletedURLs = make(map[string][]string)
}

// DeletePartialMatch applies a raw label-based deletion to every tracked vector.
// It is primarily useful for tests and advanced callers.
func (s *collectorState) DeletePartialMatch(labels prometheus.Labels) int {
	snapshot := s.snapshotVectors()
	var deleted int
	for _, v := range snapshot {
		deleted += v.collector.DeletePartialMatch(labels)
	}
	return deleted
}

// deleteListenerLocked applies listener-based cleanup while the caller already
// holds the state lock.
func (s *collectorState) deleteListenerLocked(listener string) int {
	var deleted int
	for _, v := range s.vectors {
		deleted += v.deleteListener(listener)
	}
	return deleted
}

// deleteRouteLocked applies route-based cleanup while the caller already holds
// the state lock.
func (s *collectorState) deleteRouteLocked(route string) int {
	var deleted int
	for _, v := range s.vectors {
		deleted += v.deleteRoute(route)
	}
	return deleted
}

// deleteServiceLocked applies service-based cleanup while the caller already
// holds the state lock.
func (s *collectorState) deleteServiceLocked(service string) int {
	var deleted int
	for _, v := range s.vectors {
		deleted += v.deleteService(service)
	}
	return deleted
}

// deleteBackendURLLocked applies backend URL-based cleanup while the caller
// already holds the state lock.
func (s *collectorState) deleteBackendURLLocked(service, url string) int {
	var deleted int
	for _, v := range s.vectors {
		deleted += v.deleteBackendURL(service, url)
	}
	return deleted
}

// snapshotVectors copies the tracked collector list so scrape-time iteration can
// happen without holding the state lock for the entire collection path.
func (s *collectorState) snapshotVectors() []trackedVector {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.vectors) == 0 {
		return nil
	}

	vectors := make([]trackedVector, len(s.vectors))
	copy(vectors, s.vectors)
	return vectors
}

// metricName prefixes a metric name unless the caller already passed the
// prefixed form explicitly.
func metricName(prefix, suffix string) string {
	if prefix == "" {
		return suffix
	}

	if strings.HasPrefix(suffix, prefix+"_") {
		return suffix
	}

	return prefix + "_" + suffix
}

// sortedHeaderLabels returns header-derived label names in stable order so
// metric descriptors remain deterministic across map iteration.
func sortedHeaderLabels(headerLabels map[string]string) []string {
	labels := make([]string, 0, len(headerLabels))
	for label := range headerLabels {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels
}

// baseRequestLabels strips header-derived labels because only the main request
// counter carries them; the related histograms and byte counters do not.
func baseRequestLabels(labels prometheus.Labels) prometheus.Labels {
	return prometheus.Labels{
		"code":     labels["code"],
		"method":   labels["method"],
		"protocol": labels["protocol"],
		"route":    labels["route"],
	}
}

// normalizeRoute keeps a stable fallback for requests that did not match a
// named route.
func normalizeRoute(route string) string {
	if route == "" {
		return unknownRoute
	}
	return route
}

// normalizeService keeps service label values non-empty.
func normalizeService(service string) string {
	return normalizeValue(service)
}

// normalizeMethod keeps method label values non-empty.
func normalizeMethod(method string) string {
	return normalizeValue(method)
}

// normalizeProtocol defaults empty protocols to http so metrics remain
// queryable without special-case empty labels.
func normalizeProtocol(protocol string) string {
	if protocol == "" {
		return "http"
	}
	return protocol
}

// normalizeStatusCode guarantees that request metrics always expose a numeric
// status label.
func normalizeStatusCode(statusCode int) string {
	if statusCode <= 0 {
		statusCode = http.StatusOK
	}
	return strconv.Itoa(statusCode)
}

// normalizeValue replaces empty label values with a stable placeholder.
func normalizeValue(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

// nonNegativeFloat avoids exporting negative byte counters when upstream APIs
// use negative values to indicate "unknown".
func nonNegativeFloat(value int64) float64 {
	if value < 0 {
		return 0
	}
	return float64(value)
}

// requestSize returns a Prometheus-safe request size value.
func requestSize(request *http.Request) int64 {
	if request == nil || request.ContentLength < 0 {
		return 0
	}
	return request.ContentLength
}

// tlsVersion converts the Go TLS version constant into a human-readable label.
func tlsVersion(state *tls.ConnectionState) string {
	if state == nil {
		return ""
	}

	switch state.Version {
	case tls.VersionTLS10:
		return "1.0"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS13:
		return "1.3"
	default:
		return strconv.Itoa(int(state.Version))
	}
}

// tlsCipher converts the Go TLS cipher suite into its standard name.
func tlsCipher(state *tls.ConnectionState) string {
	if state == nil {
		return ""
	}

	return tls.CipherSuiteName(state.CipherSuite)
}
