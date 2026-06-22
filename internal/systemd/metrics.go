package systemd

import "github.com/prometheus/client_golang/prometheus"

// Exporter adapts a [Collector] into a [prometheus.Collector]. Metrics are
// built fresh from the current snapshot on every scrape (the "collect on
// scrape" pattern), so exported values are never stale.
type Exporter struct {
	collector *Collector

	unitsTotal *prometheus.Desc
	unitStates *prometheus.Desc
}

// NewExporter returns a [prometheus.Collector] that exposes aggregate unit
// counts derived from c. Register it with a [prometheus.Registry].
func NewExporter(c *Collector) *Exporter {
	return &Exporter{
		collector: c,
		unitsTotal: prometheus.NewDesc(
			"systemd_units",
			"Total number of systemd units tracked by the exporter.",
			nil, nil,
		),
		unitStates: prometheus.NewDesc(
			"systemd_unit_states",
			"Number of systemd units in each aggregate state.",
			[]string{"state"}, nil,
		),
	}
}

// Describe implements [prometheus.Collector]. The descriptors are static; only
// the label values and metric values change between scrapes.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.unitsTotal
	ch <- e.unitStates
}

// Collect implements [prometheus.Collector]. It reads the current aggregate
// stats once and emits one metric per state.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	stats := e.collector.Stats()

	ch <- prometheus.MustNewConstMetric(e.unitsTotal, prometheus.GaugeValue, float64(stats.Total))

	byState := []struct {
		state string
		count int
	}{
		{"active", stats.Active},
		{"failed", stats.Failed},
		{"dead", stats.Dead},
		{"oneshot", stats.Oneshot},
	}
	for _, s := range byState {
		ch <- prometheus.MustNewConstMetric(e.unitStates, prometheus.GaugeValue, float64(s.count), s.state)
	}
}
