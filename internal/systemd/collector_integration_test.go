//go:build integration

package systemd

import (
	"strings"
	"testing"
)

func TestNew_Integration(t *testing.T) {
	ctx := t.Context()
	c, err := New(ctx)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	snap := c.Snapshot()
	if snap.Stats.Total == 0 {
		t.Error("expected at least one unit")
	}
	if snap.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestNew_WithUnitType_Integration(t *testing.T) {
	ctx := t.Context()
	c, err := New(ctx, WithUnitType(".service"))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	snap := c.Snapshot()
	for name := range snap.Units {
		if !strings.HasSuffix(name, ".service") {
			t.Errorf("expected only .service units, got %q", name)
		}
	}
}

func TestCollector_UnitState_Integration(t *testing.T) {
	ctx := t.Context()
	c, err := New(ctx)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer c.Close()

	snap := c.Snapshot()
	if len(snap.Units) == 0 {
		t.Skip("no units found — skipping")
	}

	var name string
	for name = range snap.Units {
		break
	}

	u, ok := c.UnitState(name)
	if !ok {
		t.Errorf("UnitState(%q): not found", name)
	}
	if u.Name != name {
		t.Errorf("UnitState(%q).Name=%q", name, u.Name)
	}
}
