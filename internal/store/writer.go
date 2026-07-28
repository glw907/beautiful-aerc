package store

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"time"

	"github.com/glw907/poplar/internal/uerr"
)

// WriterConfig governs the writer's admission and checkpoint timing.
// DefaultWriterConfig holds poplar's production values; tests shrink
// the windows to keep the suite fast.
type WriterConfig struct {
	// InteractiveQuiet is the window RecentInteractiveActivity
	// consults: a bulk caller (the future backfill worker) yields
	// while an interactive commit landed within this window
	// (ADR-0003 revision 2).
	InteractiveQuiet time.Duration
	// CheckpointIdle is how long the writer waits with no committed
	// job, on either lane, before running a TRUNCATE checkpoint.
	CheckpointIdle time.Duration
	// JournalSizeLimit bounds the WAL file in bytes.
	JournalSizeLimit int64
}

// DefaultWriterConfig returns poplar's production writer timing.
func DefaultWriterConfig() WriterConfig {
	return WriterConfig{
		InteractiveQuiet: time.Second,
		CheckpointIdle:   3 * time.Second,
		JournalSizeLimit: 8 << 20,
	}
}

// errWriterClosed is returned to a Submit or SubmitBulk call racing
// with Close.
var errWriterClosed = errors.New("store: writer is closed")

type writeJob struct {
	fn   func(*sql.Tx) error
	done chan error
}

// Writer is poplar's single write connection, run by one goroutine
// serving two lanes (ADR-0003): Submit for interactive, user-facing
// intents, and SubmitBulk for chunked background work such as a body
// backfill. A queued bulk backlog never delays an interactive job
// past the chunk boundary the writer is currently on.
type Writer struct {
	db  *sql.DB
	cfg WriterConfig

	interactive chan writeJob
	bulk        chan writeJob
	stop        chan struct{}
	done        chan struct{}

	lastInteractive atomic.Int64 // UnixNano of the last interactive job the writer ran
}

// NewWriter starts the writer goroutine over db, which must be the
// store's single write connection (opened through dsn with
// connReadWrite). NewWriter pins db to one physical connection,
// since modernc's connections are full-mutex and a second one would
// only queue behind the first, and disables db's automatic WAL
// checkpoint: from here on every checkpoint is one this Writer runs
// itself.
func NewWriter(db *sql.DB, cfg WriterConfig) (*Writer, error) {
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := configureCheckpointing(context.Background(), db, checkpointConfig{
		JournalSizeLimit: cfg.JournalSizeLimit,
	}); err != nil {
		return nil, uerr.New("store.writer", nil, uerr.ClassStoreLocal, err)
	}

	w := &Writer{
		db:          db,
		cfg:         cfg,
		interactive: make(chan writeJob),
		bulk:        make(chan writeJob),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
	go w.run()
	return w, nil
}

// Submit runs fn in a transaction on the interactive lane and
// returns once it commits or fails. An error fn returns rolls back
// the whole transaction, so a failure partway through never leaves a
// partial write.
func (w *Writer) Submit(ctx context.Context, fn func(*sql.Tx) error) error {
	return w.enqueue(ctx, w.interactive, fn)
}

// SubmitBulk runs fn as one chunk on the bulk lane, with the same
// all-or-nothing commit as Submit. The caller chunks its own work
// (roughly 50ms per call) and consults RecentInteractiveActivity
// between chunks so a long bulk job yields to interactive use.
func (w *Writer) SubmitBulk(ctx context.Context, fn func(*sql.Tx) error) error {
	return w.enqueue(ctx, w.bulk, fn)
}

func (w *Writer) enqueue(ctx context.Context, lane chan writeJob, fn func(*sql.Tx) error) error {
	j := writeJob{fn: fn, done: make(chan error, 1)}
	select {
	case lane <- j:
	case <-ctx.Done():
		return ctx.Err()
	case <-w.stop:
		return errWriterClosed
	}
	select {
	case err := <-j.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RecentInteractiveActivity reports whether an interactive job ran
// within quiet of now. A bulk caller consults this before starting
// its next chunk rather than the lane's queue depth, which is empty
// exactly when the writer is busy running the previous chunk
// (ADR-0003 revision 2).
func (w *Writer) RecentInteractiveActivity(quiet time.Duration) bool {
	last := w.lastInteractive.Load()
	if last == 0 {
		return false
	}
	return time.Since(time.Unix(0, last)) < quiet
}

// Close stops the writer goroutine, waits for it to exit, and closes
// db. A job already admitted runs to completion first.
func (w *Writer) Close() error {
	close(w.stop)
	<-w.done
	return w.db.Close()
}

func (w *Writer) run() {
	defer close(w.done)

	idle := time.NewTimer(w.cfg.CheckpointIdle)
	defer idle.Stop()

	for {
		// A non-blocking check first, so an interactive job already
		// waiting on the lane always runs before the writer considers
		// the next bulk chunk; the blocking select below can only
		// pick bulk when interactive was empty at this instant.
		select {
		case j := <-w.interactive:
			w.runInteractive(j)
			resetTimer(idle, w.cfg.CheckpointIdle)
			continue
		default:
		}

		select {
		case j := <-w.interactive:
			w.runInteractive(j)
			resetTimer(idle, w.cfg.CheckpointIdle)
		case j := <-w.bulk:
			w.runBulk(j)
			resetTimer(idle, w.cfg.CheckpointIdle)
		case <-idle.C:
			// A failed checkpoint only misses this round's growth
			// bound; it is not a correctness issue, so the writer
			// tries again at the next idle window rather than
			// surfacing it.
			_ = checkpoint(context.Background(), w.db, "TRUNCATE")
			idle.Reset(w.cfg.CheckpointIdle)
		case <-w.stop:
			return
		}
	}
}

func (w *Writer) runInteractive(j writeJob) {
	w.lastInteractive.Store(time.Now().UnixNano())
	j.done <- w.execute(j.fn)
}

func (w *Writer) runBulk(j writeJob) {
	j.done <- w.execute(j.fn)
	_ = checkpoint(context.Background(), w.db, "PASSIVE")
}

func (w *Writer) execute(fn func(*sql.Tx) error) error {
	tx, err := w.db.Begin()
	if err != nil {
		return uerr.New("store.write", nil, uerr.ClassStoreLocal, err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return uerr.New("store.write", nil, uerr.ClassStoreLocal, err)
	}
	if err := tx.Commit(); err != nil {
		return uerr.New("store.write", nil, uerr.ClassStoreLocal, err)
	}
	return nil
}

func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}
