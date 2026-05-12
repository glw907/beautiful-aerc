package cache

import (
	"path/filepath"
	"testing"
)

// TestMigrateV11BackfillsCachedBodies seeds a body row at v10 (before
// FTS exists), runs the v11 migration, and asserts the body column is
// populated so a body-only MATCH finds the row.
func TestMigrateV11BackfillsCachedBodies(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := openAtVersion(dbPath, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id := mustInsertMessage(t, db, "m1", 100, "from@x", "to@x", "")
	body := []byte("Subject: Hi\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"crocodile sandwich\r\n")
	mustExec(t, db, `INSERT INTO bodies(message, bytes, fetched_at) VALUES (?, ?, 100)`, id, body)

	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply v11+: %v", err)
	}

	var rowid int64
	err = db.QueryRow(`SELECT rowid FROM messages_fts WHERE messages_fts MATCH 'body:crocodile'`).Scan(&rowid)
	if err != nil {
		t.Fatalf("body MATCH after backfill: %v", err)
	}
	if rowid != id {
		t.Errorf("MATCH body:crocodile rowid = %d, want %d", rowid, id)
	}
}
