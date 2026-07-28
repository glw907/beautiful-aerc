package store

import (
	"database/sql"
	"slices"
	"testing"
)

// TestServerIDLookupUsesIndex proves the (account_id, server_id)
// upsert lookup UpsertMessage and UpsertMailbox run on every sync
// page (item 1 of the pass-1 audit) has an index to use, rather than
// scanning the whole table.
func TestServerIDLookupUsesIndex(t *testing.T) {
	db := openMigratedTestDB(t)

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			"message upsert lookup",
			`SELECT id FROM message WHERE account_id = ? AND server_id = ?`,
			[]string{"SEARCH message USING COVERING INDEX idx_message_account_server (account_id=? AND server_id=?)"},
		},
		{
			"mailbox upsert lookup",
			`SELECT id FROM mailbox WHERE account_id = ? AND server_id = ?`,
			[]string{"SEARCH mailbox USING COVERING INDEX idx_mailbox_account_server (account_id=? AND server_id=?)"},
		},
		{
			"contact_card upsert lookup",
			`SELECT id FROM contact_card WHERE account_id = ? AND server_id = ?`,
			[]string{"SEARCH contact_card USING COVERING INDEX idx_contact_card_account_server (account_id=? AND server_id=?)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := explainPlan(t, db, tt.query, 1, "srv-1")
			if !slices.Equal(got, tt.want) {
				t.Errorf("plan = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestServerIDUniqueAcrossRows proves the new indexes refuse two rows
// sharing an account and a server id, and that the partial WHERE
// clause still allows more than one origin = 'local' row (a NULL
// server_id) to coexist.
func TestServerIDUniqueAcrossRows(t *testing.T) {
	db := openMigratedTestDB(t)
	seedTestAccount(t, db)

	tests := []struct {
		name       string
		firstStmt  string
		secondStmt string
	}{
		{
			"message",
			`INSERT INTO message (account_id, server_id, received_at) VALUES (1, 'srv-1', 1000)`,
			`INSERT INTO message (account_id, server_id, received_at) VALUES (1, 'srv-1', 2000)`,
		},
		{
			"mailbox",
			`INSERT INTO mailbox (account_id, server_id, name) VALUES (1, 'srv-1', 'Inbox')`,
			`INSERT INTO mailbox (account_id, server_id, name) VALUES (1, 'srv-1', 'Archive')`,
		},
		{
			"contact_card",
			`INSERT INTO contact_card (account_id, server_id) VALUES (1, 'srv-1')`,
			`INSERT INTO contact_card (account_id, server_id) VALUES (1, 'srv-1')`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := db.Exec(tt.firstStmt); err != nil {
				t.Fatalf("first insert: %v", err)
			}
			if _, err := db.Exec(tt.secondStmt); err == nil {
				t.Fatal("second insert with the same (account_id, server_id) succeeded, want a UNIQUE constraint violation")
			}
		})
	}

	// Two origin = 'local' messages, both with a NULL server_id, must
	// coexist: the partial index excludes them entirely.
	if _, err := db.Exec(`INSERT INTO message (account_id, received_at, origin) VALUES (1, 3000, 'local')`); err != nil {
		t.Fatalf("first local draft: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO message (account_id, received_at, origin) VALUES (1, 4000, 'local')`); err != nil {
		t.Fatalf("second local draft (NULL server_id) rejected, want it to coexist with the first: %v", err)
	}
}

// seedTestAccount inserts one account row with id 1.
func seedTestAccount(t *testing.T, db *sql.DB) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO account (slug, backend_kind, address) VALUES ('a', 'jmap', 'user@example.com')`); err != nil {
		t.Fatalf("seed account: %v", err)
	}
}
