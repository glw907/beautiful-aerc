package uerr

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/adrg/xdg"
)

// captureLog redirects logger to a buffer for the test's duration and
// returns it. logger and level are unexported package state; a test
// in this package reassigns them directly instead of exporting a hook.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	origLogger, origLevel := logger, level.Level()
	logger = sync.OnceValue(func() *slog.Logger {
		return slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level}))
	})
	t.Cleanup(func() {
		logger = origLogger
		level.Set(origLevel)
	})
	return &buf
}

// decodeLogLine asserts buf holds exactly one JSON log line -- the
// one-line-per-outcome rule ADR-0013 revision 2 states -- and returns
// its fields.
func decodeLogLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1: %q", len(lines), buf.String())
	}

	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("unmarshal log line: %v", err)
	}
	return rec
}

func TestLogDefaultLocation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	xdg.Reload()
	t.Cleanup(xdg.Reload)

	w, ok := openLogWriter().(*rotatingWriter)
	if !ok {
		t.Fatalf("openLogWriter() = %T, want *rotatingWriter", openLogWriter())
	}

	want := filepath.Join(home, ".local", "state", "poplar", "poplar.log")
	if w.path != want {
		t.Errorf("log path = %q, want %q", w.path, want)
	}
}

func TestDebugIsOptIn(t *testing.T) {
	buf := captureLog(t)

	logger().Debug("trace", slog.String("op", "test"))
	if buf.Len() != 0 {
		t.Fatalf("debug logged at the default level: %q", buf.String())
	}

	SetLevel(slog.LevelDebug)
	logger().Debug("trace", slog.String("op", "test"))
	if buf.Len() == 0 {
		t.Fatal("debug did not log after SetLevel(slog.LevelDebug)")
	}
}

func TestLogRotates(t *testing.T) {
	dir := t.TempDir()
	w := &rotatingWriter{path: filepath.Join(dir, "poplar.log"), maxSize: 10, keep: 2}

	for range 5 {
		if _, err := w.Write([]byte("0123456789\n")); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}

	for _, name := range []string{"poplar.log", "poplar.log.1", "poplar.log.2"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
		}
	}
}
