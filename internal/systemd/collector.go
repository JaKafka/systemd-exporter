package systemd

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

const (
	// subscribeInterval is how often SubscribeUnitsCustom polls ListUnits.
	subscribeInterval = time.Second
	// subscribeBuffer is the depth of the change channel.
	subscribeBuffer = 16
)

// Option configures a [Collector].
type Option func(*Collector)

// WithUnitType restricts the collector to units whose name ends with the
// given suffix, e.g. ".service", ".timer", ".socket".
func WithUnitType(suffix string) Option {
	return WithNameFilter(func(name string) bool {
		return strings.HasSuffix(name, suffix)
	})
}

// WithNameFilter restricts the collector to units for which include returns
// true. When no filter option is provided, all unit types are collected.
func WithNameFilter(include func(string) bool) Option {
	return func(c *Collector) {
		c.filter = include
	}
}

// Collector subscribes to systemd via D-Bus and keeps a cached [Snapshot] of
// unit states up to date. It spawns a single background goroutine that updates
// the cache on each change. All public methods are safe for concurrent use.
type Collector struct {
	conn   *dbus.Conn
	filter func(string) bool // nil means collect all units

	mu       sync.RWMutex
	snapshot Snapshot
}

// New connects to the system D-Bus, performs an initial load of all units,
// and starts a background watcher goroutine. The goroutine runs until ctx is
// cancelled. The caller must call [Collector.Close] when done.
func New(ctx context.Context, opts ...Option) (*Collector, error) {
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		return nil, err
	}

	c := &Collector{conn: conn}
	for _, opt := range opts {
		opt(c)
	}

	if err := c.refresh(); err != nil {
		conn.Close()
		return nil, err
	}

	go c.watch(ctx)
	return c, nil
}

// Snapshot returns the current consistent view of all tracked units.
// The returned value is a copy; the caller may inspect it freely without
// holding any lock.
func (c *Collector) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Deep-copy Units so callers can't mutate collector state or race with updates.
	units := make(map[string]*UnitState, len(c.snapshot.Units))
	for name, u := range c.snapshot.Units {
		if u == nil {
			continue
		}
		uu := *u
		units[name] = &uu
	}

	snap := c.snapshot
	snap.Units = units
	return snap
}

// Stats returns the aggregate counters from the current snapshot. Unlike
// [Collector.Snapshot] it does not copy the per-unit map, so it is cheap enough
// to call on every metrics scrape.
func (c *Collector) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot.Stats
}

// UnitState returns the state of a single named unit and whether it was found
// in the current snapshot.
func (c *Collector) UnitState(name string) (*UnitState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	u, ok := c.snapshot.Units[name]
	if !ok || u == nil {
		return nil, ok
	}
	uu := *u
	return &uu, true
}

// Close releases the underlying D-Bus connection. It should be called after
// the context passed to [New] has been cancelled.
func (c *Collector) Close() {
	c.conn.Close()
}

// refresh performs a full ListUnits call and rebuilds the snapshot from scratch.
// Used for the initial load and as a recovery path after subscription errors.
func (c *Collector) refresh() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	units, err := c.conn.ListUnitsContext(ctx)
	if err != nil {
		return err
	}

	m := make(map[string]*UnitState, len(units))
	for _, u := range units {
		if c.filter != nil && !c.filter(u.Name) {
			continue
		}
		m[u.Name] = unitStateFromDBus(u)
	}

	c.mu.Lock()
	c.snapshot = Snapshot{
		Units:     m,
		Stats:     computeStats(m),
		UpdatedAt: time.Now(),
	}
	c.mu.Unlock()
	return nil
}

// watch runs in a background goroutine. It receives incremental unit change
// maps from SubscribeUnitsCustom and applies them to the cached snapshot.
// A nil value for a unit name means the unit was removed.
func (c *Collector) watch(ctx context.Context) {
	// SubscribeUnitsCustom's filterUnit returns true to SKIP a unit, which is
	// the inverse of our filter convention (true = include). We negate here.
	var skipUnit func(string) bool
	if c.filter != nil {
		skipUnit = func(name string) bool { return !c.filter(name) }
	}

	changes, errors := c.conn.SubscribeUnitsCustom(
		subscribeInterval,
		subscribeBuffer,
		unitChanged,
		skipUnit,
	)

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errors:
			if err != nil {
				_ = c.refresh()
			}
		case updates, ok := <-changes:
			if !ok || len(updates) == 0 {
				continue
			}
			c.applyUpdates(updates)
		}
	}
}

// applyUpdates merges an incremental change map into the current snapshot.
// It copies the Units map (copy-on-write) so that callers holding an older
// Snapshot value are not affected.
func (c *Collector) applyUpdates(updates map[string]*dbus.UnitStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := make(map[string]*UnitState, len(c.snapshot.Units)+len(updates))
	for k, v := range c.snapshot.Units {
		m[k] = v
	}

	for name, status := range updates {
		if c.filter != nil && !c.filter(name) {
			continue
		}
		if status == nil {
			slog.Debug("cache: removed unit", "unit", name)
			delete(m, name)
		} else {
			slog.Debug("cache: updated unit", "unit", name, "active", status.ActiveState, "sub", status.SubState)
			m[name] = unitStateFromDBus(*status)
		}
	}

	c.snapshot = Snapshot{
		Units:     m,
		Stats:     computeStats(m),
		UpdatedAt: time.Now(),
	}
}

// unitStateFromDBus maps a D-Bus UnitStatus to our internal UnitState.
func unitStateFromDBus(u dbus.UnitStatus) *UnitState {
	return &UnitState{
		Name:        u.Name,
		Description: u.Description,
		LoadState:   u.LoadState,
		ActiveState: u.ActiveState,
		SubState:    u.SubState,
	}
}

// computeStats walks the units map once and derives all aggregate counters.
func computeStats(units map[string]*UnitState) Stats {
	var s Stats
	s.Total = len(units)
	for _, u := range units {
		switch {
		case u.IsFailed():
			s.Failed++
		case u.IsOneshot():
			s.Active++
			s.Oneshot++
		case u.IsActive():
			s.Active++
		case u.IsDead():
			s.Dead++
		}
	}
	return s
}

// unitChanged is the change-detection predicate passed to SubscribeUnitsCustom.
// It returns true whenever any state field differs between old and new.
func unitChanged(old, new *dbus.UnitStatus) bool {
	if old == nil || new == nil {
		return true
	}
	return old.ActiveState != new.ActiveState ||
		old.SubState != new.SubState ||
		old.LoadState != new.LoadState
}
