// Package storetest is internal/store's half of ADR-0014's
// second-implementation pattern for engine tests
// (internal/backend/backendtest is the backend seam's half): a real,
// migrated store any engine package can stand up in a test without
// re-deriving the pragma set and migration sequence internal/store
// itself keeps private.
package storetest

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/glw907/poplar/internal/store"
)

// dsn mirrors the pragma set internal/store's own DSN builder keeps
// unexported: foreign keys on, WAL journaling, a busy timeout. A test
// writer built here enforces the same cascade and locking behavior
// production connections do.
func dsn(path string) string {
	return fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(wal)&_pragma=synchronous(normal)", path)
}

// OpenWriter opens a fresh migrated store file under t.TempDir and
// returns a Writer over it running cfg's timing, closing it on
// cleanup.
func OpenWriter(t *testing.T, cfg store.WriterConfig) *store.Writer {
	t.Helper()

	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", dsn(path))
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
