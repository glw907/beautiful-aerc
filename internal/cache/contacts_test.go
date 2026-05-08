package cache

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// openWithMigrations opens (or creates) a DB at path and runs all migrations.
func openWithMigrations(path string) (*sql.DB, error) {
	db, err := OpenDB(path)
	if err != nil {
		return nil, err
	}
	if err := applyMigrations(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// openAtVersion opens (or creates) a DB at path and runs migrations
// up to schema version n.
func openAtVersion(path string, n int) (*sql.DB, error) {
	db, err := OpenDB(path)
	if err != nil {
		return nil, err
	}
	if err := applyMigrationsTo(db, n); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func TestMigrateV8_TablesExist(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := openWithMigrations(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	want := []string{
		"addressbooks", "contacts", "contact_emails",
		"contact_phones", "message_recipients",
	}
	for _, name := range want {
		var got string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`,
			name,
		).Scan(&got)
		if err == sql.ErrNoRows {
			t.Errorf("table %s missing after migrateV8", name)
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestMigrateV8_BackfillRecipients(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Open at v7 to seed a messages row, then trigger v8 backfill on re-open.
	// messages.protocol_id is NOT NULL; id is AUTOINCREMENT.
	db, err := openAtVersion(dbPath, 7)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO messages(protocol_id, sent_at, from_addr, to_addr, cc_addr) VALUES (?, ?, ?, ?, ?)`,
		"msg-1", int64(1700000000),
		`Alice <alice@example.com>`,
		`Bob <bob@example.com>, carol@example.com`,
		``,
	); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	// Re-open triggers v7→v8 migration with backfill.
	db, err = openWithMigrations(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM message_recipients`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	// Expect 3 rows: 1 from (alice), 2 to (bob + carol).
	if n != 3 {
		t.Errorf("backfilled rows = %d; want 3 (1 from, 2 to)", n)
	}
}
