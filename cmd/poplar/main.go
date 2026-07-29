// Command poplar starts poplar's store and background writer. Pass 1
// has no screen yet, so this binary's job is the startup path: take
// the instance lock, open and migrate the store, run an integrity
// check when one is owed, offer a rebuild-from-server recovery on
// failure, and shut down cleanly.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	_ "time/tzdata"

	"github.com/adrg/xdg"
	_ "modernc.org/sqlite"

	"github.com/glw907/poplar/internal/platform"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/uerr"
)

// flags holds poplar's command-line options.
type flags struct {
	rebuildIndex bool
}

func main() {
	uerr.SetDefault()

	var f flags
	flag.BoolVar(&f.rebuildIndex, "rebuild-index", false, "rebuild the full-text index before starting")
	flag.Parse()

	dbPath, err := xdg.DataFile("poplar/store.db")
	if err != nil {
		fmt.Fprintln(os.Stderr, fmt.Errorf("resolve store path: %w", err))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, dbPath, f, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run drives poplar's startup path against the store at dbPath: the
// instance lock, store preparation (migration, integrity check,
// recovery), the writer, and a clean shutdown once ctx is done.
func run(ctx context.Context, dbPath string, f flags, out io.Writer) error {
	lock, err := platform.AcquireInstanceLock(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()

	db, err := prepareStore(ctx, dbPath, f.rebuildIndex, out)
	if err != nil {
		return err
	}

	writer, err := store.StartWriter(db, store.DefaultWriterConfig())
	if err != nil {
		_ = db.Close()
		return err
	}

	if f.rebuildIndex {
		_, _ = fmt.Fprintln(out, "rebuilding full-text index...")
		if err := store.RebuildIndex(ctx, writer); err != nil {
			_ = writer.Close()
			return err
		}
	}

	_, _ = fmt.Fprintln(out, "poplar is running; press Ctrl-C to stop")
	<-ctx.Done()

	if err := writer.Close(); err != nil {
		return err
	}
	return store.MarkCleanShutdown(dbPath)
}

// prepareStore opens dbPath, migrates it, and runs an integrity check
// when NeedsIntegrityCheck or forced says one is owed, rebuilding from
// local data on either a failed migration or a failed check (SY-8). It
// returns a connection ready for store.StartWriter.
func prepareStore(ctx context.Context, dbPath string, forced bool, out io.Writer) (*sql.DB, error) {
	db, err := store.OpenWriteConn(dbPath)
	if err != nil {
		return nil, err
	}

	before, err := store.CurrentSchemaVersion(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.Migrate(db); err != nil {
		_, _ = fmt.Fprintf(out, "migration failed, rebuilding from local data: %v\n", err)
		_ = db.Close()
		return recoverStore(ctx, dbPath, out)
	}
	after, err := store.CurrentSchemaVersion(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	if !store.NeedsIntegrityCheck(dbPath, after != before) && !forced {
		return db, nil
	}
	if err := store.CheckIntegrity(ctx, db, integrityProgress(out)); err != nil {
		_, _ = fmt.Fprintf(out, "integrity check failed, rebuilding from local data: %v\n", err)
		_ = db.Close()
		return recoverStore(ctx, dbPath, out)
	}
	return db, nil
}

// recoverStore rebuilds the store at dbPath from its non-rebuildable
// data (store.Recover) and returns a fresh, migrated connection over
// the result.
func recoverStore(ctx context.Context, dbPath string, out io.Writer) (*sql.DB, error) {
	counts, err := store.Recover(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	_, _ = fmt.Fprintf(out, "rebuilt store: %d outbox intent(s) and %d local message(s) preserved\n", counts.Outbox, counts.Messages)

	db, err := store.OpenWriteConn(dbPath)
	if err != nil {
		return nil, err
	}
	if err := store.Migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// integrityProgress returns a store.CheckIntegrity progress callback
// that prints each stage to out, the visible progress state a
// multi-second quick_check owes the operator (QA-1).
func integrityProgress(out io.Writer) func(string) {
	return func(stage string) {
		_, _ = fmt.Fprintln(out, stage+"...")
	}
}
