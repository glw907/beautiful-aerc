package store

import "testing"

// TestOutboxStateDefault proves a freshly inserted intent starts
// 'queued', ADR-0006 revision 2's vocabulary, rather than the retired
// 'pending' default.
func TestOutboxStateDefault(t *testing.T) {
	db := openMigratedTestDB(t)
	seedTestAccount(t, db)

	res, err := db.Exec(`INSERT INTO outbox (account_id, kind, payload, created_at) VALUES (1, 'send', '{}', 1000)`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}

	var state string
	if err := db.QueryRow(`SELECT state FROM outbox WHERE id = ?`, id).Scan(&state); err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state != "queued" {
		t.Errorf("state = %q, want %q", state, "queued")
	}
}

// TestOutboxStateRejectsUnknownValue proves the CHECK constraint
// refuses any state outside ADR-0006 revision 2's closed vocabulary.
func TestOutboxStateRejectsUnknownValue(t *testing.T) {
	db := openMigratedTestDB(t)
	seedTestAccount(t, db)

	_, err := db.Exec(`INSERT INTO outbox (account_id, kind, payload, state, created_at) VALUES (1, 'send', '{}', 'pending', 1000)`)
	if err == nil {
		t.Fatal("insert with state = 'pending' succeeded, want the CHECK constraint to refuse it")
	}
}
