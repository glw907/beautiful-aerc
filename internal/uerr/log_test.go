package uerr

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"
)

// captureLog redirects logger to a buffer for the test's duration and
// returns it. internal/uerr/uerrtest's Capture is this same
// redirection (through RedirectForTest) for a test outside this
// package.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	t.Cleanup(RedirectForTest(&buf))
	return &buf
}

// decodeLogLine asserts buf holds exactly one JSON log line. That is
// the one-line-per-outcome rule ADR-0013 revision 2 states. It
// returns the line's fields.
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

// TestSetDefaultInstallsLogger proves ER-2's route: once SetDefault
// runs, a bare log/slog call anywhere in the process reaches uerr's
// destination and honors uerr's shared level, instead of falling to
// slog's default text handler on stderr.
func TestSetDefaultInstallsLogger(t *testing.T) {
	buf := captureLog(t)

	origDefault := slog.Default()
	t.Cleanup(func() { slog.SetDefault(origDefault) })
	SetDefault()

	slog.Debug("trace", slog.String("op", "test"))
	if buf.Len() != 0 {
		t.Fatalf("slog.Debug logged at the default level: %q", buf.String())
	}

	SetLevel(slog.LevelDebug)
	slog.Debug("trace", slog.String("op", "test"))

	rec := decodeLogLine(t, buf)
	if rec["msg"] != "trace" {
		t.Errorf("msg = %v, want %q", rec["msg"], "trace")
	}
	if rec["op"] != "test" {
		t.Errorf("op = %v, want %q", rec["op"], "test")
	}
}

// TestRotatingWriterRecordsFailure proves a write failure is not
// silently discarded: it drives Write against a path that can never
// open (a directory) and checks the sticky failure record health
// exposes.
func TestRotatingWriterRecordsFailure(t *testing.T) {
	dir := t.TempDir()
	w := &rotatingWriter{path: dir, maxSize: 1 << 20, keep: 2}

	n, err := w.Write([]byte("line\n"))
	if err == nil {
		t.Fatal("Write against a directory path succeeded, want an error")
	}
	if n != 0 {
		t.Errorf("Write returned n = %d, want 0", n)
	}

	gotDropped, gotErr := w.health()
	if gotErr == nil {
		t.Error("health() err = nil, want the write failure")
	}
	if gotDropped != 1 {
		t.Errorf("health() dropped = %d, want 1", gotDropped)
	}
}

// TestLogHealthReportsWriterFailure proves LogHealth surfaces a
// failing writer's state without going through slog, which would
// otherwise discard it. captureLog redirects logger so LogHealth's
// forced call to it cannot reset logWriter out from under the test.
func TestLogHealthReportsWriterFailure(t *testing.T) {
	captureLog(t)

	orig := logWriter
	t.Cleanup(func() { logWriter = orig })

	dir := t.TempDir()
	w := &rotatingWriter{path: dir, maxSize: 1 << 20, keep: 2}
	if _, err := w.Write([]byte("line\n")); err == nil {
		t.Fatal("Write against a directory path succeeded, want an error")
	}
	logWriter = w

	dropped, err := LogHealth()
	if err == nil {
		t.Error("LogHealth() err = nil, want the write failure")
	}
	if dropped != 1 {
		t.Errorf("LogHealth() dropped = %d, want 1", dropped)
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
