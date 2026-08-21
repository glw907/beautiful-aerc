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

// resetLogFallbackState saves uerr's fallback-tracking package vars
// and returns a restore func, so a test that drives openLogWriter
// directly (bypassing the process-wide logHandle singleton, exactly
// as these tests already do) leaves none of that state behind for a
// later test in this package to trip over.
func resetLogFallbackState(t *testing.T) {
	t.Helper()
	origPath, origWriter, origDegraded := logFallbackPath, logWriter, logDegraded
	t.Cleanup(func() { logFallbackPath, logWriter, logDegraded = origPath, origWriter, origDegraded })
}

// TestLogFallsBackToTempDirWhenStateDirUnavailable proves row 24's
// rework: when $XDG_STATE_HOME's own poplar.log can't be resolved,
// openLogWriter falls back to a file in the process's own temp
// directory instead of stderr, LogFallbackPath reports it so a caller
// can warn the operator (ER-3), and the fallback's own engagement
// logs once through itself (C1), landing in the file it names rather
// than wherever slog's process-wide default happens to point.
func TestLogFallsBackToTempDirWhenStateDirUnavailable(t *testing.T) {
	resetLogFallbackState(t)

	home := t.TempDir()
	// A file where $XDG_STATE_HOME/poplar would need to be a
	// directory: every candidate xdg.StateFile tries fails the same
	// way an unwritable or missing state directory would.
	if err := os.MkdirAll(filepath.Join(home, ".local"), 0o750); err != nil {
		t.Fatalf("create the .local directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".local", "state"), nil, 0o600); err != nil {
		t.Fatalf("seed a file where the state directory should be: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	xdg.Reload()
	t.Cleanup(xdg.Reload)

	wantPath := filepath.Join(os.TempDir(), "poplar.log")
	t.Cleanup(func() { _ = os.Remove(wantPath) })

	w, ok := openLogWriter().(*rotatingWriter)
	if !ok {
		t.Fatalf("openLogWriter() = %T, want *rotatingWriter", openLogWriter())
	}
	if w.path != wantPath {
		t.Errorf("fallback log path = %q, want %q", w.path, wantPath)
	}

	path, ok := LogFallbackPath()
	if !ok || path != wantPath {
		t.Errorf("LogFallbackPath() = (%q, %v), want (%q, true)", path, ok, wantPath)
	}
	if LogDegraded() {
		t.Error("LogDegraded() = true, want false: the fallback engaged and works")
	}
	if dropped, err := LogHealth(); dropped != 0 || err != nil {
		t.Errorf("LogHealth() = (%d, %v), want (0, nil): the engagement line is the only write so far, and it succeeded", dropped, err)
	}

	logged, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read the fallback log: %v", err)
	}
	if !strings.Contains(string(logged), "state directory unavailable") {
		t.Errorf("fallback log = %q, want the engagement line landed in it (C1)", logged)
	}
}

// TestLogFallsBackWhenStateDirIsUnwritable proves C2: a state
// directory that resolves (xdg.StateFile succeeds; nothing is missing
// or a file where a directory belongs) but denies write, mode 0500
// standing in for a read-only home or a filesystem at quota, still
// falls back rather than silently dropping every line until an
// operator notices at exit. xdg.StateFile alone cannot catch this: it
// only resolves a path string and creates the containing directory if
// missing, never proving the file itself is writable.
func TestLogFallsBackWhenStateDirIsUnwritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permission checks; this probe needs a real denial")
	}
	resetLogFallbackState(t)

	home := t.TempDir()
	stateDir := filepath.Join(home, ".local", "state", "poplar")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("create the state directory: %v", err)
	}
	// chmod after creation, not as MkdirAll's own mode: MkdirAll needs
	// write access into each level it creates, poplar/ included, so a
	// 0500 mode has to land only once the directory already exists.
	if err := os.Chmod(stateDir, 0o500); err != nil { //nolint:gosec // G302: stateDir is a directory, not a file; 0500 denies write while keeping the execute bit traversal needs, which G302's file-mode heuristic does not account for
		t.Fatalf("deny write on the state directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(stateDir, 0o700) }) //nolint:gosec // G302: stateDir is a directory; restoring 0700 so t.TempDir's own cleanup can remove it
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	xdg.Reload()
	t.Cleanup(xdg.Reload)

	wantPath := filepath.Join(os.TempDir(), "poplar.log")
	t.Cleanup(func() { _ = os.Remove(wantPath) })

	w, ok := openLogWriter().(*rotatingWriter)
	if !ok {
		t.Fatalf("openLogWriter() = %T, want *rotatingWriter", openLogWriter())
	}
	if w.path != wantPath {
		t.Errorf("fallback log path = %q, want %q (a mode-0500 state directory must still fall back)", w.path, wantPath)
	}
	if path, ok := LogFallbackPath(); !ok || path != wantPath {
		t.Errorf("LogFallbackPath() = (%q, %v), want (%q, true)", path, ok, wantPath)
	}
}

// TestLogDegradedWhenBothDestinationsFail proves row 24's remaining
// half: when the temp-dir fallback itself can't be written either (a
// bare TMPDIR that does not exist, standing in for a fully unwritable
// temp directory), openLogWriter never tries a third destination.
// LogDegraded reports it, LogFallbackPath reports no usable path
// rather than naming one that receives nothing (m1), and LogHealth
// already shows the drop from the fallback's own trial write, with no
// further write needed to surface it.
func TestLogDegradedWhenBothDestinationsFail(t *testing.T) {
	resetLogFallbackState(t)

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".local"), 0o750); err != nil {
		t.Fatalf("create the .local directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".local", "state"), nil, 0o600); err != nil {
		t.Fatalf("seed a file where the state directory should be: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "does-not-exist"))
	xdg.Reload()
	t.Cleanup(xdg.Reload)

	if _, ok := openLogWriter().(*rotatingWriter); !ok {
		t.Fatalf("openLogWriter() = %T, want *rotatingWriter", openLogWriter())
	}

	if !LogDegraded() {
		t.Error("LogDegraded() = false, want true: both destinations are unusable")
	}
	if path, ok := LogFallbackPath(); ok {
		t.Errorf("LogFallbackPath() = (%q, true), want ok=false: nothing is actually receiving lines", path)
	}

	dropped, err := LogHealth()
	if err == nil {
		t.Error("LogHealth() err = nil, want the trial write's own failure")
	}
	if dropped != 1 {
		t.Errorf("LogHealth() dropped = %d, want 1 (the fallback's own trial write, not a second write on top of it)", dropped)
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
