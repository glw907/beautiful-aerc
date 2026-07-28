package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// openTestDB opens a fresh sqlite file under t.TempDir and returns
// the unmigrated connection; the caller decides when to call
// Migrate.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// openMigratedTestDB opens a fresh sqlite file and migrates it to
// this build's known schema version.
func openMigratedTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db := openTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}
