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
		w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
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
		dispatcher := NewDispatcher(accountID, be, w, reads)

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

// TestChunkSizeAtAdvertisedLimits holds the two bounds apart at the
// limits real JMAP servers advertise. A server's MaxObjectsInSet
// bounds one request; moveChunkMessages bounds one store transaction,
// and it is the smaller of the two at every limit poplar has seen
// reported, Fastmail's 4096 included.
func TestChunkSizeAtAdvertisedLimits(t *testing.T) {
	tests := []struct {
		name          string
		maxObjects    int
		wantWireBatch int
		wantChunk     int
	}{
		{"fastmail", 4096, 4096, moveChunkMessages},
		{"stalwart", 1000, 1000, moveChunkMessages},
		{"rfc 8620 sample session", 128, 128, 128},
		{"unreported", 0, defaultWireBatch, defaultWireBatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			be := newFakeBackend()
			be.Caps.Limits.MaxObjectsInSet = tt.maxObjects

			if got := wireBatchLimit(be); got != tt.wantWireBatch {
				t.Errorf("wireBatchLimit = %d, want %d", got, tt.wantWireBatch)
			}
			if got := moveChunkSize(be); got != tt.wantChunk {
				t.Errorf("moveChunkSize = %d, want %d", got, tt.wantChunk)
			}
		})
	}
}

// TestIdleDispatchLeavesTheInteractiveLaneAlone holds the other half
// of ADR-0003's lane discipline over the dispatch pass. Its caller
// polls on a fixed cadence whether or not anything is queued, and the
// sync engine's bulk lane yields for a whole quiet window after every
// interactive commit, so a pass that opened a transaction to find
// nothing to do would throttle background sync for the life of the
// process. The account row is seeded on the bulk lane so the only
// interactive activity this can observe is the pass itself.
func TestIdleDispatchLeavesTheInteractiveLaneAlone(t *testing.T) {
	w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())

	var accountID int64
	err := w.Apply(context.Background(), func(tx *sql.Tx) error {
		res, err := tx.Exec(`INSERT INTO account (slug, backend_kind, address) VALUES (?, ?, ?)`, "acct", "jmap", "geoff@example.com")
		if err != nil {
			return err
		}
		accountID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		t.Fatalf("seed the account on the bulk lane: %v", err)
	}
	if w.RecentInteractiveActivity(time.Hour) {
		t.Fatal("the fixture itself used the interactive lane, so this test could not tell")
	}

	be := newFakeBackend()
	be.MailSource.RenameMailboxFunc = func(_ context.Context, _, _ string) error { return nil }
	dispatcher := NewDispatcher(accountID, be, w, reads)

	if _, err := dispatcher.DispatchOnce(context.Background(), time.Now()); err != nil {
		t.Fatalf("idle dispatch: %v", err)
	}
	if w.RecentInteractiveActivity(time.Hour) {
		t.Error("a pass with nothing eligible opened an interactive transaction")
	}

	// The same probe must not talk a pass out of real work.
	mailboxID := seedMailbox(t, w, accountID, "Old", "mbx-1")
	if _, _, err := EnqueueRenameMailbox(context.Background(), w, accountID, mailboxID, "New", time.Now()); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	result, err := dispatcher.DispatchOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(result.Delivered) != 1 {
		t.Fatalf("Delivered = %+v, want the one queued intent", result.Delivered)
	}
}

// TestClaimIsBounded holds ADR-0003's admission ceiling over the claim
// transaction at the limit the production backend actually reports.
// The claim resolves every referent on the writer's single connection
// before any I/O, so an unbounded claim after a bulk action's hold
// expires runs one point query per message in the queue inside one
// interactive transaction, blocking every other writer for as long as
// that takes. What one pass leaves behind the next takes.
func TestClaimIsBounded(t *testing.T) {
	t.Run("row budget", func(t *testing.T) {
		w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		be := newFakeBackend()
		be.Caps.Limits.MaxObjectsInSet = fastmailMaxObjectsInSet
		be.MailSource.RenameMailboxFunc = func(_ context.Context, _, _ string) error { return nil }

		ids := make([]int64, 0, claimRowBudget+1)
		for i := range claimRowBudget + 1 {
			mailboxID := seedMailbox(t, w, accountID, fmt.Sprintf("Old %d", i), fmt.Sprintf("mbx-%d", i))
			id, _, err := EnqueueRenameMailbox(context.Background(), w, accountID, mailboxID, "Family", time.Now())
			if err != nil {
				t.Fatalf("enqueue %d: %v", i, err)
			}
			ids = append(ids, id)
		}

		dispatcher := NewDispatcher(accountID, be, w, reads)
		result, err := dispatcher.DispatchOnce(context.Background(), time.Now())
		if err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if len(result.Delivered) != claimRowBudget {
			t.Fatalf("delivered = %d, want %d (the claim's row budget)", len(result.Delivered), claimRowBudget)
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
	})

	t.Run("message budget", func(t *testing.T) {
		w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)
		src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
		dest := seedMailbox(t, w, accountID, "Archive", "mbx-dest")

		be := newFakeBackend()
		be.Caps.Limits.MaxObjectsInSet = fastmailMaxObjectsInSet

		// One chunk more than the message budget admits, so the bound
		// under test is the messages rather than the rows.
		total := claimMessageBudget + moveChunkSize(be)
		messageIDs := make([]int64, total)
		for i := range messageIDs {
			messageIDs[i] = seedMessage(t, w, accountID, src, fmt.Sprintf("msg-%d", i))
		}
		_, intentIDs, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, messageIDs, dest, 0, be, false, time.Now())
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		wantChunks := total / moveChunkSize(be)
		if len(intentIDs) != wantChunks {
			t.Fatalf("chunks = %d, want %d", len(intentIDs), wantChunks)
		}

		claimed, err := NewDispatcher(accountID, be, w, reads).claim(context.Background(), time.Now())
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		messages := 0
		for _, c := range claimed {
			messages += len(c.messageServerIDs)
		}
		if messages > claimMessageBudget {
			t.Errorf("claim resolved %d messages, want at most %d", messages, claimMessageBudget)
		}
		if want := claimMessageBudget / moveChunkSize(be); len(claimed) != want {
			t.Errorf("claimed rows = %d, want %d: the budget is spent on whole chunks", len(claimed), want)
		}
	})

	t.Run("a chunk wider than the budget is still claimed", func(t *testing.T) {
		w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)
		src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
		dest := seedMailbox(t, w, accountID, "Archive", "mbx-dest")

		// The shape an older build wrote, chunked at the server's own
		// batch limit rather than at a transaction budget. No pass may
		// refuse it, or the queue behind it never moves.
		oversized := claimMessageBudget + 1
		messageIDs := make([]int64, oversized)
		for i := range messageIDs {
			messageIDs[i] = seedMessage(t, w, accountID, src, fmt.Sprintf("msg-%d", i))
		}
		now := time.Now()
		var intentID int64
		err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
			var txErr error
			intentID, txErr = EnqueueMoveMessagesChunkTx(tx, accountID, messageIDs, dest, 0, newUndoGroup(), 0, now, now)
			return txErr
		})
		if err != nil {
			t.Fatalf("enqueue oversized chunk: %v", err)
		}

		be := newFakeBackend()
		be.Caps.Limits.MaxObjectsInSet = fastmailMaxObjectsInSet
		claimed, err := NewDispatcher(accountID, be, w, reads).claim(context.Background(), now)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if len(claimed) != 1 || claimed[0].id != intentID {
			t.Fatalf("claimed = %d rows, want the one oversized chunk (intent %d)", len(claimed), intentID)
		}
	})
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
