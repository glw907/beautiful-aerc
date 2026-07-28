package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestCheckpointLifecycle proves the writer's own checkpoint
// schedule (ADR-0003 revision 2): PASSIVE after each bulk chunk
// keeps the WAL synced but does not shrink its file while a reader
// holds an old snapshot, and TRUNCATE at idle shrinks it back down
// once that snapshot is released.
func TestCheckpointLifecycle(t *testing.T) {
	cfg := DefaultWriterConfig()
	cfg.CheckpointIdle = 30 * time.Millisecond
	w, path := newTestWriter(t, cfg)

	reader, err := sql.Open("sqlite", dsn(path, connReadOnly))
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	rtx, err := reader.Begin()
	if err != nil {
		t.Fatalf("begin reader tx: %v", err)
	}
	var accounts int
	if err := rtx.QueryRow(`SELECT COUNT(*) FROM account`).Scan(&accounts); err != nil {
		t.Fatalf("hold reader snapshot: %v", err)
	}

	big := strings.Repeat("x", 4096)
	for i := range 200 {
		err := w.SubmitBulk(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.Exec(
				`INSERT INTO account (slug, backend_kind, address, data) VALUES (?, ?, ?, ?)`,
				fmt.Sprintf("acct-%d", i), "jmap", "user@example.com", big)
			return err
		})
		if err != nil {
			t.Fatalf("SubmitBulk(%d): %v", i, err)
		}
	}

	walPath := path + "-wal"
	grown := walSize(t, walPath)
	if grown < 500_000 {
		t.Fatalf("wal size = %d bytes, want it grown past PASSIVE's reach while the reader held a snapshot", grown)
	}

	if err := rtx.Rollback(); err != nil {
		t.Fatalf("release reader snapshot: %v", err)
	}

	// Give the writer's idle timer room to fire a TRUNCATE checkpoint
	// now that nothing holds an old snapshot open.
	time.Sleep(4 * cfg.CheckpointIdle)

	shrunk := walSize(t, walPath)
	if shrunk > 4096 {
		t.Errorf("wal size after idle = %d bytes, want it back near its bound", shrunk)
	}
}

func walSize(t *testing.T, path string) int64 {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Size()
}
