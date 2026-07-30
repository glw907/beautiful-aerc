package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/uerr"
)

// TestKeyResolutionAtDispatch covers an offline create-folder-then-
// move: both intents are enqueued referencing poplar's internal keys
// only (the move's destination names the create intent's own id, not
// a server id that does not exist yet), and one DispatchOnce pass
// dispatches them as one batch, the move naming its destination by the
// create's creation id so the server resolves the back-reference
// itself.
func TestKeyResolutionAtDispatch(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
	msgID := seedMessage(t, w, accountID, src, "msg-1")

	createMailboxCalls := 0
	var batches [][]backend.Mutation
	be := newFakeBackend()
	be.MailSource.CreateMailboxFunc = func(_ context.Context, _, _ string) (string, error) {
		createMailboxCalls++
		return "mbx-separate-call", nil
	}
	be.MailSource.ApplyBatchFunc = func(_ context.Context, muts []backend.Mutation) (backend.BatchResult, error) {
		batches = append(batches, muts)
		created := map[string]string{}
		for _, mut := range muts {
			if mut.Op == backend.MutationCreate {
				created[mut.CreationID] = "mbx-new-1"
			}
		}
		return backend.BatchResult{Created: created, Failed: map[string]error{}}, nil
	}
	dispatcher := NewDispatcher(accountID, be, w)

	now := time.Now()
	createID, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, now)
	if err != nil {
		t.Fatalf("enqueue create: %v", err)
	}
	_, moveIDs, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{msgID}, 0, createID, be, false, now)
	if err != nil {
		t.Fatalf("enqueue move: %v", err)
	}

	result, err := dispatcher.DispatchOnce(context.Background(), now)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if createMailboxCalls != 0 {
		t.Errorf("CreateMailbox calls = %d, want 0 (the create rides the batch)", createMailboxCalls)
	}
	if len(batches) != 1 {
		t.Fatalf("ApplyBatch calls = %d, want 1 (create and move dispatch as one batch)", len(batches))
	}
	muts := batches[0]
	if len(muts) != 2 {
		t.Fatalf("batch mutations = %+v, want the create and the move", muts)
	}
	create := muts[0]
	if create.Op != backend.MutationCreate || create.Kind != backend.ObjectKindMailbox {
		t.Fatalf("mutation 0 = op %v kind %v, want a mailbox create", create.Op, create.Kind)
	}
	if create.CreationID == "" {
		t.Fatal("mailbox create carries no CreationID for the move to reference")
	}
	if box, _ := create.Fields.(backend.MailboxCreate); box.Name != "Projects" {
		t.Errorf("created name = %v, want Projects", create.Fields)
	}
	move := muts[1]
	if move.ID != "msg-1" {
		t.Errorf("mutation 1 ID = %q, want %q", move.ID, "msg-1")
	}
	patch, _ := move.Fields.(backend.MessagePatch)
	if len(patch.MailboxIDs) != 1 || patch.MailboxIDs[0] != "#"+create.CreationID {
		t.Errorf("MailboxIDs = %v, want [#%s] (the create's back-reference)", patch.MailboxIDs, create.CreationID)
	}

	if len(result.Delivered) != 2 {
		t.Fatalf("Delivered = %+v, want both intents", result.Delivered)
	}
	if n := outboxCount(t, w, createID); n != 0 {
		t.Errorf("create intent %d still queued", createID)
	}
	if n := outboxCount(t, w, moveIDs[0]); n != 0 {
		t.Errorf("move intent %d still queued", moveIDs[0])
	}
}

// TestKeyResolutionSurvivesRequeue covers the cross-pass case
// TestKeyResolutionAtDispatch's single batch cannot: the create
// dispatches on its own and its row is deleted in the same pass, but
// the dependent move fails and requeues. A later pass, with the
// create's row long gone, must still resolve the move's destination,
// because the dispatcher persisted the resolved server id into the
// move's own payload the moment the create succeeded.
//
// A one-object MaxObjectsInSet is what splits them: the create's own
// mutation exhausts the batch's budget, so the move cannot ride it and
// takes the separate-call path this test needs.
func TestKeyResolutionSurvivesRequeue(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
	msgID := seedMessage(t, w, accountID, src, "msg-1")

	applyCalls := 0
	be := newFakeBackend()
	be.Caps.Limits.MaxObjectsInSet = 1
	be.MailSource.CreateMailboxFunc = func(_ context.Context, _, _ string) (string, error) {
		return "mbx-new-1", nil
	}
	be.MailSource.ApplyBatchFunc = func(_ context.Context, muts []backend.Mutation) (backend.BatchResult, error) {
		applyCalls++
		if applyCalls == 1 {
			return backend.BatchResult{}, backend.Failure{Class: uerr.ClassConnection, Cause: errors.New("connection dropped")}
		}
		return backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}, nil
	}
	dispatcher := NewDispatcher(accountID, be, w)

	now := time.Now()
	createID, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, now)
	if err != nil {
		t.Fatalf("enqueue create: %v", err)
	}
	_, moveIDs, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{msgID}, 0, createID, be, false, now)
	if err != nil {
		t.Fatalf("enqueue move: %v", err)
	}

	if _, err := dispatcher.DispatchOnce(context.Background(), now); err != nil {
		t.Fatalf("pass 1: %v", err)
	}
	if n := outboxCount(t, w, createID); n != 0 {
		t.Fatalf("create intent %d still queued after it dispatched", createID)
	}
	if state, attempts := outboxState(t, w, moveIDs[0]); state != "queued" || attempts != 1 {
		t.Fatalf("move intent state = %s attempts = %d, want queued/1", state, attempts)
	}

	pass2 := now.Add(2 * time.Second)
	if _, err := dispatcher.DispatchOnce(context.Background(), pass2); err != nil {
		t.Fatalf("pass 2: %v", err)
	}
	if applyCalls != 2 {
		t.Fatalf("ApplyBatch calls = %d, want 2", applyCalls)
	}
	if n := outboxCount(t, w, moveIDs[0]); n != 0 {
		t.Errorf("move intent %d still queued after pass 2, its destination reference was lost", moveIDs[0])
	}
}
