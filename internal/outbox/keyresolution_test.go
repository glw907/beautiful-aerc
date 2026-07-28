package outbox

import (
	"context"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// TestKeyResolutionAtDispatch covers an offline create-folder-then-
// move: both intents are enqueued referencing poplar's internal keys
// only (the move's destination names the create intent's own id, not
// a server id that does not exist yet), and one DispatchOnce pass
// resolves the back-reference and dispatches both.
//
// The backend seam as built does not let this reach the wire as one
// JMAP request: Mail.ApplyBatch speaks only Email/set, and mailbox
// lifecycle (CreateMailbox) is a separate call with no creation-id
// parameter of its own (ADR-0004 revision 2's mailbox lifecycle is
// explicit calls, not a Mutation the way a message create would be).
// This test proves the resolution ADR-0006 revision 2 actually
// requires: the dispatcher's own claim-time key resolution, not a
// combined wire batch.
func TestKeyResolutionAtDispatch(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
	msgID := seedMessage(t, w, accountID, src, "msg-1")

	var createdName, createdParent string
	var moveMutations []backend.Mutation
	be := newFakeBackend()
	be.MailSource.CreateMailboxFunc = func(_ context.Context, name, parentID string) (string, error) {
		createdName, createdParent = name, parentID
		return "mbx-new-1", nil
	}
	be.MailSource.ApplyBatchFunc = func(_ context.Context, muts []backend.Mutation) (backend.BatchResult, error) {
		moveMutations = muts
		return backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}, nil
	}
	dispatcher := NewDispatcher(accountID, be, w)

	now := time.Now()
	createID, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, now)
	if err != nil {
		t.Fatalf("enqueue create: %v", err)
	}
	_, moveIDs, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{msgID}, 0, createID, 10, false, now)
	if err != nil {
		t.Fatalf("enqueue move: %v", err)
	}

	result, err := dispatcher.DispatchOnce(context.Background(), now)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if createdName != "Projects" || createdParent != "" {
		t.Fatalf("CreateMailbox called with (%q, %q), want (%q, \"\")", createdName, createdParent, "Projects")
	}
	if len(moveMutations) != 1 {
		t.Fatalf("ApplyBatch mutations = %d, want 1", len(moveMutations))
	}
	mut := moveMutations[0]
	if mut.ID != "msg-1" {
		t.Errorf("mutation ID = %q, want %q", mut.ID, "msg-1")
	}
	ids, _ := mut.Fields["mailbox_ids"].([]string)
	if len(ids) != 1 || ids[0] != "mbx-new-1" {
		t.Errorf("mailbox_ids = %v, want [mbx-new-1] (the offline create's resolved server id)", ids)
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
