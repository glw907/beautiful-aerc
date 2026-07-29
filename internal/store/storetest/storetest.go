// Package storetest is internal/store's half of ADR-0014's
// second-implementation pattern for engine tests
// (internal/backend/backendtest is the backend seam's half): a real,
// migrated store any engine package can stand up in a test without
// re-deriving the pragma set and migration sequence internal/store
// itself keeps private.
package storetest

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/glw907/poplar/internal/store"
)

// OpenWriter opens a fresh migrated store file under t.TempDir and
// returns a Writer over it running cfg's timing, closing it on
// cleanup. The connection carries store.OpenWriteConn's pragma set,
// the one place poplar spells it, so a test writer never drifts from
// production's cache budget and locking behavior.
func OpenWriter(t *testing.T, cfg store.WriterConfig) *store.Writer {
	t.Helper()

	path := filepath.Join(t.TempDir(), "store.db")
	db, err := store.OpenWriteConn(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	w, err := store.NewWriter(db, cfg)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w
}

// Insert runs stmt on w's interactive lane and returns the id of the
// row it inserted. An engine test seeds its fixture rows with raw SQL
// rather than the engine under test, so a fixture never depends on the
// code path it is there to exercise.
func Insert(t *testing.T, w *store.Writer, stmt string, args ...any) int64 {
	t.Helper()

	var id int64
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		res, err := tx.Exec(stmt, args...)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		t.Fatalf("insert (%s): %v", stmt, err)
	}
	return id
}

// ScanValue runs a single-row, single-column query on w's interactive
// lane and returns the value it read. A query matching no row fails
// the test.
func ScanValue[T any](t *testing.T, w *store.Writer, query string, args ...any) T {
	t.Helper()

	var v T
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(query, args...).Scan(&v)
	})
	if err != nil {
		t.Fatalf("query (%s): %v", query, err)
	}
	return v
}
