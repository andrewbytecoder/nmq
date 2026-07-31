// Package metrics provides a reusable Prometheus metrics registry for Gin-based HTTP services.
//
// It keeps the parts of Traefik's Prometheus implementation that are broadly useful in
// application code: request counters, latency histograms, backend health metrics, optional
// header labels, and stale-series cleanup when routes or backend targets disappear.
package metrics
