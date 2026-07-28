package uerr

import (
	"fmt"
	"io"
	"log/slog"
	"os"
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

// logger is uerr's log destination, built once per process. It is
// unexported package state, not a public hook; a test needing a
// redirected destination reassigns it directly.
var logger = sync.OnceValue(func() *slog.Logger {
	return slog.New(slog.NewJSONHandler(openLogWriter(), &slog.HandlerOptions{Level: level}))
})

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
// ~/.local/state/poplar/ home per ER-4), falling back to stderr if
// the state directory can't be created. It records the writer so
// LogHealth can report a write failure later.
func openLogWriter() io.Writer {
	path, err := xdg.StateFile("poplar/poplar.log")
	if err != nil {
		return os.Stderr
	}
	w := &rotatingWriter{path: path, maxSize: 10 << 20, keep: 2}
	logWriter = w
	return w
}

// logWriter is the process's rotatingWriter, set by openLogWriter
// when the log destination is a file. LogHealth reads it. It stays
// nil when the log falls back to stderr, which never fails silently
// on its own.
var logWriter *rotatingWriter

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

	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // G304: w.path comes from xdg.StateFile or a test fixture, never user input
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
