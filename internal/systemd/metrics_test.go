package systemd

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestExporter_Describe(t *testing.T) {
	e := NewExporter(&Collector{})

	ch := make(chan *prometheus.Desc, 2)
	e.Describe(ch)
	close(ch)

	var descs []*prometheus.Desc
	for d := range ch {
		descs = append(descs, d)
	}
	if len(descs) != 2 {
		t.Errorf("Describe() sent %d descriptors, want 2", len(descs))
	}
}

func TestExporter_Collect(t *testing.T) {
	c := newTestCollector(map[string]*UnitState{
		"a.service": {ActiveState: "active", SubState: "running"},
		"b.service": {ActiveState: "failed", SubState: "failed"},
		"c.service": {ActiveState: "inactive", SubState: "dead"},
		"d.service": {ActiveState: "active", SubState: "exited"},
	})
	e := NewExporter(c)

	const want = `
		# HELP systemd_unit_states Number of systemd units in each aggregate state.
		# TYPE systemd_unit_states gauge
		systemd_unit_states{state="active"} 2
		systemd_unit_states{state="dead"} 1
		systemd_unit_states{state="failed"} 1
		systemd_unit_states{state="oneshot"} 1
		# HELP systemd_units Total number of systemd units tracked by the exporter.
		# TYPE systemd_units gauge
		systemd_units 4
	`
	if err := testutil.CollectAndCompare(e, strings.NewReader(want)); err != nil {
		t.Errorf("unexpected metrics:\n%v", err)
	}
}
