package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// TestIdempotentReplay covers every intent kind: a second dispatch of
// an equivalent row (standing in for the recovery requeue a crash
// between a successful backend call and this pass's finalize step
// would produce) leaves the same server and store state, and does not
// repeat a side effect the backend itself cannot absorb twice.
func TestIdempotentReplay(t *testing.T) {
	t.Run("create mailbox", func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		createCalls := 0
		be := newFakeBackend()
		be.MailSource.CreateMailboxFunc = func(_ context.Context, name, _ string) (string, error) {
			createCalls++
			return "mbx-1", nil
		}
		dispatcher := NewDispatcher(accountID, be, w)

		id1, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, time.Now())
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		if _, err := dispatcher.DispatchOnce(context.Background(), time.Now()); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if n := outboxCount(t, w, id1); n != 0 {
			t.Fatalf("intent %d still queued after success", id1)
		}
		if createCalls != 1 {
			t.Fatalf("CreateMailbox calls = %d, want 1", createCalls)
		}

		// Manufacture the replay: a row already carrying the
		// resolved server id, as recovery would find after a crash
		// between the backend call and this pass's finalize.
		payload, err := json.Marshal(CreateMailboxPayload{Name: "Projects", ResolvedServerID: "mbx-1"})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var id2 int64
		err = w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
			var txErr error
			id2, txErr = insertRow(tx, accountID, KindCreateMailbox, payload, "replay", 0, time.Now(), time.Now())
			return txErr
		})
		if err != nil {
			t.Fatalf("insert replay row: %v", err)
		}
		if _, err := dispatcher.DispatchOnce(context.Background(), time.Now()); err != nil {
			t.Fatalf("dispatch replay: %v", err)
		}
		if n := outboxCount(t, w, id2); n != 0 {
			t.Fatalf("replayed intent %d still queued", id2)
		}
		if createCalls != 1 {
			t.Fatalf("CreateMailbox calls after replay = %d, want 1 (memoized)", createCalls)
		}
	})

	t.Run("rename mailbox", func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)
		mailboxID := seedMailbox(t, w, accountID, "Old", "mbx-1")

		renameCalls := 0
		var lastName string
		be := newFakeBackend()
		be.MailSource.RenameMailboxFunc = func(_ context.Context, _, name string) error {
			renameCalls++
			lastName = name
			return nil
		}
		dispatcher := NewDispatcher(accountID, be, w)

		for i := range 2 {
			id, _, err := EnqueueRenameMailbox(context.Background(), w, accountID, mailboxID, "Family", time.Now())
			if err != nil {
				t.Fatalf("enqueue %d: %v", i, err)
			}
			if _, err := dispatcher.DispatchOnce(context.Background(), time.Now()); err != nil {
				t.Fatalf("dispatch %d: %v", i, err)
			}
			if n := outboxCount(t, w, id); n != 0 {
				t.Fatalf("intent %d still queued", id)
			}
		}
		if renameCalls != 2 {
			t.Fatalf("RenameMailbox calls = %d, want 2", renameCalls)
		}
		if lastName != "Family" {
			t.Fatalf("final name = %q, want %q", lastName, "Family")
		}
	})

	t.Run("delete mailbox", func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)
		mailboxID := seedMailbox(t, w, accountID, "Spam", "mbx-1")

		deleteCalls := 0
		be := newFakeBackend()
		be.MailSource.DeleteMailboxFunc = func(_ context.Context, _ string) error {
			deleteCalls++
			return nil // a real backend absorbs a repeat delete as notFound-is-success; the seam already returns nil for it
		}
		dispatcher := NewDispatcher(accountID, be, w)

		for i := range 2 {
			id, _, err := EnqueueDeleteMailbox(context.Background(), w, accountID, mailboxID, time.Now())
			if err != nil {
				t.Fatalf("enqueue %d: %v", i, err)
			}
			if _, err := dispatcher.DispatchOnce(context.Background(), time.Now()); err != nil {
				t.Fatalf("dispatch %d: %v", i, err)
			}
			if n := outboxCount(t, w, id); n != 0 {
				t.Fatalf("intent %d still queued", id)
			}
		}
		if deleteCalls != 2 {
			t.Fatalf("DeleteMailbox calls = %d, want 2", deleteCalls)
		}
	})

	t.Run("move messages", func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)
		src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
		dest := seedMailbox(t, w, accountID, "Archive", "mbx-dest")
		msgID := seedMessage(t, w, accountID, src, "msg-1")

		applyCalls := 0
		serverMailbox := map[string]string{"msg-1": "mbx-src"}
		be := newFakeBackend()
		be.MailSource.ApplyBatchFunc = func(_ context.Context, muts []backend.Mutation) (backend.BatchResult, error) {
			applyCalls++
			for _, m := range muts {
				ids, _ := m.Fields["mailbox_ids"].([]string)
				if len(ids) > 0 {
					serverMailbox[m.ID] = ids[0]
				}
			}
			return backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}, nil
		}
		dispatcher := NewDispatcher(accountID, be, w)

		for i := range 2 {
			_, ids, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{msgID}, dest, 0, be, false, time.Now())
			if err != nil {
				t.Fatalf("enqueue %d: %v", i, err)
			}
			if _, err := dispatcher.DispatchOnce(context.Background(), time.Now()); err != nil {
				t.Fatalf("dispatch %d: %v", i, err)
			}
			if n := outboxCount(t, w, ids[0]); n != 0 {
				t.Fatalf("intent %d still queued", ids[0])
			}
		}
		if applyCalls != 2 {
			t.Fatalf("ApplyBatch calls = %d, want 2", applyCalls)
		}
		if serverMailbox["msg-1"] != "mbx-dest" {
			t.Fatalf("msg-1's server mailbox = %q, want %q", serverMailbox["msg-1"], "mbx-dest")
		}
	})
}
