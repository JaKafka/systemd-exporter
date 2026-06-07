//go:build !journal

package systemd

import (
	"context"
	"errors"
)

// ErrJournalUnavailable is returned when the binary was built without the
// "journal" build tag (journal support is not compiled in).
var ErrJournalUnavailable = errors.New("journal support not compiled in: rebuild with -tags journal (requires CGO_ENABLED=1 and libsystemd-dev)")

// GetServiceLogs is a stub that returns [ErrJournalUnavailable] when built
// without the journal build tag.
func GetServiceLogs(_ string, _ int) ([]LogEntry, error) {
	return nil, ErrJournalUnavailable
}

// StreamServiceLogs is a stub that returns [ErrJournalUnavailable] when built
// without the journal build tag.
func StreamServiceLogs(_ context.Context, _ string, _ int) (<-chan LogEntry, error) {
	return nil, ErrJournalUnavailable
}
