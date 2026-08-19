package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
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
	// WriteCeiling is the admission ceiling a single transaction is
	// expected to stay under (ADR-0003 revision 2, CO-6). execute
	// does not cancel a transaction that runs past it, since
	// interrupting a live *sql.Tx mid-statement risks a stuck
	// connection; it logs instead, so a caller that runs over the
	// ceiling is visible rather than silently slow.
	WriteCeiling time.Duration
}

// DefaultWriterConfig returns poplar's production writer timing.
func DefaultWriterConfig() WriterConfig {
	return WriterConfig{
		InteractiveQuiet: time.Second,
		CheckpointIdle:   3 * time.Second,
		JournalSizeLimit: 8 << 20,
		WriteCeiling:     50 * time.Millisecond,
	}
}

// errWriterClosed is returned to a submit or submitBulk call racing
// with Close.
var errWriterClosed = errors.New("store: writer is closed")

type writeJob struct {
	fn   func(*sql.Tx) error
	done chan error
}

// Writer is poplar's single write connection, run by one goroutine
// serving two lanes (ADR-0003): submit for interactive, user-facing
// intents, and submitBulk for chunked background work such as a body
// backfill. A queued bulk backlog never delays an interactive job
// past the chunk boundary the writer is currently on. Both methods
// are unexported: nothing outside internal/store reaches a write
// entry point except through the intent gateway (ADR-0003, the
// package-boundary rule the writecall analyzer enforces).
type Writer struct {
	db  *sql.DB
	cfg WriterConfig

	interactive chan writeJob
	bulk        chan writeJob
	stop        chan struct{}
	done        chan struct{}
	closeOnce   sync.Once
	closeErr    error

	lastInteractive atomic.Int64 // UnixNano of the last interactive job the writer ran
	rev             RevisionCounter
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
		return nil, localErr("store.writer", err)
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

// Open opens the store at path, migrates it, and starts the writer
// over the result: the one call outside internal/store allowed to
// reach the writer's construction, since it does the open-and-migrate
// work NewWriter's caller must already have done, rather than merely
// relaying to NewWriter under another name (ADR-0003, the write-call
// analyzer). cmd/poplar's startup path is its only caller.
func Open(path string, cfg WriterConfig) (*Writer, error) {
	db, err := OpenWriteConn(path)
	if err != nil {
		return nil, err
	}
	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	w, err := NewWriter(db, cfg)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return w, nil
}

// submit runs fn in a transaction on the interactive lane and
// returns once it commits or fails. An error fn returns rolls back
// the whole transaction, so a failure partway through never leaves a
// partial write.
func (w *Writer) submit(ctx context.Context, fn func(*sql.Tx) error) error {
	return w.enqueue(ctx, w.interactive, fn)
}

// submitBulk runs fn as one chunk on the bulk lane, with the same
// all-or-nothing commit as submit. The caller chunks its own work
// (roughly 50ms per call) and consults RecentInteractiveActivity
// between chunks so a long bulk job yields to interactive use.
func (w *Writer) submitBulk(ctx context.Context, fn func(*sql.Tx) error) error {
	return w.enqueue(ctx, w.bulk, fn)
}

// enqueue hands fn to lane and waits for it to run. Once fn is
// admitted onto lane, enqueue always waits for its outcome rather
// than racing ctx.Done(): a caller whose context is cancelled after
// admission still learns the true result instead of seeing a
// cancellation error for a write that lands anyway.
func (w *Writer) enqueue(ctx context.Context, lane chan writeJob, fn func(*sql.Tx) error) error {
	j := writeJob{fn: fn, done: make(chan error, 1)}
	select {
	case lane <- j:
	case <-ctx.Done():
		return localErr("store.write", ctx.Err())
	case <-w.stop:
		return localErr("store.write", errWriterClosed)
	}
	return <-j.done
}

// Revision returns the store's revision counter, advanced once for
// every transaction this writer commits. A ReadPool opened over the
// same store file shares it, so every read result carries the
// revision it saw (SY-1's read path).
func (w *Writer) Revision() *RevisionCounter {
	return &w.rev
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
// db. A job already admitted runs to completion first. It is safe to
// call more than once: a test that closes the writer mid-test to
// release the file for a recovery rebuild still wants its t.Cleanup
// close to run without a double-close panic on w.stop.
func (w *Writer) Close() error {
	w.closeOnce.Do(func() {
		close(w.stop)
		<-w.done
		w.closeErr = w.db.Close()
	})
	return w.closeErr
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
			w.runIdleCheckpoint()
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
	// A failed checkpoint only misses this chunk's reclaim; the next
	// chunk or the idle TRUNCATE tries again, so the writer logs and
	// moves on rather than surfacing it to the caller whose job
	// already committed.
	if err := checkpoint(context.Background(), w.db, "PASSIVE"); err != nil {
		slog.Warn("store: checkpoint failed", "mode", "PASSIVE", "error", err)
	}
}

// runIdleCheckpoint runs a bounded incremental_vacuum step, followed
// by the idle-triggered TRUNCATE checkpoint that returns its freed
// pages to the OS. In WAL mode, incremental_vacuum's page moves land
// in the WAL, not the main file; running it before the TRUNCATE
// checkpoint lets that same checkpoint flush and shrink the vacuum's
// result in one idle window, rather than leaving the main file
// unshrunk until the next. TRUNCATE is the one checkpoint mode that
// can block on a reader's open snapshot, so it runs under a
// busy_timeout far shorter than the connection's normal 5s bound: a
// reader outliving that short window leaves the WAL larger than its
// bound until the next idle window, rather than delaying a queued
// interactive job for seconds.
func (w *Writer) runIdleCheckpoint() {
	if err := incrementalVacuum(context.Background(), w.db, incrementalVacuumPages); err != nil {
		slog.Warn("store: incremental vacuum failed", "error", err)
	}
	if err := checkpointTruncate(context.Background(), w.db); err != nil {
		slog.Warn("store: checkpoint failed", "mode", "TRUNCATE", "error", err)
	}
}

func (w *Writer) execute(fn func(*sql.Tx) error) error {
	start := time.Now()
	defer w.warnIfOverCeiling(start)

	tx, err := w.db.Begin()
	if err != nil {
		return localErr("store.write", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return localErr("store.write", err)
	}
	if err := tx.Commit(); err != nil {
		return localErr("store.write", err)
	}
	w.rev.advance()
	return nil
}

// warnIfOverCeiling logs when a transaction that started at start ran
// past WriteCeiling. It does not classify or surface an error: a slow
// transaction still committed (or failed and rolled back) on its own
// merits, and this is visibility into the admission ceiling, not a
// new failure mode.
func (w *Writer) warnIfOverCeiling(start time.Time) {
	if elapsed := time.Since(start); elapsed > w.cfg.WriteCeiling {
		slog.Warn("store: transaction exceeded admission ceiling", "elapsed", elapsed, "ceiling", w.cfg.WriteCeiling)
	}
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
