// Command poplar starts poplar's store and background writer. Pass 1
// has no screen yet, so this binary's job is the startup path: take
// the instance lock, open and migrate the store, run an integrity
// check when one is owed, offer a rebuild-from-server recovery on
// failure, reclaim whatever a previous run left mid-dispatch, and
// shut down cleanly.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "time/tzdata"

	"github.com/adrg/xdg"
	_ "modernc.org/sqlite"

	"github.com/glw907/poplar/internal/outbox"
	"github.com/glw907/poplar/internal/platform"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/uerr"
)

// flags holds poplar's command-line options.
type flags struct {
	rebuildIndex bool
	recover      bool
	startupTrace bool
}

func main() {
	uerr.SetDefault()

	var f flags
	flag.BoolVar(&f.rebuildIndex, "rebuild-index", false, "rebuild the full-text index before starting")
	flag.BoolVar(&f.recover, "recover", false, "rebuild the store from local data after detecting corruption or a failed migration")
	flag.BoolVar(&f.startupTrace, "startup-trace", false, "print exec-to-first-list-page phase timings as JSON and exit, for the QA-1 perf harness")
	flag.Parse()

	dbPath, err := xdg.DataFile("poplar/store.db")
	if err != nil {
		reportStartupFailure(os.Stderr, uerr.New("main.startup", nil, uerr.ClassStoreLocal, fmt.Errorf("resolve store path: %w", err)))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, dbPath, f, os.Stdout); err != nil {
		reportStartupFailure(os.Stderr, err)
		os.Exit(1)
	}
}

// reportStartupFailure prints err's fixed user-facing sentence to w,
// followed by its unwrapped cause when err is a uerr.Error. The
// sentence is the same fixed string for every error of its class, so
// the pid a locked instance names, or the instructions a refused
// recovery owes, live only in the cause; printing only the sentence
// leaves the operator with no actionable detail (SY-7, ADR-0015).
func reportStartupFailure(w io.Writer, err error) {
	_, _ = fmt.Fprintln(w, err)
	var uerrErr uerr.Error
	if errors.As(err, &uerrErr) && uerrErr.Cause != nil {
		_, _ = fmt.Fprintln(w, uerrErr.Cause)
	}
}

// run drives poplar's startup path against the store at dbPath: the
// instance lock, store preparation (migration, integrity check,
// recovery), the writer, the orphaned-intent sweep, and a clean
// shutdown once ctx is done. start is run's own entry time, the
// in-process origin QA-1's --startup-trace measures against.
func run(ctx context.Context, dbPath string, f flags, out io.Writer) error {
	start := time.Now()

	lock, err := platform.AcquireInstanceLock(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()

	if err := prepareStore(ctx, dbPath, f, out); err != nil {
		return err
	}

	writer, err := store.Open(dbPath, store.DefaultWriterConfig())
	if err != nil {
		return err
	}

	// The lock above is what makes this sweep unambiguous, and it runs
	// on every startup because a stranded intent fails no integrity
	// check and triggers no recovery.
	if err := outbox.ReclaimOrphaned(ctx, writer); err != nil {
		_ = writer.Close()
		return err
	}

	if f.rebuildIndex {
		_, _ = fmt.Fprintln(out, "rebuilding full-text index...")
		if err := store.RebuildIndex(ctx, writer); err != nil {
			_ = writer.Close()
			return err
		}
	}

	if f.startupTrace {
		return runStartupTrace(ctx, dbPath, writer, start, out)
	}

	_, _ = fmt.Fprintln(out, "poplar is running; press Ctrl-C to stop")
	<-ctx.Done()

	if err := writer.Close(); err != nil {
		return err
	}
	return store.MarkCleanShutdown(dbPath)
}

// startupTraceResult is the single JSON line --startup-trace prints to
// stdout: QA-1's in-process phase timings from run's own entry to a
// first mailbox-list page, in nanoseconds.
type startupTraceResult struct {
	OpenNS      int64 `json:"open_ns"`
	FirstPageNS int64 `json:"first_page_ns"`
	TotalNS     int64 `json:"total_ns"`
	Rows        int   `json:"rows"`
}

// runStartupTrace times a first mailbox-list page against the store
// writer already opened, then prints the result to out and shuts down
// without waiting on ctx: --startup-trace is a perf-harness exec, not
// an interactive session. It picks the lowest-id mailbox rather than
// exposing a mailbox selector, since pass 1 has no onboarding flow to
// name one.
func runStartupTrace(ctx context.Context, dbPath string, writer *store.Writer, start time.Time, out io.Writer) error {
	opened := time.Since(start)

	rows, err := timeFirstPage(ctx, dbPath, writer.Revision())
	total := time.Since(start)
	if err != nil {
		_ = writer.Close()
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}
	if err := store.MarkCleanShutdown(dbPath); err != nil {
		return err
	}

	return json.NewEncoder(out).Encode(startupTraceResult{
		OpenNS:      opened.Nanoseconds(),
		FirstPageNS: (total - opened).Nanoseconds(),
		TotalNS:     total.Nanoseconds(),
		Rows:        len(rows),
	})
}

// timeFirstPage opens a single-connection read pool over dbPath and
// returns a first mailbox-list page through it, closing the pool
// before it returns so the caller's elapsed time covers the whole
// read.
func timeFirstPage(ctx context.Context, dbPath string, rev *store.RevisionCounter) (rows []store.MailboxRow, err error) {
	reads, err := store.NewReadPool(dbPath, 1, rev)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := reads.Close()
		if err == nil && closeErr != nil {
			err = uerr.New("main.startup-trace", nil, uerr.ClassStoreLocal, fmt.Errorf("close read pool: %w", closeErr))
		}
	}()

	mailboxID, err := reads.FirstMailboxID(ctx)
	if err != nil {
		return nil, err
	}
	page, err := reads.ListMailboxForward(ctx, mailboxID, store.MailboxCursor{}, 50)
	if err != nil {
		return nil, err
	}
	return page.Rows, nil
}

// prepareStore ensures the store at dbPath is migrated and, when one
// is owed, passes its integrity check, offering a rebuild-from-local
// recovery on either failure rather than running one unasked (SY-8).
// It holds no connection open past its own return; store.Open reopens
// dbPath once prepareStore reports success.
func prepareStore(ctx context.Context, dbPath string, f flags, out io.Writer) error {
	db, err := store.OpenWriteConn(dbPath)
	if err != nil {
		return err
	}

	before, err := store.CurrentSchemaVersion(db)
	if err != nil {
		_ = db.Close()
		return err
	}
	if err := store.Migrate(db); err != nil {
		_ = db.Close()
		return offerRecovery(ctx, dbPath, f.recover, err, out)
	}
	after, err := store.CurrentSchemaVersion(db)
	if err != nil {
		_ = db.Close()
		return err
	}

	if !store.NeedsIntegrityCheck(dbPath, after != before) && !f.rebuildIndex {
		return db.Close()
	}
	if err := store.CheckIntegrity(ctx, db, integrityProgress(out)); err != nil {
		_ = db.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return offerRecovery(ctx, dbPath, f.recover, err, out)
	}
	return db.Close()
}

// offerRecovery reports cause and, when allowed, rebuilds the store
// at dbPath from its non-rebuildable local data (SY-8). Recovery is
// an offer the operator accepts through --recover, never an automatic
// response to a detected failure: refused, it prints how to accept
// and returns cause unchanged so the store is left exactly as found.
// A cause with no ClassStoreLocal, a newer store's ClassSchemaVersion
// most notably, is not this recovery's business and propagates as is.
func offerRecovery(ctx context.Context, dbPath string, allowed bool, cause error, out io.Writer) error {
	var uerrErr uerr.Error
	if !errors.As(cause, &uerrErr) || uerrErr.Class != uerr.ClassStoreLocal {
		return cause
	}
	if !allowed {
		_, _ = fmt.Fprintf(out, "store needs recovery: %v\nrerun with --recover to rebuild from local data (the outbox and drafts are preserved)\n", cause)
		return cause
	}

	_, _ = fmt.Fprintf(out, "rebuilding from local data: %v\n", cause)
	counts, err := store.Recover(context.WithoutCancel(ctx), dbPath)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "rebuilt store: %d outbox intent(s) and %d local message(s) preserved\n", counts.Outbox, counts.Messages)
	return nil
}

// integrityProgress returns a store.CheckIntegrity progress callback
// that prints each stage to out, the visible progress state a
// multi-second quick_check owes the operator (QA-1).
func integrityProgress(out io.Writer) func(string) {
	return func(stage string) {
		_, _ = fmt.Fprintln(out, stage+"...")
	}
}
