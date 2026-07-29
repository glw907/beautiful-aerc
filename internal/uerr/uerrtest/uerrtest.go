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
	"strings"
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
