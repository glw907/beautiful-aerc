package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
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
//
// SQLite reports a checkpoint it could not finish in the pragma's own
// result row, never as an error: busy is 1 and the checkpointed frame
// count falls short of the log's. A caller reading only the error sees
// a clean run while the WAL grows past its bound, so checkpoint reads
// the row and logs the refusal instead.
func checkpoint(ctx context.Context, db *sql.DB, mode string) error {
	var busy, logFrames, checkpointed int
	if err := db.QueryRowContext(ctx, "PRAGMA wal_checkpoint("+mode+")").Scan(&busy, &logFrames, &checkpointed); err != nil {
		return err
	}
	if busy != 0 {
		slog.Warn("store: checkpoint could not drain the write-ahead log",
			"mode", mode, "log_frames", logFrames, "checkpointed_frames", checkpointed)
	}
	return nil
}

// incrementalVacuumPages bounds each idle-triggered incremental_vacuum
// step: with auto_vacuum=INCREMENTAL, a deleted row's pages sit on
// the freelist until reclaimed, and an unbounded incremental_vacuum()
// call would walk the whole freelist in one shot, the same unbounded
// stall a bare VACUUM causes. Reclaiming a bounded slice per idle
// window keeps every call fast, at the cost of needing more than one
// idle window to shrink a file after a large delete.
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
// checkpoint succeeded. A failed restore joins the returned error:
// the writer's pool holds one physical connection, so a connection
// left at the checkpoint's short timeout stays there for the life of
// the process, and every transaction after it gives up on
// SQLITE_BUSY in a hundredth of the time it should.
func checkpointTruncate(ctx context.Context, db *sql.DB) (err error) {
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout = %d", checkpointBusyTimeoutMS)); err != nil {
		return fmt.Errorf("lower busy_timeout for checkpoint: %w", err)
	}
	defer func() {
		if _, restoreErr := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeoutMS)); restoreErr != nil {
			slog.Error("store: the write connection is stuck at the checkpoint's short busy_timeout, where later transactions give up on a lock early",
				"timeout_ms", checkpointBusyTimeoutMS, "error", restoreErr)
			err = errors.Join(err, fmt.Errorf("restore busy_timeout: %w", restoreErr))
		}
	}()
	return checkpoint(ctx, db, "TRUNCATE")
}
