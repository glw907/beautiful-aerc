package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// The fixture rows every test in this package hangs its own rows off:
// one account and one mailbox, both at id 1 so a fixture can name
// account_id or mailbox_id 1 literally.
const (
	seedAccountSQL = `INSERT INTO account (id, slug, backend_kind, address) VALUES (1, 'a', 'jmap', 'a@example.com')`
	seedMailboxSQL = `INSERT INTO mailbox (id, account_id, name) VALUES (1, 1, 'Inbox')`
)

// seedAccount inserts the fixture account directly on db, for a test
// holding a connection rather than a Writer.
func seedAccount(t *testing.T, db *sql.DB) {
	t.Helper()

	if _, err := db.Exec(seedAccountSQL); err != nil {
		t.Fatalf("seed account: %v", err)
	}
}

// seedAccountAndMailbox inserts both fixture rows through w, in one
// committed transaction, the fixture every read test in this package
// builds a mailbox on.
func seedAccountAndMailbox(t *testing.T, w *Writer) {
	t.Helper()

	err := w.submit(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(seedAccountSQL); err != nil {
			return err
		}
		_, err := tx.Exec(seedMailboxSQL)
		return err
	})
	if err != nil {
		t.Fatalf("seed account and mailbox: %v", err)
	}
}

// openTestDB opens a fresh sqlite file under t.TempDir through dsn,
// the pragma set every store connection carries, and returns the
// unmigrated connection; the caller decides when to call Migrate.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", dsn(path, connReadWrite))
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
