package store

import (
	"context"
	"database/sql"
	"fmt"
)

// checkpointConfig governs the writer-owned checkpoint policy
// (ADR-0003 revision 2).
type checkpointConfig struct {
	// JournalSizeLimit bounds the WAL file in bytes, independent of
	// the PASSIVE/TRUNCATE schedule the writer runs itself.
	JournalSizeLimit int64
}

// configureCheckpointing turns off db's automatic WAL checkpoint and
// applies journal_size_limit, so every checkpoint against db from
// here on is one the writer runs itself between or after a batch,
// never one a commit triggers mid-transaction.
func configureCheckpointing(ctx context.Context, db *sql.DB, cfg checkpointConfig) error {
	if _, err := db.ExecContext(ctx, "PRAGMA wal_autocheckpoint = 0"); err != nil {
		return fmt.Errorf("disable wal_autocheckpoint: %w", err)
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA journal_size_limit = %d", cfg.JournalSizeLimit)); err != nil {
		return fmt.Errorf("set journal_size_limit: %w", err)
	}
	return nil
}

// checkpoint runs a wal_checkpoint pragma in the given mode
// (PASSIVE or TRUNCATE) against db.
func checkpoint(ctx context.Context, db *sql.DB, mode string) error {
	_, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint("+mode+")")
	return err
}

// incrementalVacuumPages bounds each idle-triggered incremental_vacuum
// step (item 3 of the pass-1 audit): with auto_vacuum=INCREMENTAL, a
// deleted row's pages sit on the freelist until reclaimed, and an
// unbounded incremental_vacuum() call would walk the whole freelist
// in one shot, the same unbounded stall a bare VACUUM causes.
// Reclaiming a bounded slice per idle window keeps every call fast,
// at the cost of needing more than one idle window to shrink a file
// after a large delete.
const incrementalVacuumPages = 500

// incrementalVacuum runs a bounded incremental_vacuum pragma against
// db, reclaiming up to pages freelist pages back to the OS.
func incrementalVacuum(ctx context.Context, db *sql.DB, pages int) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA incremental_vacuum(%d)", pages))
	return err
}

// checkpointBusyTimeoutMS is the busy_timeout checkpointTruncate sets
// for the duration of a TRUNCATE checkpoint, restored to
// busyTimeoutMS immediately after. TRUNCATE is the one checkpoint
// mode that can block on a reader's open snapshot; capping that wait
// at the writer's own admission ceiling keeps an interactive job
// queued behind it from waiting anywhere near busyTimeoutMS's 5s
// bound.
const checkpointBusyTimeoutMS = 50

// checkpointTruncate runs a TRUNCATE checkpoint against db under
// checkpointBusyTimeoutMS rather than db's normal busy_timeout,
// restoring the normal timeout before returning whether or not the
// checkpoint succeeded.
func checkpointTruncate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout = %d", checkpointBusyTimeoutMS)); err != nil {
		return fmt.Errorf("lower busy_timeout for checkpoint: %w", err)
	}
	defer func() {
		_, _ = db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeoutMS))
	}()
	return checkpoint(ctx, db, "TRUNCATE")
}
