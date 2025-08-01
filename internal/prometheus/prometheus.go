package prometheus

import (
	"github.com/nmq/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Counter implements prometheus.Counter, via prometheus CounterVec
type Counter struct {
	/*
	 * CounterVec is a Collector that bundles a set of Counters that all share thesame Desc,
	 */
	cv  *prometheus.CounterVec
	lvs metrics.LabelValues
}

// NewCounterFrom creates a new Counter from a CounterOpts and a list of label names
func NewCounterFrom(opts prometheus.CounterOpts, labelNames []string) *Counter {
	return NewCounter(promauto.NewCounterVec(opts, labelNames))
}

// NewCounter wraps the CounterVec and returns a usable Counter object
func NewCounter(cv *prometheus.CounterVec) *Counter {
	return &Counter{cv: cv}
}

// With implements Counter
func (c *Counter) With(labelValues ...string) metrics.Counter {
	return &Counter{
		cv:  c.cv,
		lvs: c.lvs.With(labelValues...),
	}
}

func (c *Counter) Add(delta float64) {
	c.cv.With(makeLabels(c.lvs...)).Add(delta)
}

func makeLabels(labelValues ...string) prometheus.Labels {
	labels := prometheus.Labels{}
	for i := 0; i < len(labelValues); i += 2 {
		labels[labelValues[i]] = labelValues[i+1]
	}
	return labels
}

// Gauge implements prometheus.Gauge, via prometheus GaugeVec
type Gauge struct {
	/*
	 * GaugeVec is a Collector that bundles a set of Gauges that all share the same Desc,
	 */
	gv  *prometheus.GaugeVec
	lvs metrics.LabelValues
}

// NewGaugeFrom creates a new Gauge from a GaugeOpts and a list of label names
func NewGaugeFrom(opts prometheus.GaugeOpts, labelNames []string) *Gauge {
	return NewGauge(promauto.NewGaugeVec(opts, labelNames))
}

// NewGauge wraps the GaugeVec and returns a usable Gauge object
func NewGauge(gv *prometheus.GaugeVec) *Gauge {
	return &Gauge{gv: gv}
}

// With implements Gauge
func (g *Gauge) With(labelValues ...string) metrics.Gauge {
	return &Gauge{
		gv:  g.gv,
		lvs: g.lvs.With(labelValues...),
	}
}

// Set implements Gauge
func (g *Gauge) Set(value float64) {
	g.gv.With(makeLabels(g.lvs...)).Set(value)
}

// Add implements Gauge
func (g *Gauge) Add(delta float64) {
	g.gv.With(makeLabels(g.lvs...)).Add(delta)
}

// Summary implements Histogram, via a Prometheus SummaryVec. The difference
// between a Summary and a Histogram is that Summaries don't require predefined
// quantile buckets, but cannot be statistically aggregated.
type Summary struct {
	sv  *prometheus.SummaryVec
	lvs metrics.LabelValues
}

// NewSummaryFrom constructs and registers a Prometheus SummaryVec,
// and returns a usable Summary object.
func NewSummaryFrom(opts prometheus.SummaryOpts, labelNames []string) *Summary {
	return NewSummary(promauto.NewSummaryVec(opts, labelNames))
}

// NewSummary wraps the SummaryVec and returns a usable Summary object
func NewSummary(sv *prometheus.SummaryVec) *Summary {
	return &Summary{sv: sv}
}

func (s *Summary) With(labelValues ...string) metrics.Histogram {
	return &Summary{
		sv:  s.sv,
		lvs: s.lvs.With(labelValues...),
	}
}

func (s *Summary) Observe(value float64) {
	s.sv.With(makeLabels(s.lvs...)).Observe(value)
}

// Histogram implements Histogram via a Prometheus HistogramVec. The difference
// between a Histogram and a Summary is that Histograms require predefined
// quantile buckets, and can be statistically aggregated.
type Histogram struct {
	hv  *prometheus.HistogramVec
	lvs metrics.LabelValues
}

// NewHistogramFrom constructs and registers a Prometheus HistogramVec,
// and returns a usable Histogram object.
func NewHistogramFrom(opts prometheus.HistogramOpts, labelNames []string) *Histogram {
	return NewHistogram(promauto.NewHistogramVec(opts, labelNames))
}

// NewHistogram wraps the HistogramVec and returns a usable Histogram object.
func NewHistogram(hv *prometheus.HistogramVec) *Histogram {
	return &Histogram{
		hv: hv,
	}
}

// With implements Histogram.
func (h *Histogram) With(labelValues ...string) metrics.Histogram {
	return &Histogram{
		hv:  h.hv,
		lvs: h.lvs.With(labelValues...),
	}
}

// Observe implements Histogram.
func (h *Histogram) Observe(value float64) {
	h.hv.With(makeLabels(h.lvs...)).Observe(value)
}
