package cache

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/contacts"
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

func TestQueueContactPut_ReconcilesEmails(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()

	mustExec(t, a.db, `INSERT INTO addressbooks(href, display_name, description, sync_token, ctag, supports_sync, last_synced_at) VALUES ('/b/', 'Default', '', '', '', 1, 0)`)
	mustExec(t, a.db,
		`INSERT INTO contacts(uid, addressbook_href, href, etag, vcard, rev, fn, family, given, org, title, note, last_synced_at)
		 VALUES ('u1', '/b/', '/b/u1.vcf', '"e1"', ?, '', 'Ada', 'Lovelace', 'Ada', '', '', '', 0)`,
		[]byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u1\r\nFN:Ada\r\nN:Lovelace;Ada;;;\r\nEMAIL;PREF=1:old@x.example\r\nEND:VCARD\r\n"))
	mustExec(t, a.db, `INSERT INTO contact_emails(contact_uid, address, label, pref) VALUES ('u1','old@x.example','',1)`)

	// Seed a message and recipient row so we can verify the projection
	// is not disturbed by QueueContactPut.
	id := mustInsertMessage(t, a.db, "m10", 100, "new@x.example", "", "")
	mustExec(t, a.db, `INSERT INTO message_recipients(message_uid, role, address, name, sent_at) VALUES (?, 'from', 'new@x.example', 'Ada', 100)`, id)
	mustExec(t, a.db, `INSERT INTO message_recipients(message_uid, role, address, name, sent_at) VALUES (?, 'from', 'old@x.example', 'Ada', 100)`, id)

	c := contacts.Contact{
		Kind:   contacts.KindPerson,
		Given:  "Ada",
		Family: "Lovelace",
		Name:   "Ada Lovelace",
		Emails: []contacts.Email{{Address: "new@x.example"}},
	}
	body := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u1\r\nEMAIL;PREF=1:new@x.example\r\nEND:VCARD\r\n")
	if err := a.QueueContactPut(ctx, "/b/", "u1", "/b/u1.vcf", `"e1"`, c, body); err != nil {
		t.Fatal(err)
	}

	// Outbox row exists with the right kind.
	var rowKind string
	if err := a.db.QueryRow(`SELECT kind FROM outbox WHERE kind = ?`, string(KindContactPut)).Scan(&rowKind); err != nil {
		t.Fatal(err)
	}
	if rowKind != string(KindContactPut) {
		t.Errorf("kind=%q want contact-put", rowKind)
	}

	// contact_emails reconciled to the new address only.
	rows, err := a.db.Query(`SELECT address FROM contact_emails WHERE contact_uid='u1'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var addrs []string
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		addrs = append(addrs, s)
	}
	if len(addrs) != 1 || addrs[0] != "new@x.example" {
		t.Errorf("contact_emails=%v want [new@x.example]", addrs)
	}

	// message_recipients untouched. reconcileRecipientsTx is a no-op.
	var nrec int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM message_recipients WHERE message_uid=?`, id).Scan(&nrec); err != nil {
		t.Fatal(err)
	}
	if nrec != 2 {
		t.Errorf("message_recipients rows=%d want 2", nrec)
	}
}

func TestQueueContactDelete_RemovesRowAndRecipientPivots(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()

	mustExec(t, a.db, `INSERT INTO addressbooks(href, display_name, description, sync_token, ctag, supports_sync, last_synced_at) VALUES ('/b/', 'Default', '', '', '', 1, 0)`)
	mustExec(t, a.db,
		`INSERT INTO contacts(uid, addressbook_href, href, etag, vcard, rev, fn, family, given, org, title, note, last_synced_at)
		 VALUES ('u1', '/b/', '/b/u1.vcf', '"e1"', x'', '', 'Ada', '', 'Ada', '', '', '', 0)`)
	mustExec(t, a.db, `INSERT INTO contact_emails(contact_uid, address, label, pref) VALUES ('u1','a@x.example','',1)`)

	if err := a.QueueContactDelete(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM contacts WHERE uid='u1'`).Scan(&n)
	if n != 0 {
		t.Errorf("contacts row not deleted")
	}
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM contact_emails WHERE contact_uid='u1'`).Scan(&n)
	if n != 0 {
		t.Errorf("contact_emails not cascaded")
	}
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE kind=?`, string(KindContactDelete)).Scan(&n)
	if n != 1 {
		t.Errorf("outbox row missing for delete")
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
