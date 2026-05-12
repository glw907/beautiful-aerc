package cache

import (
	"path/filepath"
	"testing"
)

// TestMigrateV13DropsPreExistingOrphans seeds an orphan recipient row
// at v12 (where the column has no FK), then migrates to v13 and asserts
// the JOIN scrub drops the orphan.
func TestMigrateV13DropsPreExistingOrphans(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := openAtVersion(dbPath, 12)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id := mustInsertMessage(t, db, "m1", 100, "from@x", "to@x", "")
	mustExec(t, db, `INSERT INTO message_recipients(message_uid, role, address, sent_at) VALUES (?, 'to', 'live@x', 100)`, id)
	mustExec(t, db, `INSERT INTO message_recipients(message_uid, role, address, sent_at) VALUES (?, 'to', 'ghost@x', 100)`, 9999)

	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply v13: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM message_recipients WHERE address = 'ghost@x'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("orphan recipient survived migration: %d rows", n)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM message_recipients WHERE address = 'live@x'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("live recipient lost: %d rows", n)
	}
}

// TestMigrateV13CascadesOnDelete asserts the v13 FK actually cascades
// at runtime: deleting a message removes its recipient rows.
func TestMigrateV13CascadesOnDelete(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := openWithMigrations(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id := mustInsertMessage(t, db, "m1", 100, "from@x", "to@x", "")
	mustExec(t, db, `INSERT INTO message_recipients(message_uid, role, address, sent_at) VALUES (?, 'to', 'alice@x', 100)`, id)

	mustExec(t, db, `DELETE FROM messages WHERE id = ?`, id)

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM message_recipients WHERE message_uid = ?`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("recipient rows survived parent delete: %d", n)
	}
}

// TestMigrateV13ScrubsOrphanFTSRows covers the F2 piggyback: orphan
// messages_fts rows left behind by historical deletes are dropped.
func TestMigrateV13ScrubsOrphanFTSRows(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := openAtVersion(dbPath, 12)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id := mustInsertMessage(t, db, "m1", 100, "from@x", "to@x", "")
	mustExec(t, db, `INSERT INTO messages_fts(rowid, subject, from_addr, to_addr, cc_addr, body) VALUES (?, 'live', '', '', '', '')`, id)
	mustExec(t, db, `INSERT INTO messages_fts(rowid, subject, from_addr, to_addr, cc_addr, body) VALUES (?, 'ghost', '', '', '', '')`, 9999)

	if err := applyMigrations(db); err != nil {
		t.Fatalf("apply v13: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM messages_fts WHERE rowid = 9999`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("orphan fts row survived migration: %d", n)
	}
}
