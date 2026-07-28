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
