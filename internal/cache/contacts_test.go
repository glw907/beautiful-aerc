package cache

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

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

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("mustExec %q: %v", query, err)
	}
}

func mustInsertMessage(t *testing.T, db *sql.DB, protocolID string, sentAt int64, from, to, cc string) int64 {
	t.Helper()
	res, err := db.Exec(
		`INSERT INTO messages(protocol_id, sent_at, from_addr, to_addr, cc_addr) VALUES (?, ?, ?, ?, ?)`,
		protocolID, sentAt, from, to, cc,
	)
	if err != nil {
		t.Fatalf("insert message %q: %v", protocolID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return id
}

func TestSuggestAddresses_RecencyDecay(t *testing.T) {
	acct := openTestAccount(t)

	now := time.Now().Unix()
	d30 := now - 30*86400

	id1 := mustInsertMessage(t, acct.db, "1", d30, "", "alice@x", "")
	id2 := mustInsertMessage(t, acct.db, "2", d30, "", "alice@x", "")
	id3 := mustInsertMessage(t, acct.db, "3", now, "", "bob@x", "")

	mustExec(t, acct.db, `INSERT INTO message_recipients(message_uid, role, address, sent_at) VALUES (?, 'to', 'alice@x', ?)`, id1, d30)
	mustExec(t, acct.db, `INSERT INTO message_recipients(message_uid, role, address, sent_at) VALUES (?, 'to', 'alice@x', ?)`, id2, d30)
	mustExec(t, acct.db, `INSERT INTO message_recipients(message_uid, role, address, sent_at) VALUES (?, 'to', 'bob@x', ?)`, id3, now)

	got, err := acct.SuggestAddresses(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 {
		t.Fatalf("got %d suggestions; want ≥ 2", len(got))
	}
	if got[0].Email != "bob@x" {
		t.Errorf("recency-weighted top = %q; want bob@x", got[0].Email)
	}
}

func TestSuggestAddresses_PrefixFilter(t *testing.T) {
	acct := openTestAccount(t)
	now := time.Now().Unix()

	id := mustInsertMessage(t, acct.db, "1", now, "", "alice@x", "")
	mustExec(t, acct.db, `INSERT INTO message_recipients(message_uid, role, address, sent_at) VALUES (?, 'to', 'alice@x', ?)`, id, now)

	got, _ := acct.SuggestAddresses(context.Background(), "ali")
	if len(got) != 1 || got[0].Email != "alice@x" {
		t.Errorf("prefix=ali → %+v", got)
	}
	got, _ = acct.SuggestAddresses(context.Background(), "zzz")
	if len(got) != 0 {
		t.Errorf("prefix=zzz → %+v", got)
	}
}

func TestLookupContact_HitMiss(t *testing.T) {
	acct := openTestAccount(t)
	mustExec(t, acct.db, `INSERT INTO addressbooks(href, display_name) VALUES ('/b/', 'Default')`)
	mustExec(t, acct.db,
		`INSERT INTO contacts(uid, addressbook_href, href, etag, vcard, fn, last_synced_at) VALUES ('u1', '/b/', '/b/u1', 'e1', '', 'Alice', ?)`,
		time.Now().Unix())
	mustExec(t, acct.db, `INSERT INTO contact_emails(contact_uid, address) VALUES ('u1', 'alice@x')`)

	c, ok := acct.LookupContact(context.Background(), "alice@x")
	if !ok || c.Name != "Alice" {
		t.Errorf("hit: ok=%v c=%+v", ok, c)
	}
	_, ok = acct.LookupContact(context.Background(), "nobody@x")
	if ok {
		t.Error("miss should return ok=false")
	}
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
