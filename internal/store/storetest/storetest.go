// Package storetest is internal/store's half of ADR-0014's
// second-implementation pattern for engine tests
// (internal/backend/backendtest is the backend seam's half): a real,
// migrated store any engine package can stand up in a test without
// re-deriving the pragma set and migration sequence internal/store
// itself keeps private.
package storetest

import (
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
