package uerr

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/adrg/xdg"
)

// level is uerr's log level. slog.LevelInfo is the default; SetLevel
// opts a session into slog.LevelDebug.
var level = new(slog.LevelVar)

// SetLevel sets uerr's log level.
func SetLevel(l slog.Level) {
	level.Set(l)
}

// logHandle builds uerr's log destination once per process.
// RedirectForTest swaps it for one over a test's buffer.
var logHandle = sync.OnceValue(func() *slog.Logger {
	return slog.New(slog.NewJSONHandler(openLogWriter(), &slog.HandlerOptions{Level: level}))
})

// logMu guards logHandle. Every New call reads it and RedirectForTest
// writes it, so a background engine goroutine logging while a test
// redirects would otherwise race.
var logMu sync.Mutex

// logger returns uerr's log destination.
func logger() *slog.Logger {
	logMu.Lock()
	defer logMu.Unlock()
	return logHandle()
}

// RedirectForTest points uerr's log destination at w and returns a
// restore func that puts the prior destination back. It is the only
// route by which a package outside internal/uerr can observe what
// uerr.New actually wrote: New's destination is package-private
// state, so without this hook no ER-1 assertion anywhere else in the
// tree (a caller's dedup discipline under ADR-0013 revision 2, most
// notably) can tell a construction happened from one that never did.
// internal/uerr/uerrtest wraps this for a *testing.T caller.
//
// It is a test hook, and a production call would divert every error
// line in the process for its lifetime. scripts/hookcheck fails the
// build on a reference to it from any non-test file outside this
// package.
func RedirectForTest(w io.Writer) (restore func()) {
	logMu.Lock()
	defer logMu.Unlock()

	origHandle, origLevel := logHandle, level.Level()
	logHandle = sync.OnceValue(func() *slog.Logger {
		return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
	})
	return func() {
		logMu.Lock()
		defer logMu.Unlock()
		logHandle = origHandle
		level.Set(origLevel)
	}
}

// SetDefault installs uerr's logger as slog's process-wide default.
// Every other package logs through plain log/slog calls per
// go-conventions, including ER-2's debug-level action trace. Before
// SetDefault runs, those calls reach slog's default text handler on
// stderr instead, scribbling over a full-screen bubbletea UI.
// cmd/poplar's startup path calls SetDefault once, before anything
// else logs.
func SetDefault() {
	slog.SetDefault(logger())
}

// openLogWriter resolves ADR-0013's log destination,
// $XDG_STATE_HOME/poplar/poplar.log (the existing
// ~/.local/state/poplar/ home per ER-4). xdg.StateFile only resolves
// a path string, creating the containing directory if it is missing;
// it never proves the file itself is writable, so a state directory
// that exists but denies write (mode 0500, a read-only home, a full
// quota) resolves happily and only reveals itself the first time
// something tries to write. tryOpen's trial write is what catches
// that before the fallback decision is made, not xdg.StateFile's
// error alone (dispositions row 24, C2).
//
// When the state destination fails its trial, the fallback is a file
// in the process's temp directory, never stderr: a bubbletea UI
// may already own the terminal by the time this runs, and a JSON log
// line scribbled into it is exactly the corruption row 24 declined to
// fix while poplar shipped no screen. The fallback's engagement
// logs once, through itself, so the line lands in the destination it
// names (C1). If the temp-dir file fails its trial too,
// logFallbackPath stays unset rather than naming a path nothing
// reaches (m1); logDegraded records that instead, and LogHealth
// already carries the trial's drop count with no further write
// needed to surface it.
func openLogWriter() io.Writer {
	if primary, err := xdg.StateFile("poplar/poplar.log"); err == nil {
		if w, ok := tryOpen(primary); ok {
			logWriter = w
			return w
		}
	}

	fallback := filepath.Join(os.TempDir(), "poplar.log")
	w, ok := tryOpen(fallback)
	logWriter = w
	if !ok {
		logDegraded = true
		return w
	}

	logFallbackPath = fallback
	slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})).Warn(
		"uerr: state directory unavailable, logging to the temp-dir fallback", "path", fallback)
	return w
}

// tryOpen builds a rotatingWriter over path and proves it can
// actually be written by driving a zero-byte probe through the same
// Write path every real log line takes: a payload of nil never
// triggers rotation and, on success, leaves w's dropped count and
// lastErr untouched, but a failure records both exactly as a real
// line's failure would, so a caller that keeps a failed probe's
// writer (openLogWriter's fallback branch, never its primary one)
// finds LogHealth already truthful with no further write required.
func tryOpen(path string) (*rotatingWriter, bool) {
	w := &rotatingWriter{path: path, maxSize: 10 << 20, keep: 2}
	_, err := w.Write(nil)
	return w, err == nil
}

// logWriter is the process's rotatingWriter, set by openLogWriter.
// LogHealth reads it.
var logWriter *rotatingWriter

// logFallbackPath holds the temp-dir path openLogWriter fell back to
// and confirmed writable, or "" when the log is at its normal
// state-dir home, or when the fallback itself failed its trial
// (logDegraded covers that case instead). LogFallbackPath reads it.
var logFallbackPath string

// logDegraded records that openLogWriter's fallback engaged but its
// own trial write still failed: neither destination works, and there
// is no path left to name (m1). LogDegraded reads it.
var logDegraded bool

// LogFallbackPath reports the temp-dir path uerr's log fell back to
// when the state directory's log file could not be written, and
// whether that fallback is in effect (dispositions row 24,
// ER-3): a caller uses this to warn the operator once, since the
// fallback itself writes silently otherwise. It reports false, not a
// path nothing reaches, when the fallback itself is also unwritable;
// LogDegraded is that case's signal.
//
// LogFallbackPath forces logger's one-time build to run first, so its
// read of logFallbackPath is ordered after openLogWriter's write to
// it by sync.OnceValue rather than racing it.
func LogFallbackPath() (path string, ok bool) {
	logger()
	return logFallbackPath, logFallbackPath != ""
}

// LogDegraded reports whether uerr's log destination failed both its
// state-dir home and its temp-dir fallback's trial write
// (dispositions row 24, m1): a caller has nowhere true to point a
// LogFallbackPath at, and should say logging is degraded rather than
// naming a path nothing reaches.
//
// LogDegraded forces logger's one-time build to run first, the same
// ordering guarantee LogFallbackPath and LogHealth rely on.
func LogDegraded() bool {
	logger()
	return logDegraded
}

// SetLogFallbackForTest sets the state LogFallbackPath and LogDegraded
// report and returns a restore func: uerr's own test seam for that
// pair, the same shape RedirectForTest already gives a caller outside
// this package for the log destination itself, so a test can drive
// logFallbackPath/logDegraded's two states without engineering a real
// unwritable filesystem to trigger openLogWriter's own probes.
func SetLogFallbackForTest(path string, degraded bool) func() {
	origPath, origDegraded := logFallbackPath, logDegraded
	logFallbackPath, logDegraded = path, degraded
	return func() { logFallbackPath, logDegraded = origPath, origDegraded }
}

// LogHealth reports uerr's log writer's most recent write failure,
// if any, and the number of log lines dropped because of it. slog
// discards a handler's write error, so this is the only route to
// SY-8's "disk-full during any write degrades visibly" for the log
// itself. A caller checks it at startup or on a periodic health pass;
// the writer itself never blocks or panics on a write failure.
//
// LogHealth forces logger's one-time build to run first, so its read
// of logWriter is ordered after openLogWriter's write to it by
// sync.OnceValue rather than racing it.
func LogHealth() (dropped int, err error) {
	logger()
	if logWriter == nil {
		return 0, nil
	}
	return logWriter.health()
}

// logError writes e's log line: operation, ids, class, cause, and
// the user sentence, so the line correlates with what the UI showed.
// ADR-0013 revision 2's rule holds here: New is called once per
// surfacing event, never per retry attempt, so this is one log line
// per outcome.
func logError(e Error) {
	logger().Error(e.Op,
		slog.Any("ids", e.IDs),
		slog.String("class", e.Class.String()),
		slog.String("cause", causeText(e.Cause)),
		slog.String("message", e.Message),
	)
}

func causeText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// rotatingWriter is a size-based log rotator: ADR-0013 rejects a
// rotation dependency for what a handful of renames covers. At
// maxSize it shifts path -> path.1 -> path.2 and so on up to keep
// backups, dropping the oldest.
//
// slog.Handler.Handle discards whatever error Write returns, so a
// disk-full or permission failure would otherwise vanish with no
// trace anywhere, defeating the seam this package exists to hold
// (SY-8). fail records the last such error and a running dropped-line
// count under mu, so LogHealth can surface it even though slog never
// sees it.
type rotatingWriter struct {
	mu      sync.Mutex
	path    string
	maxSize int64
	keep    int
	lastErr error
	dropped int
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if info, err := os.Stat(w.path); err == nil && info.Size()+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return w.fail(err)
		}
	}

	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // G304: w.path comes from xdg.StateFile, its temp-dir fallback, or a test fixture, never user input
	if err != nil {
		return w.fail(err)
	}
	defer func() { _ = f.Close() }()

	n, err := f.Write(p)
	if err != nil {
		return w.fail(err)
	}
	if n < len(p) {
		return w.fail(io.ErrShortWrite)
	}
	return n, nil
}

// fail records err as w's last write failure, counts the line as
// dropped, and returns the (0, error) pair Write reports to its
// caller.
func (w *rotatingWriter) fail(err error) (int, error) {
	w.lastErr = err
	w.dropped++
	return 0, err
}

// health returns w's dropped-line count and its last write failure,
// if any, both under mu since Write updates them concurrently.
func (w *rotatingWriter) health() (dropped int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dropped, w.lastErr
}

func (w *rotatingWriter) rotate() error {
	for i := w.keep; i >= 1; i-- {
		src := w.path
		if i > 1 {
			src = w.backupPath(i - 1)
		}
		if err := os.Rename(src, w.backupPath(i)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (w *rotatingWriter) backupPath(n int) string {
	return fmt.Sprintf("%s.%d", w.path, n)
}
