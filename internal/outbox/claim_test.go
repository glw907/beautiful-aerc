package outbox

import (
	"context"
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
