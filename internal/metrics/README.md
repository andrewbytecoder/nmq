# Gin Metrics Library

This directory contains a reusable Prometheus metrics package extracted from the patterns used in Traefik's metrics implementation and adapted for Gin services.

## What it provides

- Prometheus registry and `/metrics` handler.
- Gin middleware for HTTP request count, latency, request bytes, and response bytes.
- Optional request header labels on `*_http_requests_total`.
- Backend request, retry, and health metrics.
- Runtime registration of new Prometheus metrics in the same registry.
- Stale series cleanup for routes, listeners, services, and backend URLs after topology changes.

## Quick start

```go
package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	appmetrics "github.com/traefik/traefik/v3/metrics"
)

func main() {
	registry := appmetrics.New(appmetrics.Config{
		Prefix:       "orders",
		Buckets:      []float64{0.05, 0.1, 0.3, 1, 3},
		HeaderLabels: map[string]string{"tenant": "X-Tenant"},
	})

	router := gin.New()
	router.Use(registry.GinMiddleware(appmetrics.GinMiddlewareOptions{
		ShouldObserve: func(c *gin.Context) bool {
			return c.FullPath() != "/metrics"
		},
	}))

	router.GET("/metrics", registry.GinHandler())
	router.GET("/orders/:id", func(c *gin.Context) {
		start := time.Now()

		c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})

		registry.ObserveBackendRequest(appmetrics.BackendRequestObservation{
			Service:    "inventory",
			Method:     http.MethodGet,
			Protocol:   "http",
			StatusCode: http.StatusOK,
			Duration:   time.Since(start),
		})
	})

	registry.SyncGinRoutes(router)
	registry.UpdateResources(appmetrics.ResourceSnapshot{
		Services: []string{"inventory"},
		BackendURLs: map[string][]string{
			"inventory": {"http://inventory.default.svc.cluster.local"},
		},
	})

	cacheHits := registry.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "Cache hit count.",
	}, []string{"cache"})
	cacheHits.WithLabelValues("product").Inc()

	_ = router.Run(":8080")
}
```

## Metric names

With `Prefix: "orders"`, the main metrics are:

- `orders_http_requests_total`
- `orders_http_request_duration_seconds`
- `orders_http_requests_bytes_total`
- `orders_http_responses_bytes_total`
- `orders_backend_requests_total`
- `orders_backend_request_duration_seconds`
- `orders_backend_retries_total`
- `orders_backend_server_up`

## Resource cleanup

If routes or backend targets change at runtime, call `UpdateResources` again with the latest snapshot. Stale time series are removed after the next Prometheus scrape, which matches the cleanup strategy used in Traefik.

## Add custom metrics at runtime

You can register extra Prometheus collectors after `New(...)` returns:

```go
jobsTotal := registry.NewCounterVec(prometheus.CounterOpts{
	Name: "jobs_total",
	Help: "Background jobs processed.",
}, []string{"queue", "result"})

jobsTotal.WithLabelValues("email", "success").Inc()
```

You can also create non-`Vec` metrics when you do not need dynamic labels:

```go
totalJobs := registry.NewCounter(prometheus.CounterOpts{
	Name: "jobs_total",
	Help: "Background jobs processed.",
})

totalJobs.Add(1)
```

If you need fixed labels on a non-`Vec` metric, use `ConstLabels` in `prometheus.CounterOpts`, `GaugeOpts`, `HistogramOpts`, or `SummaryOpts`.

If you already created a collector yourself, register it into the same registry:

```go
inFlight := prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: registry.MetricName("workers_inflight"),
	Help: "In-flight workers.",
}, []string{"worker"})

registry.MustRegisterCollectors(inFlight)
```

If a runtime metric should be cleaned automatically when routes, listeners, services, or backend URLs disappear, register it with resource label mappings:

```go
workerLatency := registry.NewManagedHistogramVec(prometheus.HistogramOpts{
	Name: "worker_latency_seconds",
	Help: "Background worker latency.",
}, []string{"service_name", "backend_url"}, appmetrics.ResourceLabelNames{
	Service:    "service_name",
	BackendURL: "backend_url",
})

workerLatency.WithLabelValues("inventory", "http://inventory.default.svc.cluster.local").Observe(0.12)
```

`UpdateResources(...)` will then remove stale time series for this metric after the next scrape, using the label names you mapped.
