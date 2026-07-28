package sync

import (
	"context"
	"database/sql"
	"slices"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// TestApplyMessageReconcilesMailboxes asserts upsertMessage's
// mailbox_ids handling: a create lands the association and the
// seen-derived unread flag, and a later update that moves the
// message to a different mailbox drops the old association and
// inserts the new one rather than accumulating both.
func TestApplyMessageReconcilesMailboxes(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	inboxID := seedMailbox(t, w, accountID, "mb-inbox", "Inbox")
	archiveID := seedMailbox(t, w, accountID, "mb-archive", "Archive")

	rec := backend.Record{ID: "m1", Fields: map[string]any{
		"subject":     "hello",
		"received_at": time.Unix(1000, 0),
		"seen":        false,
		"mailbox_ids": []string{"mb-inbox"},
	}}

	if err := w.Apply(context.Background(), func(tx *sql.Tx) error {
		return upsertMessage(tx, accountID, rec)
	}); err != nil {
		t.Fatalf("upsertMessage: %v", err)
	}

	if got := mailboxIDsForServerID(t, w, accountID); !slices.Equal(got, []int64{inboxID}) {
		t.Fatalf("mailboxes = %v, want [%d] (Inbox)", got, inboxID)
	}
	if !unreadForServerID(t, w, accountID, "m1", inboxID) {
		t.Fatal("unread = false, want true: seen was false")
	}

	rec.Fields["mailbox_ids"] = []string{"mb-archive"}
	rec.Fields["seen"] = true
	if err := w.Apply(context.Background(), func(tx *sql.Tx) error {
		return upsertMessage(tx, accountID, rec)
	}); err != nil {
		t.Fatalf("upsertMessage (move): %v", err)
	}

	if got := mailboxIDsForServerID(t, w, accountID); !slices.Equal(got, []int64{archiveID}) {
		t.Fatalf("mailboxes after move = %v, want [%d] (Archive)", got, archiveID)
	}
	if unreadForServerID(t, w, accountID, "m1", archiveID) {
		t.Fatal("unread = true, want false: seen is now true")
	}
}

// TestFlagsFromFields asserts each of Record.Fields' five boolean
// keywords sets its own store.Flags bit, an absent field leaves its
// bit clear, and every bit combines rather than the last one winning.
func TestFlagsFromFields(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string]any
		want   store.Flags
	}{
		{"none set", map[string]any{}, 0},
		{"seen", map[string]any{"seen": true}, store.FlagSeen},
		{"flagged", map[string]any{"flagged": true}, store.FlagFlagged},
		{"answered", map[string]any{"answered": true}, store.FlagAnswered},
		{"draft", map[string]any{"draft": true}, store.FlagDraft},
		{"forwarded", map[string]any{"forwarded": true}, store.FlagForwarded},
		{
			"all five combine",
			map[string]any{"seen": true, "flagged": true, "answered": true, "draft": true, "forwarded": true},
			store.FlagSeen | store.FlagFlagged | store.FlagAnswered | store.FlagDraft | store.FlagForwarded,
		},
		{"false value clears", map[string]any{"seen": false, "flagged": true}, store.FlagFlagged},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flagsFromFields(tt.fields); got != tt.want {
				t.Errorf("flagsFromFields(%v) = %v, want %v", tt.fields, got, tt.want)
			}
		})
	}
}

// TestFirstAddress asserts firstAddress's rendering: a named address
// as "Name <email>", a bare address as just the email, the first
// entry of a multi-address list, and every non-matching input shape
// as the empty string.
func TestFirstAddress(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want string
	}{
		{"nil", nil, ""},
		{"wrong type", "not an address list", ""},
		{"empty list", []map[string]string{}, ""},
		{"named", []map[string]string{{"name": "Ada Lovelace", "email": "ada@example.com"}}, "Ada Lovelace <ada@example.com>"},
		{"bare, empty name", []map[string]string{{"name": "", "email": "ada@example.com"}}, "ada@example.com"},
		{
			"multiple entries take the first",
			[]map[string]string{
				{"name": "First", "email": "a@example.com"},
				{"name": "Second", "email": "b@example.com"},
			},
			"First <a@example.com>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstAddress(tt.v); got != tt.want {
				t.Errorf("firstAddress(%v) = %q, want %q", tt.v, got, tt.want)
			}
		})
	}
}

// TestUpsertMailboxRepairsOrphanedAssociations asserts
// repairMailboxAssociations: a message synced before its mailbox
// existed keeps its mailbox_ids in message.data unresolved, and once
// that mailbox's row is later created, upsertMailbox re-associates
// the message without needing another message-side update.
func TestUpsertMailboxRepairsOrphanedAssociations(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	rec := backend.Record{ID: "m1", Fields: map[string]any{
		"subject":     "hello",
		"received_at": time.Unix(1000, 0),
		"seen":        false,
		"mailbox_ids": []string{"mb-inbox"},
	}}
	if err := w.Apply(context.Background(), func(tx *sql.Tx) error {
		return upsertMessage(tx, accountID, rec)
	}); err != nil {
		t.Fatalf("upsertMessage: %v", err)
	}

	if got := mailboxIDsForServerID(t, w, accountID); len(got) != 0 {
		t.Fatalf("mailboxes before the mailbox row exists = %v, want none", got)
	}

	mailboxRec := backend.Record{ID: "mb-inbox", Fields: map[string]any{"name": "Inbox"}}
	if err := w.Apply(context.Background(), func(tx *sql.Tx) error {
		return upsertMailbox(tx, accountID, mailboxRec)
	}); err != nil {
		t.Fatalf("upsertMailbox: %v", err)
	}

	inboxID := seedMailboxID(t, w, accountID, "mb-inbox")
	got := mailboxIDsForServerID(t, w, accountID)
	if !slices.Equal(got, []int64{inboxID}) {
		t.Fatalf("mailboxes after the mailbox is created = %v, want [%d] (Inbox), repaired without another message update", got, inboxID)
	}
	if !unreadForServerID(t, w, accountID, "m1", inboxID) {
		t.Fatal("unread = false, want true: seen was false")
	}
}

// seedMailboxID returns serverID's internal mailbox id within
// accountID.
func seedMailboxID(t *testing.T, w *store.Writer, accountID int64, serverID string) int64 {
	t.Helper()

	var id int64
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT id FROM mailbox WHERE account_id = ? AND server_id = ?`, accountID, serverID).Scan(&id)
	})
	if err != nil {
		t.Fatalf("mailbox id for %q: %v", serverID, err)
	}
	return id
}
