package metrics

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

// ResourceLabelNames tells the registry which labels of a managed custom metric
// correspond to topology resources tracked by UpdateResources.
//
// When a managed custom metric is registered, the cleanup logic uses these label
// names to remove stale time series after routes, listeners, services, or
// backend URLs disappear from the latest ResourceSnapshot.
type ResourceLabelNames struct {
	// Listener names the label that holds a listener identifier.
	Listener string
	// Route names the label that holds a route identifier.
	Route string
	// Service names the label that holds a logical backend service identifier.
	Service string
	// BackendURL names the label that holds a concrete backend target URL.
	// This requires Service to be set as well so cleanup can scope URL removal to
	// a specific service.
	BackendURL string
}

// validate rejects cleanup mappings that cannot be applied safely.
func (r ResourceLabelNames) validate() error {
	if r.BackendURL != "" && r.Service == "" {
		return errors.New("backend URL cleanup requires a service label name")
	}

	return nil
}

// MetricName returns the fully prefixed metric name used by this registry.
// It is useful when callers create collectors manually and still want naming to
// remain consistent with the registry prefix.
func (r *Registry) MetricName(name string) string {
	return metricName(r.config.Prefix, name)
}

// RegisterCollector registers an arbitrary Prometheus collector in the current
// registry.
//
// If the collector supports DeletePartialMatch, it is routed through the
// registry's internal cleanup-aware collector state so that it participates in
// scrape ordering consistently with built-in metrics. Otherwise it is registered
// directly in the underlying Prometheus registry.
func (r *Registry) RegisterCollector(collector prometheus.Collector) error {
	if collector == nil {
		return errors.New("collector is nil")
	}

	if managedCollector, ok := collector.(vector); ok {
		r.state.addVectors(managedCollector)
		return nil
	}

	return r.promRegistry.Register(collector)
}

// RegisterManagedCollector registers a custom collector that should also
// participate in resource-based stale-series cleanup.
//
// Only vector-style collectors that implement DeletePartialMatch can be managed,
// because cleanup must be able to remove individual label combinations.
func (r *Registry) RegisterManagedCollector(collector prometheus.Collector, resourceLabels ResourceLabelNames) error {
	if collector == nil {
		return errors.New("collector is nil")
	}

	if err := resourceLabels.validate(); err != nil {
		return err
	}

	managedCollector, ok := collector.(vector)
	if !ok {
		return errors.New("collector does not support DeletePartialMatch; use vector collectors such as CounterVec, GaugeVec, HistogramVec, or SummaryVec")
	}

	r.state.addTrackedVectors(trackedVector{
		collector:      managedCollector,
		resourceLabels: resourceLabels,
	})

	return nil
}

// MustRegisterCollectors is the panic-on-error form of RegisterCollector.
func (r *Registry) MustRegisterCollectors(collectors ...prometheus.Collector) {
	for _, collector := range collectors {
		if err := r.RegisterCollector(collector); err != nil {
			panic(err)
		}
	}
}

// MustRegisterManagedCollector is the panic-on-error form of
// RegisterManagedCollector.
func (r *Registry) MustRegisterManagedCollector(collector prometheus.Collector, resourceLabels ResourceLabelNames) {
	if err := r.RegisterManagedCollector(collector, resourceLabels); err != nil {
		panic(err)
	}
}

// NewCounter creates, prefixes, registers, and returns a plain Prometheus
// counter.
//
// Use this helper when the metric does not need dynamic labels. For fixed
// labels, rely on opts.ConstLabels.
func (r *Registry) NewCounter(opts prometheus.CounterOpts) prometheus.Counter {
	opts.Name = r.MetricName(opts.Name)
	collector := prometheus.NewCounter(opts)
	r.MustRegisterCollectors(collector)
	return collector
}

// NewCounterVec creates, prefixes, registers, and returns a Prometheus
// CounterVec.
func (r *Registry) NewCounterVec(opts prometheus.CounterOpts, labelNames []string) *prometheus.CounterVec {
	opts.Name = r.MetricName(opts.Name)
	collector := prometheus.NewCounterVec(opts, labelNames)
	r.MustRegisterCollectors(collector)
	return collector
}

// NewManagedCounterVec creates a CounterVec that also participates in
// UpdateResources-based stale-series cleanup.
func (r *Registry) NewManagedCounterVec(opts prometheus.CounterOpts, labelNames []string, resourceLabels ResourceLabelNames) *prometheus.CounterVec {
	opts.Name = r.MetricName(opts.Name)
	collector := prometheus.NewCounterVec(opts, labelNames)
	r.MustRegisterManagedCollector(collector, resourceLabels)
	return collector
}

// NewGauge creates, prefixes, registers, and returns a plain Prometheus gauge.
func (r *Registry) NewGauge(opts prometheus.GaugeOpts) prometheus.Gauge {
	opts.Name = r.MetricName(opts.Name)
	collector := prometheus.NewGauge(opts)
	r.MustRegisterCollectors(collector)
	return collector
}

// NewGaugeVec creates, prefixes, registers, and returns a Prometheus GaugeVec.
func (r *Registry) NewGaugeVec(opts prometheus.GaugeOpts, labelNames []string) *prometheus.GaugeVec {
	opts.Name = r.MetricName(opts.Name)
	collector := prometheus.NewGaugeVec(opts, labelNames)
	r.MustRegisterCollectors(collector)
	return collector
}

// NewManagedGaugeVec creates a GaugeVec that also participates in
// UpdateResources-based stale-series cleanup.
func (r *Registry) NewManagedGaugeVec(opts prometheus.GaugeOpts, labelNames []string, resourceLabels ResourceLabelNames) *prometheus.GaugeVec {
	opts.Name = r.MetricName(opts.Name)
	collector := prometheus.NewGaugeVec(opts, labelNames)
	r.MustRegisterManagedCollector(collector, resourceLabels)
	return collector
}

// NewHistogram creates, prefixes, registers, and returns a plain Prometheus
// histogram.
//
// When opts.Buckets is empty, the registry-level default buckets are used.
func (r *Registry) NewHistogram(opts prometheus.HistogramOpts) prometheus.Histogram {
	opts.Name = r.MetricName(opts.Name)
	if len(opts.Buckets) == 0 {
		opts.Buckets = r.config.Buckets
	}

	collector := prometheus.NewHistogram(opts)
	r.MustRegisterCollectors(collector)
	return collector
}

// NewHistogramVec creates, prefixes, registers, and returns a Prometheus
// HistogramVec. Empty bucket definitions inherit the registry defaults.
func (r *Registry) NewHistogramVec(opts prometheus.HistogramOpts, labelNames []string) *prometheus.HistogramVec {
	opts.Name = r.MetricName(opts.Name)
	if len(opts.Buckets) == 0 {
		opts.Buckets = r.config.Buckets
	}

	collector := prometheus.NewHistogramVec(opts, labelNames)
	r.MustRegisterCollectors(collector)
	return collector
}

// NewManagedHistogramVec creates a HistogramVec that also participates in
// UpdateResources-based stale-series cleanup.
func (r *Registry) NewManagedHistogramVec(opts prometheus.HistogramOpts, labelNames []string, resourceLabels ResourceLabelNames) *prometheus.HistogramVec {
	opts.Name = r.MetricName(opts.Name)
	if len(opts.Buckets) == 0 {
		opts.Buckets = r.config.Buckets
	}

	collector := prometheus.NewHistogramVec(opts, labelNames)
	r.MustRegisterManagedCollector(collector, resourceLabels)
	return collector
}

// NewSummary creates, prefixes, registers, and returns a plain Prometheus
// summary.
func (r *Registry) NewSummary(opts prometheus.SummaryOpts) prometheus.Summary {
	opts.Name = r.MetricName(opts.Name)
	collector := prometheus.NewSummary(opts)
	r.MustRegisterCollectors(collector)
	return collector
}

// NewSummaryVec creates, prefixes, registers, and returns a Prometheus
// SummaryVec.
func (r *Registry) NewSummaryVec(opts prometheus.SummaryOpts, labelNames []string) *prometheus.SummaryVec {
	opts.Name = r.MetricName(opts.Name)
	collector := prometheus.NewSummaryVec(opts, labelNames)
	r.MustRegisterCollectors(collector)
	return collector
}

// NewManagedSummaryVec creates a SummaryVec that also participates in
// UpdateResources-based stale-series cleanup.
func (r *Registry) NewManagedSummaryVec(opts prometheus.SummaryOpts, labelNames []string, resourceLabels ResourceLabelNames) *prometheus.SummaryVec {
	opts.Name = r.MetricName(opts.Name)
	collector := prometheus.NewSummaryVec(opts, labelNames)
	r.MustRegisterManagedCollector(collector, resourceLabels)
	return collector
}
