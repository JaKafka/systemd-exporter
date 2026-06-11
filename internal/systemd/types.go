// Package systemd provides a cached, event-driven view of systemd unit
// states by subscribing to the systemd D-Bus API via go-systemd.
//
// The primary entry point is [New], which returns a [Collector] that
// maintains an up-to-date [Snapshot] in memory, updating only when units
// change state. Callers read the snapshot at any time through [Collector.Snapshot]
// without touching D-Bus.
//
// By default the collector tracks all unit types. Use [WithUnitType] or
// [WithNameFilter] to restrict collection to a subset.
package systemd

import "time"

// UnitState holds the current state of a single systemd unit.
// Fields map directly to the D-Bus UnitStatus properties returned by systemd.
type UnitState struct {
	// Name is the full unit name, e.g. "sshd.service" or "dev-sda.device".
	Name string
	// Description is the human-readable unit description.
	Description string
	// LoadState reflects whether the unit file was loaded successfully.
	// Common values: "loaded", "not-found", "masked", "error".
	LoadState string
	// ActiveState is the high-level activation state.
	// Common values: "active", "inactive", "failed", "activating", "deactivating", "reloading".
	ActiveState string
	// SubState is the more granular sub-state, specific to the unit type.
	// For services: "running", "dead", "exited", "failed", "start", "stop", etc.
	SubState string
}

// IsActive returns true when the unit is currently active.
func (u *UnitState) IsActive() bool {
	return u.ActiveState == "active"
}

// IsFailed returns true when the unit has entered the failed state.
func (u *UnitState) IsFailed() bool {
	return u.ActiveState == "failed"
}

// IsDead returns true for inactive units whose sub-state is "dead".
func (u *UnitState) IsDead() bool {
	return u.ActiveState == "inactive" && u.SubState == "dead"
}

// IsOneshot returns true for oneshot services that have completed execution.
// systemd keeps them in ActiveState "active" / SubState "exited" after a
// successful run.
func (u *UnitState) IsOneshot() bool {
	return u.ActiveState == "active" && u.SubState == "exited"
}

// Stats holds aggregated counters derived from the current set of tracked units.
// All fields are consistent with each other — computed from the same snapshot
// in a single pass.
type Stats struct {
	// Total is the number of units known to systemd.
	Total int
	// Active is the count of units with ActiveState == "active".
	// Includes both running and completed oneshot units.
	Active int
	// Failed is the count of units with ActiveState == "failed".
	Failed int
	// Dead is the count of inactive units in sub-state "dead".
	Dead int
	// Oneshot is the count of units that completed as oneshot
	// (ActiveState "active", SubState "exited").
	Oneshot int
}

// Snapshot is a point-in-time, consistent view of all tracked units and their
// aggregated statistics. Snapshots are cheap to copy — the Units map is
// replaced atomically on each update rather than mutated in place.
type Snapshot struct {
	// Units maps unit name to its current state.
	Units map[string]*UnitState
	// Stats contains pre-computed aggregates over all tracked units.
	Stats Stats
	// UpdatedAt is the wall-clock time when this snapshot was last rebuilt.
	UpdatedAt time.Time
}
