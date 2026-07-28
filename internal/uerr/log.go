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

// openLogWriter resolves ADR-0013's log destination,
// $XDG_STATE_HOME/poplar/poplar.log (the existing
// ~/.local/state/poplar/ home per ER-4), falling back to stderr if
// the state directory can't be created.
func openLogWriter() io.Writer {
	path, err := xdg.StateFile("poplar/poplar.log")
	if err != nil {
		return os.Stderr
	}
	return &rotatingWriter{path: path, maxSize: 10 << 20, keep: 2}
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
type rotatingWriter struct {
	mu      sync.Mutex
	path    string
	maxSize int64
	keep    int
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if info, err := os.Stat(w.path); err == nil && info.Size()+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // path is built from xdg.StateFile, not user input
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	return f.Write(p)
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
