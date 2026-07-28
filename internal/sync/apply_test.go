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

	if got := mailboxIDsForServerID(t, w, accountID, "m1"); !slices.Equal(got, []int64{inboxID}) {
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

	if got := mailboxIDsForServerID(t, w, accountID, "m1"); !slices.Equal(got, []int64{archiveID}) {
		t.Fatalf("mailboxes after move = %v, want [%d] (Archive)", got, archiveID)
	}
	if unreadForServerID(t, w, accountID, "m1", archiveID) {
		t.Fatal("unread = true, want false: seen is now true")
	}
}
