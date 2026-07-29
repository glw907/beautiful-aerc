// Package uerrtest is internal/uerr's half of ADR-0014's
// second-implementation pattern for engine tests
// (internal/backend/backendtest and internal/store/storetest are the
// other two): it lets a test in any other package observe what
// uerr.New actually wrote, the same assertion internal/uerr's own
// log_test.go makes on itself, without internal/uerr exporting its
// private logger. No other package can otherwise tell a state
// transition (ADR-0013 revision 2's one-line-per-outcome rule)
// actually constructed a uerr.Error from one that never did.
package uerrtest

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/glw907/poplar/internal/uerr"
)

// Capture redirects uerr's log destination to a buffer for t's
// duration, restoring it on cleanup, and returns the buffer: the same
// redirection internal/uerr's own captureLog performs on itself
// (log_test.go), through uerr.RedirectForTest.
func Capture(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	t.Cleanup(uerr.RedirectForTest(&buf))
	return &buf
}

// CaptureDefault redirects slog's process-wide default logger to a
// buffer for t's duration, restoring the previous default on cleanup,
// and returns the buffer. Capture covers uerr.New's own output only;
// uerr.New never reads slog.Default, so a plain log/slog call (an
// engine's recovery line, most notably) needs this half instead.
func CaptureDefault(t *testing.T) *Buffer {
	t.Helper()

	buf := &Buffer{}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return buf
}

// Buffer is CaptureDefault's destination. It is guarded because the
// engine goroutines under test log on their own schedule while the
// test goroutine reads what has arrived.
type Buffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write appends p to b.
func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String returns everything written to b so far.
func (b *Buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Lines decodes buf's JSON log lines, one per uerr.New call
// (ADR-0013 revision 2's one-line-per-outcome rule), failing t if any
// line does not parse.
func Lines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	text := strings.TrimSpace(buf.String())
	if text == "" {
		return nil
	}
	rawLines := strings.Split(text, "\n")
	lines := make([]map[string]any, len(rawLines))
	for i, raw := range rawLines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(raw), &rec); err != nil {
			t.Fatalf("unmarshal log line %d: %v (%q)", i, err, raw)
		}
		lines[i] = rec
	}
	return lines
}
