package metrics

// Config controls how the reusable metrics registry is initialized.
//
// The same Config is used for both the built-in HTTP/backend metrics and any
// custom Prometheus collectors created through Registry helper methods.
type Config struct {
	// Prefix is prepended to every metric name created by this package.
	// For example, Prefix "orders" turns "http_requests_total" into
	// "orders_http_requests_total".
	Prefix string
	// Buckets configures the default histogram buckets for latency-style metrics.
	// It is applied to the built-in histograms and to custom histograms when the
	// caller does not explicitly provide buckets.
	Buckets []float64
	// HeaderLabels maps exported metric label names to HTTP request header names.
	// The mapping is only applied to built-in HTTP request counters.
	HeaderLabels map[string]string
	// RegisterProcessMetrics enables the standard Prometheus process collector in
	// the registry created by New.
	RegisterProcessMetrics bool
	// RegisterGoMetrics enables the standard Prometheus Go runtime collector in
	// the registry created by New.
	RegisterGoMetrics bool
	// RegisterServerMetrics enables the standard Prometheus server collector in
	// the registry created by New.
	RegisterServerMetrics bool
}

// withDefaults fills only values that are safe package defaults.
func (c Config) withDefaults() Config {
	if c.Prefix == "" {
		c.Prefix = "app"
	}

	if len(c.Buckets) == 0 {
		c.Buckets = []float64{0.1, 0.3, 1.2, 5.0}
	}

	if c.HeaderLabels == nil {
		c.HeaderLabels = map[string]string{}
	}

	return c
}
