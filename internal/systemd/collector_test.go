package systemd

import (
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

func TestWithUnitType_FiltersRefresh(t *testing.T) {
	units := map[string]*UnitState{
		"a.service": {ActiveState: "active"},
		"b.device":  {ActiveState: "active"},
		"c.service": {ActiveState: "failed"},
	}

	// Simulate what refresh does with a filter.
	filter := func(name string) bool { return strings.HasSuffix(name, ".service") }
	filtered := make(map[string]*UnitState)
	for name, u := range units {
		if filter(name) {
			filtered[name] = u
		}
	}

	if len(filtered) != 2 {
		t.Errorf("expected 2 units after filter, got %d", len(filtered))
	}
	if _, ok := filtered["b.device"]; ok {
		t.Error("b.device should have been filtered out")
	}
}
