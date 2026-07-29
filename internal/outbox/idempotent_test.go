package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
// repeat a side effect the backend itself cannot absorb twice. A
// create replays from a payload already carrying its resolved server
// id, which is the state the transaction after the backend call
// leaves behind; TestCreateMailboxReplayWindow holds the crash that
// lands between the two.
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

// TestCreateMailboxReplayWindow states the one window replay is not
// idempotent across. A create records its resolved server id in the
// transaction after its backend call, so a run killed between the two
// leaves a mailbox on the server and a payload that never learned its
// id. The startup sweep requeues that row, and the replay creates the
// mailbox a second time: the account ends up with two folders of the
// same name.
//
// Closing the window needs a mailbox lookup the backend seam does not
// carry, plus a rule for when poplar adopts a folder it did not
// create. Both sit outside internal/outbox. Until they exist this
// test holds the behavior in place, so changing it is a decision
// somebody made rather than a regression nobody saw.
func TestCreateMailboxReplayWindow(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	var created []string
	be := newFakeBackend()
	be.MailSource.CreateMailboxFunc = func(_ context.Context, name, _ string) (string, error) {
		created = append(created, name)
		return fmt.Sprintf("mbx-%d", len(created)), nil
	}

	id, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, time.Now())
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// The killed run, reconstructed: its claim transaction committed,
	// its CreateMailbox call reached the server, and it died before
	// the transaction that writes the new id into the payload.
	strandDispatching(t, w, id)
	if _, err := be.Mail().CreateMailbox(context.Background(), "Projects", ""); err != nil {
		t.Fatalf("create the mailbox the killed run created: %v", err)
	}

	if err := ReclaimOrphaned(context.Background(), w); err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if _, err := NewDispatcher(accountID, be, w).DispatchOnce(context.Background(), time.Now()); err != nil {
		t.Fatalf("dispatch the reclaimed intent: %v", err)
	}

	if n := outboxCount(t, w, id); n != 0 {
		t.Fatalf("intent %d still queued after its replay", id)
	}
	if len(created) != 2 {
		t.Errorf("CreateMailbox calls = %d, want 2: the replay leaves the account holding two Projects mailboxes", len(created))
	}
}
