package store

import (
	"database/sql"
	"testing"
)

// TestMessageFTSSurvivesCascadeDelete proves message_fts stays
// consistent with message across a cascading delete, not only a
// direct one. Deleting an account cascades through mailbox to
// message inside SQLite itself, below the store-internal helper
// that otherwise owns FTS maintenance; without
// trg_message_fts_delete, that cascade orphans the deleted row's
// terms in the index. An orphaned term still matches a search and,
// once a caller reads message_fts's content columns for it, raises
// SQLite's own "missing row from content table" corruption error,
// which is the concrete failure this trigger prevents.
func TestMessageFTSSurvivesCascadeDelete(t *testing.T) {
	db := openMigratedTestDB(t)
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign_keys: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO account (id, slug, backend_kind, address) VALUES (1, 'a', 'jmap', 'a@example.com')`); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO mailbox (id, account_id, name) VALUES (1, 1, 'Inbox')`); err != nil {
		t.Fatalf("insert mailbox: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO message (id, account_id, received_at, subject, search_text) VALUES (1, 1, 0, 'hello', 'world')`); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO message_fts(rowid, subject, search_text) VALUES (1, 'hello', 'world')`); err != nil {
		t.Fatalf("index message: %v", err)
	}
	if got := matchingRowCount(t, db, "hello"); got != 1 {
		t.Fatalf("matching rows before delete = %d, want 1", got)
	}

	if _, err := db.Exec(`DELETE FROM account WHERE id = 1`); err != nil {
		t.Fatalf("delete account: %v", err)
	}

	var messageCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM message`).Scan(&messageCount); err != nil {
		t.Fatalf("count message: %v", err)
	}
	if messageCount != 0 {
		t.Fatalf("message row survived the account cascade, want 0, got %d", messageCount)
	}

	if got := matchingRowCount(t, db, "hello"); got != 0 {
		t.Fatalf("matching rows after cascade delete = %d, want 0: message_fts still holds the deleted row's term", got)
	}
}

// matchingRowCount returns how many message_fts rows match term.
func matchingRowCount(t *testing.T, db *sql.DB, term string) int {
	t.Helper()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM message_fts WHERE message_fts MATCH ?`, term).Scan(&count); err != nil {
		t.Fatalf("count matches for %q: %v", term, err)
	}
	return count
}
