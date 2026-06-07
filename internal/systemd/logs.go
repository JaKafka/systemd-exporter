//go:build journal

package systemd

import (
	"context"
	"fmt"
	"time"

	"github.com/coreos/go-systemd/v22/sdjournal"
)

// GetServiceLogs reads up to n most recent journal entries for the named unit
// and returns them in chronological order. Stateless and safe for concurrent use.
func GetServiceLogs(serviceName string, n int) ([]LogEntry, error) {
	if n < 0 {
		return nil, fmt.Errorf("n must be >= 0, got %d", n)
	}

	j, err := sdjournal.NewJournal()
	if err != nil {
		return nil, fmt.Errorf("open journal: %w", err)
	}
	defer j.Close()

	if err := j.AddMatch(sdjournal.SD_JOURNAL_FIELD_SYSTEMD_UNIT + "=" + serviceName); err != nil {
		return nil, fmt.Errorf("add unit match: %w", err)
	}
	if err := j.SeekTail(); err != nil {
		return nil, fmt.Errorf("seek journal tail: %w", err)
	}

	entries := make([]LogEntry, 0, n)
	for i := 0; i < n; i++ {
		moved, err := j.Previous()
		if err != nil {
			return nil, fmt.Errorf("read journal: %w", err)
		}
		if moved == 0 {
			break
		}
		entry, err := j.GetEntry()
		if err != nil {
			continue
		}
		entries = append(entries, logEntryFromJournal(entry))
	}

	for lo, hi := 0, len(entries)-1; lo < hi; lo, hi = lo+1, hi-1 {
		entries[lo], entries[hi] = entries[hi], entries[lo]
	}
	return entries, nil
}

// StreamServiceLogs streams journal entries for the given unit to the returned
// channel. The last nInitial historical entries are sent first, then new entries
// are streamed as they arrive. The channel is closed when ctx is cancelled.
func StreamServiceLogs(ctx context.Context, serviceName string, nInitial int) (<-chan LogEntry, error) {
	if nInitial < 0 {
		return nil, fmt.Errorf("nInitial must be >= 0, got %d", nInitial)
	}

	j, err := sdjournal.NewJournal()
	if err != nil {
		return nil, fmt.Errorf("open journal: %w", err)
	}
	if err := j.AddMatch(sdjournal.SD_JOURNAL_FIELD_SYSTEMD_UNIT + "=" + serviceName); err != nil {
		j.Close()
		return nil, fmt.Errorf("add unit match: %w", err)
	}
	if err := j.SeekTail(); err != nil {
		j.Close()
		return nil, fmt.Errorf("seek tail: %w", err)
	}

	// Read last nInitial entries backwards.
	historical := make([]LogEntry, 0, nInitial)
	for i := 0; i < nInitial; i++ {
		moved, err := j.Previous()
		if err != nil || moved == 0 {
			break
		}
		entry, err := j.GetEntry()
		if err != nil {
			continue
		}
		historical = append(historical, logEntryFromJournal(entry))
	}
	for lo, hi := 0, len(historical)-1; lo < hi; lo, hi = lo+1, hi-1 {
		historical[lo], historical[hi] = historical[hi], historical[lo]
	}

	// Re-seek to tail so Next() picks up only new entries from here on.
	if err := j.SeekTail(); err != nil {
		j.Close()
		return nil, fmt.Errorf("seek tail for streaming: %w", err)
	}

	ch := make(chan LogEntry, 32)

	go func() {
		defer close(ch)
		defer j.Close()

		for _, e := range historical {
			select {
			case ch <- e:
			case <-ctx.Done():
				return
			}
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if r := j.Wait(250 * time.Millisecond); r != sdjournal.SD_JOURNAL_APPEND {
				continue
			}

			for {
				moved, err := j.Next()
				if err != nil || moved == 0 {
					break
				}
				entry, err := j.GetEntry()
				if err != nil {
					continue
				}
				select {
				case ch <- logEntryFromJournal(entry):
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

func logEntryFromJournal(e *sdjournal.JournalEntry) LogEntry {
	ts := time.Unix(0, int64(e.RealtimeTimestamp)*int64(time.Microsecond))

	priority := 6
	if p, ok := e.Fields[sdjournal.SD_JOURNAL_FIELD_PRIORITY]; ok {
		_, _ = fmt.Sscanf(p, "%d", &priority)
	}

	return LogEntry{
		Timestamp: ts,
		Priority:  priority,
		Message:   e.Fields[sdjournal.SD_JOURNAL_FIELD_MESSAGE],
		Unit:      e.Fields[sdjournal.SD_JOURNAL_FIELD_SYSTEMD_UNIT],
	}
}
