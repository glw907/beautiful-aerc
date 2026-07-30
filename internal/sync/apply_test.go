package sync

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/uerr"
)

// TestApplyMessageReconcilesMailboxes asserts upsertMessage's
// MailboxIDs handling: a create lands the association and the
// FlagSeen-derived unread flag, and a later update that moves the
// message to a different mailbox drops the old association and
// inserts the new one rather than accumulating both.
func TestApplyMessageReconcilesMailboxes(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	inboxID := seedMailbox(t, w, accountID, "mb-inbox", "Inbox")
	archiveID := seedMailbox(t, w, accountID, "mb-archive", "Archive")

	f := backend.MessageFields{
		Subject:    "hello",
		ReceivedAt: time.Unix(1000, 0),
		MailboxIDs: []string{"mb-inbox"},
	}

	if err := w.Apply(context.Background(), func(tx *sql.Tx) error {
		return upsertMessage(tx, accountID, "m1", f)
	}); err != nil {
		t.Fatalf("upsertMessage: %v", err)
	}

	if got := mailboxIDsForServerID(t, w, accountID); !slices.Equal(got, []int64{inboxID}) {
		t.Fatalf("mailboxes = %v, want [%d] (Inbox)", got, inboxID)
	}
	if !unreadForMessage(t, w, accountID, inboxID) {
		t.Fatal("unread = false, want true: FlagSeen was clear")
	}

	f.MailboxIDs = []string{"mb-archive"}
	f.Flags = backend.FlagSeen
	if err := w.Apply(context.Background(), func(tx *sql.Tx) error {
		return upsertMessage(tx, accountID, "m1", f)
	}); err != nil {
		t.Fatalf("upsertMessage (move): %v", err)
	}

	if got := mailboxIDsForServerID(t, w, accountID); !slices.Equal(got, []int64{archiveID}) {
		t.Fatalf("mailboxes after move = %v, want [%d] (Archive)", got, archiveID)
	}
	if unreadForMessage(t, w, accountID, archiveID) {
		t.Fatal("unread = true, want false: FlagSeen is now set")
	}
}

// TestUpsertMessageWritesEveryColumn pins which store column each of
// the message vocabulary's fields reaches. Every value here is
// distinct, so a translation that lands the right value in the wrong
// column fails: nothing else in this package's tests would notice
// blob_id and thread_key swapping places.
func TestUpsertMessageWritesEveryColumn(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	inboxID := seedMailbox(t, w, accountID, "mb-inbox", "Inbox")

	f := backend.MessageFields{
		BlobID:        "blob-1",
		ThreadKey:     "thread-1",
		Subject:       "Quarterly numbers",
		From:          []backend.Address{{Name: "Ada Lovelace", Email: "ada@example.com"}},
		MailboxIDs:    []string{"mb-inbox"},
		ReceivedAt:    time.Unix(1700000000, 0),
		Size:          4096,
		HasAttachment: true,
		Flags:         backend.FlagFlagged | backend.FlagAnswered,
	}
	if err := w.Apply(context.Background(), func(tx *sql.Tx) error {
		return upsertMessage(tx, accountID, "m1", f)
	}); err != nil {
		t.Fatalf("upsertMessage: %v", err)
	}

	var got struct {
		serverID      string
		blobID        string
		threadKey     string
		subject       string
		fromAddr      string
		flags         store.Flags
		size          int64
		hasAttachment bool
		receivedAt    int64
	}
	if err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(
			`SELECT server_id, blob_id, thread_key, subject, from_addr, flags, size, has_attachment, received_at
			   FROM message WHERE account_id = ?`, accountID,
		).Scan(&got.serverID, &got.blobID, &got.threadKey, &got.subject, &got.fromAddr,
			&got.flags, &got.size, &got.hasAttachment, &got.receivedAt)
	}); err != nil {
		t.Fatalf("read message row: %v", err)
	}

	for _, c := range []struct {
		column string
		got    any
		want   any
	}{
		{"server_id", got.serverID, "m1"},
		{"blob_id", got.blobID, "blob-1"},
		{"thread_key", got.threadKey, "thread-1"},
		{"subject", got.subject, "Quarterly numbers"},
		{"from_addr", got.fromAddr, "Ada Lovelace <ada@example.com>"},
		{"flags", got.flags, store.FlagFlagged | store.FlagAnswered},
		{"size", got.size, int64(4096)},
		{"has_attachment", got.hasAttachment, true},
		{"received_at", got.receivedAt, int64(1700000000)},
	} {
		if c.got != c.want {
			t.Errorf("message.%s = %v, want %v", c.column, c.got, c.want)
		}
	}
	if ids := mailboxIDsForServerID(t, w, accountID); !slices.Equal(ids, []int64{inboxID}) {
		t.Errorf("message_mailbox = %v, want [%d] (Inbox)", ids, inboxID)
	}
	if !unreadForMessage(t, w, accountID, inboxID) {
		t.Error("message_mailbox.unread = false, want true: FlagSeen was clear")
	}
}

// TestUpsertMessageClearsMembershipWhenMailboxIDsIsEmpty pins whole-
// membership-replacement as intentional: a message's MailboxIDs is
// its whole folder membership (MessageFields's doc comment), so a
// re-upsert that comes back with no mailboxes clears an association a
// prior upsert set rather than leaving it in place. If the reconcile
// ever became a merge instead of a replace, this fails.
func TestUpsertMessageClearsMembershipWhenMailboxIDsIsEmpty(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	seedMailbox(t, w, accountID, "mb-inbox", "Inbox")

	f := backend.MessageFields{
		Subject:    "hello",
		ReceivedAt: time.Unix(1000, 0),
		MailboxIDs: []string{"mb-inbox"},
	}
	if err := w.Apply(context.Background(), func(tx *sql.Tx) error {
		return upsertMessage(tx, accountID, "m1", f)
	}); err != nil {
		t.Fatalf("upsertMessage: %v", err)
	}
	if got := mailboxIDsForServerID(t, w, accountID); len(got) != 1 {
		t.Fatalf("mailboxes before clearing = %v, want one association", got)
	}

	f.MailboxIDs = nil
	if err := w.Apply(context.Background(), func(tx *sql.Tx) error {
		return upsertMessage(tx, accountID, "m1", f)
	}); err != nil {
		t.Fatalf("upsertMessage (clear): %v", err)
	}
	if got := mailboxIDsForServerID(t, w, accountID); len(got) != 0 {
		t.Fatalf("mailboxes after clearing = %v, want none", got)
	}
}

// TestUpsertRecordRejectsMismatchedFields covers a record whose
// payload disagrees with the kind the Changes page it arrived on
// asked for, the sync-side counterpart of jmapsource's
// TestApplyBatchRejectsMismatchedFields. Nothing reaches either
// table: a wrong-table write is worse than a rejected page, since a
// resync's stale-delete pass only reasons about the collection kind
// names. For the two mismatched-kind cases, the cause the writer
// wraps must name both the record and the payload type; this is what
// caught the mismatch error once rendering its page kind as %v
// printed an integer instead of "mailbox" or "message" (the value
// slog.Info logs).
func TestUpsertRecordRejectsMismatchedFields(t *testing.T) {
	tests := []struct {
		name     string
		kind     backend.ObjectKind
		rec      backend.Record
		wantType string
	}{
		{
			name:     "message fields on a mailbox page",
			kind:     backend.ObjectKindMailbox,
			rec:      backend.Record{ID: "m1", Fields: backend.MessageFields{Subject: "hello"}},
			wantType: "backend.MessageFields",
		},
		{
			name:     "mailbox fields on a message page",
			kind:     backend.ObjectKindMessage,
			rec:      backend.Record{ID: "mb-1", Fields: backend.MailboxFields{Name: "Inbox"}},
			wantType: "backend.MailboxFields",
		},
		{
			name: "nil fields",
			kind: backend.ObjectKindMessage,
			rec:  backend.Record{ID: "m1", Fields: nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := storetest.OpenWriter(t, store.DefaultWriterConfig())
			accountID := seedAccount(t, w)

			err := w.Apply(context.Background(), func(tx *sql.Tx) error {
				return upsertRecord(tx, accountID, tt.kind, tt.rec)
			})
			if err == nil {
				t.Fatal("upsertRecord = nil, want an error")
			}
			if got := countRows(t, w, "message", accountID); got != 0 {
				t.Errorf("message rows = %d, want 0", got)
			}
			if got := countRows(t, w, "mailbox", accountID); got != 0 {
				t.Errorf("mailbox rows = %d, want 0", got)
			}
			if tt.wantType == "" {
				return
			}
			var ue uerr.Error
			if !errors.As(err, &ue) {
				t.Fatalf("error %v is not a uerr.Error", err)
			}
			cause := ue.Cause.Error()
			if !strings.Contains(cause, tt.rec.ID) {
				t.Errorf("cause %q does not name the record %q", cause, tt.rec.ID)
			}
			if !strings.Contains(cause, tt.wantType) {
				t.Errorf("cause %q does not name the payload type %q", cause, tt.wantType)
			}
			if !strings.Contains(cause, kindName(tt.kind)) {
				t.Errorf("cause %q does not name the page kind %q", cause, kindName(tt.kind))
			}
		})
	}
}

// TestUpsertMailboxWritesEveryColumn is the mailbox counterpart of
// TestUpsertMessageWritesEveryColumn. The two counts differ so a
// translation that swaps total for unread fails here.
func TestUpsertMailboxWritesEveryColumn(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	f := backend.MailboxFields{
		Role:        "archive",
		Name:        "Old Stuff",
		SortOrder:   7,
		TotalCount:  42,
		UnreadCount: 5,
	}
	if err := w.Apply(context.Background(), func(tx *sql.Tx) error {
		return upsertMailbox(tx, accountID, "mb-1", f)
	}); err != nil {
		t.Fatalf("upsertMailbox: %v", err)
	}

	var got struct {
		serverID    string
		role        string
		name        string
		sortOrder   int64
		totalCount  int64
		unreadCount int64
	}
	if err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(
			`SELECT server_id, role, name, sort_order, total_count, unread_count
			   FROM mailbox WHERE account_id = ?`, accountID,
		).Scan(&got.serverID, &got.role, &got.name, &got.sortOrder, &got.totalCount, &got.unreadCount)
	}); err != nil {
		t.Fatalf("read mailbox row: %v", err)
	}

	for _, c := range []struct {
		column string
		got    any
		want   any
	}{
		{"server_id", got.serverID, "mb-1"},
		{"role", got.role, "archive"},
		{"name", got.name, "Old Stuff"},
		{"sort_order", got.sortOrder, int64(7)},
		{"total_count", got.totalCount, int64(42)},
		{"unread_count", got.unreadCount, int64(5)},
	} {
		if c.got != c.want {
			t.Errorf("mailbox.%s = %v, want %v", c.column, c.got, c.want)
		}
	}
}

// TestStoreFlags asserts each of the seam's five flags sets its own
// store.Flags bit, a flag left out of the set leaves its bit clear,
// and every bit combines rather than the last one winning.
func TestStoreFlags(t *testing.T) {
	tests := []struct {
		name string
		set  backend.MessageFlags
		want store.Flags
	}{
		{"none set", 0, 0},
		{"seen", backend.FlagSeen, store.FlagSeen},
		{"flagged", backend.FlagFlagged, store.FlagFlagged},
		{"answered", backend.FlagAnswered, store.FlagAnswered},
		{"draft", backend.FlagDraft, store.FlagDraft},
		{"forwarded", backend.FlagForwarded, store.FlagForwarded},
		{
			"all five combine",
			backend.FlagSeen | backend.FlagFlagged | backend.FlagAnswered | backend.FlagDraft | backend.FlagForwarded,
			store.FlagSeen | store.FlagFlagged | store.FlagAnswered | store.FlagDraft | store.FlagForwarded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := storeFlags(tt.set); got != tt.want {
				t.Errorf("storeFlags(%v) = %v, want %v", tt.set, got, tt.want)
			}
		})
	}
}

// TestFirstAddress asserts firstAddress's rendering: a named address
// as "Name <email>", a bare address as just the email, the first
// entry of a multi-address list, and an absent or empty list as the
// empty string.
func TestFirstAddress(t *testing.T) {
	tests := []struct {
		name string
		v    []backend.Address
		want string
	}{
		{"nil", nil, ""},
		{"empty list", []backend.Address{}, ""},
		{"named", []backend.Address{{Name: "Ada Lovelace", Email: "ada@example.com"}}, "Ada Lovelace <ada@example.com>"},
		{"bare, empty name", []backend.Address{{Email: "ada@example.com"}}, "ada@example.com"},
		{
			"multiple entries take the first",
			[]backend.Address{
				{Name: "First", Email: "a@example.com"},
				{Name: "Second", Email: "b@example.com"},
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

	f := backend.MessageFields{
		Subject:    "hello",
		ReceivedAt: time.Unix(1000, 0),
		MailboxIDs: []string{"mb-inbox"},
	}
	if err := w.Apply(context.Background(), func(tx *sql.Tx) error {
		return upsertMessage(tx, accountID, "m1", f)
	}); err != nil {
		t.Fatalf("upsertMessage: %v", err)
	}

	if got := mailboxIDsForServerID(t, w, accountID); len(got) != 0 {
		t.Fatalf("mailboxes before the mailbox row exists = %v, want none", got)
	}

	if err := w.Apply(context.Background(), func(tx *sql.Tx) error {
		return upsertMailbox(tx, accountID, "mb-inbox", backend.MailboxFields{Name: "Inbox"})
	}); err != nil {
		t.Fatalf("upsertMailbox: %v", err)
	}

	inboxID := seedMailboxID(t, w, accountID, "mb-inbox")
	got := mailboxIDsForServerID(t, w, accountID)
	if !slices.Equal(got, []int64{inboxID}) {
		t.Fatalf("mailboxes after the mailbox is created = %v, want [%d] (Inbox), repaired without another message update", got, inboxID)
	}
	if !unreadForMessage(t, w, accountID, inboxID) {
		t.Fatal("unread = false, want true: FlagSeen was clear")
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
