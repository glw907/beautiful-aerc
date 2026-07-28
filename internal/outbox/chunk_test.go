package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/uerr"
)

// TestChunkedBulk covers ADR-0006 revision 2's batch shape: each
// chunk sized under the backend's limit, the compensating group
// restoring exact prior state per message, and partial dispatch
// retrying only the chunks a prior pass never finished.
func TestChunkedBulk(t *testing.T) {
	t.Run("chunk size bound", func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)
		src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
		dest := seedMailbox(t, w, accountID, "Archive", "mbx-dest")

		const total = 25
		const maxPerChunk = 10
		messageIDs := make([]int64, total)
		for i := range messageIDs {
			messageIDs[i] = seedMessage(t, w, accountID, src, "msg-"+strconv.Itoa(i))
		}

		be := newFakeBackend()
		be.Caps.Limits.MaxObjectsInSet = maxPerChunk
		undoGroup, intentIDs, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, messageIDs, dest, 0, be, false, time.Now())
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		if len(intentIDs) != 3 {
			t.Fatalf("chunk count = %d, want 3 (ceil(25/10))", len(intentIDs))
		}

		wantSizes := []int{10, 10, 5}
		for i, id := range intentIDs {
			p := readMovePayload(t, w, id)
			if len(p.MessageIDs) != wantSizes[i] {
				t.Errorf("chunk %d size = %d, want %d", i, len(p.MessageIDs), wantSizes[i])
			}
			if got := readUndoGroup(t, w, id); got != undoGroup {
				t.Errorf("chunk %d undo_group = %q, want %q", i, got, undoGroup)
			}
		}
	})

	t.Run("partial dispatch retries only unfinished chunks", func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)
		src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
		dest := seedMailbox(t, w, accountID, "Archive", "mbx-dest")

		msgIDs := []int64{
			seedMessage(t, w, accountID, src, "msg-0"),
			seedMessage(t, w, accountID, src, "msg-1"),
			seedMessage(t, w, accountID, src, "msg-2"),
		}
		be := newFakeBackend()
		be.Caps.Limits.MaxObjectsInSet = 1
		_, intentIDs, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, msgIDs, dest, 0, be, false, time.Now())
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		if len(intentIDs) != 3 {
			t.Fatalf("chunk count = %d, want 3", len(intentIDs))
		}

		applyCalls := 0
		be.MailSource.ApplyBatchFunc = func(_ context.Context, muts []backend.Mutation) (backend.BatchResult, error) {
			applyCalls++
			if applyCalls == 2 {
				return backend.BatchResult{}, backend.MutationFailure{Class: uerr.ClassConnection, Cause: errors.New("connection dropped")}
			}
			return backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}, nil
		}
		dispatcher := NewDispatcher(accountID, be, w)

		pass1 := time.Now()
		if _, err := dispatcher.DispatchOnce(context.Background(), pass1); err != nil {
			t.Fatalf("pass 1: %v", err)
		}
		if applyCalls != 2 {
			t.Fatalf("ApplyBatch calls after pass 1 = %d, want 2 (chunk 2 never attempted)", applyCalls)
		}
		if n := outboxCount(t, w, intentIDs[0]); n != 0 {
			t.Errorf("chunk 0 (succeeded) still present")
		}
		if state, attempts := outboxState(t, w, intentIDs[1]); state != "queued" || attempts != 1 {
			t.Errorf("chunk 1 (failed) state = %s attempts = %d, want queued/1", state, attempts)
		}
		if state, attempts := outboxState(t, w, intentIDs[2]); state != "queued" || attempts != 0 {
			t.Errorf("chunk 2 (never attempted) state = %s attempts = %d, want queued/0", state, attempts)
		}

		be.MailSource.ApplyBatchFunc = func(_ context.Context, muts []backend.Mutation) (backend.BatchResult, error) {
			applyCalls++
			return backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}, nil
		}
		pass2 := pass1.Add(2 * time.Second)
		if _, err := dispatcher.DispatchOnce(context.Background(), pass2); err != nil {
			t.Fatalf("pass 2: %v", err)
		}
		if applyCalls != 4 {
			t.Fatalf("ApplyBatch calls after pass 2 = %d, want 4 (chunk 1 retried, chunk 2 attempted)", applyCalls)
		}
		for _, id := range intentIDs {
			if n := outboxCount(t, w, id); n != 0 {
				t.Errorf("intent %d still present after pass 2", id)
			}
		}
	})

	t.Run("compensating group restores exact prior state per message", func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)
		src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
		dest := seedMailbox(t, w, accountID, "Archive", "mbx-dest")

		msgIDs := []int64{
			seedMessage(t, w, accountID, src, "msg-0"),
			seedMessage(t, w, accountID, src, "msg-1"),
		}

		serverMailbox := map[string]string{"msg-0": "mbx-src", "msg-1": "mbx-src"}
		be := newFakeBackend()
		be.MailSource.ApplyBatchFunc = func(_ context.Context, muts []backend.Mutation) (backend.BatchResult, error) {
			for _, m := range muts {
				ids, _ := m.Fields["mailbox_ids"].([]string)
				if len(ids) > 0 {
					serverMailbox[m.ID] = ids[0]
				}
			}
			return backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}, nil
		}
		dispatcher := NewDispatcher(accountID, be, w)

		_, _, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, msgIDs, dest, 0, be, false, time.Now())
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		result, err := dispatcher.DispatchOnce(context.Background(), time.Now())
		if err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if len(result.Delivered) != 1 || result.Delivered[0].Move == nil {
			t.Fatalf("Delivered = %+v, want one move entry with a resolved payload", result.Delivered)
		}
		prior := result.Delivered[0].Move.PriorMailboxIDs
		for _, id := range msgIDs {
			if prior[id] != src {
				t.Errorf("prior mailbox for message %d = %d, want %d (src)", id, prior[id], src)
			}
		}
		if serverMailbox["msg-0"] != "mbx-dest" || serverMailbox["msg-1"] != "mbx-dest" {
			t.Fatalf("server state after move = %+v, want both in mbx-dest", serverMailbox)
		}

		// The caller (pass 2's overlay) restores exact prior state
		// using the payload it was just handed, with no second store
		// read: every message here shares one prior mailbox, so one
		// compensating move suffices.
		_, _, err = EnqueueMoveMessagesBulk(context.Background(), w, accountID, msgIDs, src, 0, be, false, time.Now())
		if err != nil {
			t.Fatalf("enqueue compensation: %v", err)
		}
		if _, err := dispatcher.DispatchOnce(context.Background(), time.Now()); err != nil {
			t.Fatalf("dispatch compensation: %v", err)
		}
		if serverMailbox["msg-0"] != "mbx-src" || serverMailbox["msg-1"] != "mbx-src" {
			t.Fatalf("server state after compensation = %+v, want both restored to mbx-src", serverMailbox)
		}
	})
}

func readMovePayload(t *testing.T, w *store.Writer, id int64) MoveMessagesPayload {
	t.Helper()
	raw := readPayload(t, w, id)
	var p MoveMessagesPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal move payload %d: %v", id, err)
	}
	return p
}
