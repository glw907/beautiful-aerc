package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// TestClaimIsTransactional races Undo against DispatchOnce's claim
// for every intent in one trial, many trials over, and asserts the
// central invariant ADR-0006 revision 2 exists to guarantee: no
// intent is ever both annihilated (Undo caught it still queued) and
// sent (the dispatcher's backend call went out for it). The claim
// query moving queued to dispatching and Undo's conditional delete
// are the same guarded statement from opposite sides, so whichever of
// the two transactions the writer's single connection runs first for
// a given row decides that row's fate for good.
func TestClaimIsTransactional(t *testing.T) {
	const messagesPerTrial = 20
	const trials = 20

	for trial := range trials {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)
		src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
		dest := seedMailbox(t, w, accountID, "Archive", "mbx-dest")

		messageIDs := make([]int64, messagesPerTrial)
		for i := range messageIDs {
			messageIDs[i] = seedMessage(t, w, accountID, src, fmt.Sprintf("trial%d-msg%d", trial, i))
		}

		be := newFakeBackend()
		be.Caps.Limits.MaxObjectsInSet = 1
		_, intentIDs, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, messageIDs, dest, 0, be, false, time.Now())
		if err != nil {
			t.Fatalf("trial %d: enqueue: %v", trial, err)
		}

		var mu sync.Mutex
		var sent []string
		be.MailSource.ApplyBatchFunc = func(_ context.Context, muts []backend.Mutation) (backend.BatchResult, error) {
			mu.Lock()
			for _, m := range muts {
				sent = append(sent, m.ID)
			}
			mu.Unlock()
			return backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}, nil
		}
		dispatcher := NewDispatcher(accountID, be, w)

		start := make(chan struct{})
		var wg sync.WaitGroup
		annihilatedBy := make([][]int64, len(intentIDs))
		for i, id := range intentIDs {
			wg.Go(func() {
				<-start
				got, err := Undo(context.Background(), w, []int64{id})
				if err != nil {
					t.Errorf("trial %d: undo %d: %v", trial, id, err)
					return
				}
				annihilatedBy[i] = got
			})
		}
		wg.Go(func() {
			<-start
			if _, err := dispatcher.DispatchOnce(context.Background(), time.Now()); err != nil {
				t.Errorf("trial %d: dispatch: %v", trial, err)
			}
		})
		close(start)
		wg.Wait()

		annihilated := map[int64]bool{}
		for _, got := range annihilatedBy {
			for _, id := range got {
				annihilated[id] = true
			}
		}
		mu.Lock()
		sentSet := make(map[string]bool, len(sent))
		for _, s := range sent {
			sentSet[s] = true
		}
		mu.Unlock()

		for i, id := range intentIDs {
			serverID := fmt.Sprintf("trial%d-msg%d", trial, i)
			if annihilated[id] && sentSet[serverID] {
				t.Fatalf("trial %d: intent %d both annihilated and sent", trial, id)
			}
			if !annihilated[id] && !sentSet[serverID] {
				t.Fatalf("trial %d: intent %d neither annihilated nor sent", trial, id)
			}
		}
	}
}

// TestClaimIsBounded holds ADR-0003's admission ceiling over the claim
// transaction. The claim resolves every referent on the writer's
// single connection before any I/O, so an unbounded claim after a bulk
// action's hold expires runs one point query per message in the queue
// inside one interactive transaction, blocking every other writer for
// as long as that takes. What one pass leaves behind the next takes.
func TestClaimIsBounded(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	// claimMessageBudget divided by the backend's per-batch limit, the
	// largest number of messages one claimed chunk can name.
	const limit = 10

	be := newFakeBackend()
	be.Caps.Limits.MaxObjectsInSet = 100
	be.MailSource.RenameMailboxFunc = func(_ context.Context, _, _ string) error { return nil }

	if got := claimLimit(be); got != limit {
		t.Fatalf("claimLimit = %d, want %d", got, limit)
	}

	ids := make([]int64, 0, limit+1)
	for i := range limit + 1 {
		mailboxID := seedMailbox(t, w, accountID, fmt.Sprintf("Old %d", i), fmt.Sprintf("mbx-%d", i))
		id, _, err := EnqueueRenameMailbox(context.Background(), w, accountID, mailboxID, "Family", time.Now())
		if err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
		ids = append(ids, id)
	}

	dispatcher := NewDispatcher(accountID, be, w)
	result, err := dispatcher.DispatchOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(result.Delivered) != limit {
		t.Fatalf("delivered = %d, want %d (the claim's bound)", len(result.Delivered), limit)
	}

	last := ids[len(ids)-1]
	if state, attempts := outboxState(t, w, last); state != "queued" || attempts != 0 {
		t.Errorf("intent %d state = %s attempts = %d, want queued/0: it was never claimed", last, state, attempts)
	}

	if _, err := dispatcher.DispatchOnce(context.Background(), time.Now()); err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	if n := outboxCount(t, w, last); n != 0 {
		t.Errorf("intent %d survived the second pass, want the next pass to drain it", last)
	}
}

// TestUndoReportsNothingWhenItsTransactionRollsBack pins the other
// half of Undo's contract: its id list names rows a committed
// transaction deleted. A delete that aborts partway rolls the whole
// transaction back, earlier deletes included, so a caller that read
// an id list alongside the error would skip compensating for rows
// that are still there.
func TestUndoReportsNothingWhenItsTransactionRollsBack(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
	dest := seedMailbox(t, w, accountID, "Archive", "mbx-dest")
	msgIDs := []int64{
		seedMessage(t, w, accountID, src, "msg-0"),
		seedMessage(t, w, accountID, src, "msg-1"),
	}

	be := newFakeBackend()
	be.Caps.Limits.MaxObjectsInSet = 1
	_, intentIDs, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, msgIDs, dest, 0, be, false, time.Now())
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if len(intentIDs) != 2 {
		t.Fatalf("chunk count = %d, want 2", len(intentIDs))
	}

	// Aborting the second chunk's delete leaves the first one already
	// deleted inside the same transaction, the shape that separates a
	// list built inside the closure from one the commit earned.
	err = w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		_, execErr := tx.Exec(`
			CREATE TRIGGER fail_second_delete
			BEFORE DELETE ON outbox
			WHEN OLD.chunk_seq = 1
			BEGIN SELECT RAISE(ABORT, 'simulated delete failure'); END`)
		return execErr
	})
	if err != nil {
		t.Fatalf("install failing trigger: %v", err)
	}

	annihilated, err := Undo(context.Background(), w, intentIDs)
	if err == nil {
		t.Fatal("expected an error from the aborted delete")
	}
	if len(annihilated) != 0 {
		t.Errorf("annihilated = %v, want none: the transaction rolled back", annihilated)
	}
	for _, id := range intentIDs {
		if n := outboxCount(t, w, id); n != 1 {
			t.Errorf("intent %d is gone, want it restored by the rollback", id)
		}
	}
}
