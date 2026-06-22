package systemd

import (
	"context"
	"strings"
	"testing"

	"github.com/coreos/go-systemd/v22/dbus"
)

// ---------------------------------------------------------------------------
// UnitState helpers
// ---------------------------------------------------------------------------

func TestUnitState_IsActive(t *testing.T) {
	tests := []struct {
		activeState string
		want        bool
	}{
		{"active", true},
		{"inactive", false},
		{"failed", false},
		{"activating", false},
	}
	for _, tt := range tests {
		u := &UnitState{ActiveState: tt.activeState}
		if got := u.IsActive(); got != tt.want {
			t.Errorf("ActiveState=%q: IsActive()=%v, want %v", tt.activeState, got, tt.want)
		}
	}
}

func TestUnitState_IsFailed(t *testing.T) {
	tests := []struct {
		activeState string
		want        bool
	}{
		{"failed", true},
		{"active", false},
		{"inactive", false},
	}
	for _, tt := range tests {
		u := &UnitState{ActiveState: tt.activeState}
		if got := u.IsFailed(); got != tt.want {
			t.Errorf("ActiveState=%q: IsFailed()=%v, want %v", tt.activeState, got, tt.want)
		}
	}
}

func TestUnitState_IsDead(t *testing.T) {
	tests := []struct {
		activeState string
		subState    string
		want        bool
	}{
		{"inactive", "dead", true},
		{"inactive", "failed", false},
		{"active", "dead", false},
		{"failed", "dead", false},
	}
	for _, tt := range tests {
		u := &UnitState{ActiveState: tt.activeState, SubState: tt.subState}
		if got := u.IsDead(); got != tt.want {
			t.Errorf("ActiveState=%q SubState=%q: IsDead()=%v, want %v",
				tt.activeState, tt.subState, got, tt.want)
		}
	}
}

func TestUnitState_IsOneshot(t *testing.T) {
	tests := []struct {
		activeState string
		subState    string
		want        bool
	}{
		{"active", "exited", true},
		{"active", "running", false},
		{"inactive", "exited", false},
	}
	for _, tt := range tests {
		u := &UnitState{ActiveState: tt.activeState, SubState: tt.subState}
		if got := u.IsOneshot(); got != tt.want {
			t.Errorf("ActiveState=%q SubState=%q: IsOneshot()=%v, want %v",
				tt.activeState, tt.subState, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// computeStats
// ---------------------------------------------------------------------------

func TestComputeStats(t *testing.T) {
	units := map[string]*UnitState{
		"running.service": {ActiveState: "active", SubState: "running"},
		"failed.service":  {ActiveState: "failed", SubState: "failed"},
		"dead.service":    {ActiveState: "inactive", SubState: "dead"},
		"oneshot.service": {ActiveState: "active", SubState: "exited"},
		"loading.service": {ActiveState: "activating", SubState: "start"},
	}

	s := computeStats(units)

	if s.Total != 5 {
		t.Errorf("Total=%d, want 5", s.Total)
	}
	if s.Active != 2 {
		t.Errorf("Active=%d, want 2", s.Active)
	}
	if s.Failed != 1 {
		t.Errorf("Failed=%d, want 1", s.Failed)
	}
	if s.Dead != 1 {
		t.Errorf("Dead=%d, want 1", s.Dead)
	}
	if s.Oneshot != 1 {
		t.Errorf("Oneshot=%d, want 1", s.Oneshot)
	}
}

func TestComputeStats_Empty(t *testing.T) {
	s := computeStats(map[string]*UnitState{})
	if s.Total != 0 || s.Active != 0 || s.Failed != 0 {
		t.Errorf("expected all-zero stats for empty map, got %+v", s)
	}
}

// ---------------------------------------------------------------------------
// unitChanged
// ---------------------------------------------------------------------------

func TestUnitChanged_NilOld(t *testing.T) {
	n := &dbus.UnitStatus{ActiveState: "active"}
	if !unitChanged(nil, n) {
		t.Error("expected unitChanged=true when old is nil")
	}
}

func TestUnitChanged_NilNew(t *testing.T) {
	old := &dbus.UnitStatus{ActiveState: "active"}
	if !unitChanged(old, nil) {
		t.Error("expected unitChanged=true when new is nil")
	}
}

func TestUnitChanged_StateTransitions(t *testing.T) {
	tests := []struct {
		old  dbus.UnitStatus
		new  dbus.UnitStatus
		want bool
	}{
		{
			old:  dbus.UnitStatus{ActiveState: "active", SubState: "running", LoadState: "loaded"},
			new:  dbus.UnitStatus{ActiveState: "active", SubState: "running", LoadState: "loaded"},
			want: false,
		},
		{
			old:  dbus.UnitStatus{ActiveState: "active", SubState: "running", LoadState: "loaded"},
			new:  dbus.UnitStatus{ActiveState: "failed", SubState: "failed", LoadState: "loaded"},
			want: true,
		},
		{
			old:  dbus.UnitStatus{ActiveState: "active", SubState: "running", LoadState: "loaded"},
			new:  dbus.UnitStatus{ActiveState: "active", SubState: "stop", LoadState: "loaded"},
			want: true,
		},
	}
	for _, tt := range tests {
		got := unitChanged(&tt.old, &tt.new)
		if got != tt.want {
			t.Errorf("unitChanged(%+v, %+v)=%v, want %v", tt.old, tt.new, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// unitStateFromDBus
// ---------------------------------------------------------------------------

func TestUnitStateFromDBus(t *testing.T) {
	u := dbus.UnitStatus{
		Name:        "test.service",
		Description: "Test Service",
		LoadState:   "loaded",
		ActiveState: "active",
		SubState:    "running",
	}
	s := unitStateFromDBus(u)
	if s.Name != u.Name {
		t.Errorf("Name=%q, want %q", s.Name, u.Name)
	}
	if s.ActiveState != u.ActiveState {
		t.Errorf("ActiveState=%q, want %q", s.ActiveState, u.ActiveState)
	}
	if s.SubState != u.SubState {
		t.Errorf("SubState=%q, want %q", s.SubState, u.SubState)
	}
}

// ---------------------------------------------------------------------------
// WithNameFilter / WithUnitType options
// ---------------------------------------------------------------------------

func TestWithUnitType_SetsFilter(t *testing.T) {
	c := &Collector{}
	WithUnitType(".service")(c)

	if c.filter == nil {
		t.Fatal("expected filter to be set")
	}
	if !c.filter("a.service") {
		t.Error("a.service should pass the filter")
	}
	if c.filter("b.device") {
		t.Error("b.device should not pass the filter")
	}
}

func TestWithNameFilter_SetsFilter(t *testing.T) {
	c := &Collector{}
	WithNameFilter(func(name string) bool { return strings.HasPrefix(name, "docker") })(c)

	if !c.filter("docker.service") {
		t.Error("docker.service should pass the filter")
	}
	if c.filter("sshd.service") {
		t.Error("sshd.service should not pass the filter")
	}
}

// ---------------------------------------------------------------------------
// Collector read methods (no D-Bus connection required)
// ---------------------------------------------------------------------------

func newTestCollector(units map[string]*UnitState) *Collector {
	c := &Collector{}
	c.snapshot = Snapshot{
		Units: units,
		Stats: computeStats(units),
	}
	return c
}

func TestCollector_Snapshot(t *testing.T) {
	c := newTestCollector(map[string]*UnitState{
		"a.service": {Name: "a.service", ActiveState: "active"},
	})

	snap := c.Snapshot()
	if snap.Stats.Total != 1 {
		t.Errorf("Stats.Total=%d, want 1", snap.Stats.Total)
	}

	// Mutating the returned snapshot must not affect the collector's state.
	snap.Units["a.service"].ActiveState = "failed"
	if c.snapshot.Units["a.service"].ActiveState != "active" {
		t.Error("Snapshot() did not deep-copy units; mutation leaked into collector state")
	}
}

func TestCollector_Stats(t *testing.T) {
	c := newTestCollector(map[string]*UnitState{
		"a.service": {ActiveState: "active"},
		"b.service": {ActiveState: "failed"},
	})

	stats := c.Stats()
	if stats.Total != 2 || stats.Active != 1 || stats.Failed != 1 {
		t.Errorf("Stats()=%+v, want Total=2 Active=1 Failed=1", stats)
	}
}

func TestCollector_UnitState(t *testing.T) {
	c := newTestCollector(map[string]*UnitState{
		"a.service": {Name: "a.service", ActiveState: "active"},
	})

	u, ok := c.UnitState("a.service")
	if !ok {
		t.Fatal("expected a.service to be found")
	}
	if u.ActiveState != "active" {
		t.Errorf("ActiveState=%q, want %q", u.ActiveState, "active")
	}

	if _, ok := c.UnitState("missing.service"); ok {
		t.Error("expected missing.service to be not found")
	}
}

// ---------------------------------------------------------------------------
// applyUpdates
// ---------------------------------------------------------------------------

func TestCollector_ApplyUpdates_AddsAndUpdates(t *testing.T) {
	c := newTestCollector(map[string]*UnitState{
		"a.service": {Name: "a.service", ActiveState: "active"},
	})

	c.applyUpdates(map[string]*dbus.UnitStatus{
		"a.service": {Name: "a.service", ActiveState: "failed"},
		"b.service": {Name: "b.service", ActiveState: "active"},
	})

	snap := c.Snapshot()
	if snap.Stats.Total != 2 {
		t.Fatalf("Stats.Total=%d, want 2", snap.Stats.Total)
	}
	if u, _ := c.UnitState("a.service"); u.ActiveState != "failed" {
		t.Errorf("a.service ActiveState=%q, want %q", u.ActiveState, "failed")
	}
	if _, ok := c.UnitState("b.service"); !ok {
		t.Error("expected b.service to be added")
	}
}

func TestCollector_ApplyUpdates_RemovesUnit(t *testing.T) {
	c := newTestCollector(map[string]*UnitState{
		"a.service": {Name: "a.service", ActiveState: "active"},
	})

	c.applyUpdates(map[string]*dbus.UnitStatus{
		"a.service": nil,
	})

	if _, ok := c.UnitState("a.service"); ok {
		t.Error("expected a.service to be removed")
	}
	if c.Snapshot().Stats.Total != 0 {
		t.Errorf("Stats.Total=%d, want 0", c.Snapshot().Stats.Total)
	}
}

func TestCollector_ApplyUpdates_RespectsFilter(t *testing.T) {
	c := newTestCollector(nil)
	WithUnitType(".service")(c)

	c.applyUpdates(map[string]*dbus.UnitStatus{
		"a.service": {Name: "a.service", ActiveState: "active"},
		"b.device":  {Name: "b.device", ActiveState: "active"},
	})

	if _, ok := c.UnitState("a.service"); !ok {
		t.Error("expected a.service to pass the filter")
	}
	if _, ok := c.UnitState("b.device"); ok {
		t.Error("expected b.device to be filtered out")
	}
}

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func TestNew_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := New(ctx); err == nil {
		t.Error("expected New() to fail with an already-cancelled context")
	}
}

// ---------------------------------------------------------------------------
// Snapshot
// ---------------------------------------------------------------------------

func TestCollector_Snapshot_SkipsNilUnits(t *testing.T) {
	c := &Collector{}
	c.snapshot = Snapshot{
		Units: map[string]*UnitState{
			"a.service": {Name: "a.service", ActiveState: "active"},
			"b.service": nil,
		},
	}

	snap := c.Snapshot()
	if _, ok := snap.Units["b.service"]; ok {
		t.Error("expected nil unit to be skipped by Snapshot()")
	}
	if len(snap.Units) != 1 {
		t.Errorf("len(snap.Units)=%d, want 1", len(snap.Units))
	}
}
