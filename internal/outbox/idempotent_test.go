package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
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
		w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		createCalls := 0
		be := newFakeBackend()
		be.MailSource.CreateMailboxFunc = func(_ context.Context, name, _ string) (string, error) {
			createCalls++
			return "mbx-1", nil
		}
		dispatcher := NewDispatcher(accountID, be, w, reads)

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
		w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
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
		dispatcher := NewDispatcher(accountID, be, w, reads)

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
		w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)
		mailboxID := seedMailbox(t, w, accountID, "Spam", "mbx-1")

		deleteCalls := 0
		be := newFakeBackend()
		be.MailSource.DeleteMailboxFunc = func(_ context.Context, _ string) error {
			deleteCalls++
			return nil // a real backend absorbs a repeat delete as notFound-is-success; the seam already returns nil for it
		}
		dispatcher := NewDispatcher(accountID, be, w, reads)

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
		w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
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
				patch, _ := m.Fields.(backend.MessagePatch)
				if len(patch.MailboxIDs) > 0 {
					serverMailbox[m.ID] = patch.MailboxIDs[0]
				}
			}
			return backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}, nil
		}
		dispatcher := NewDispatcher(accountID, be, w, reads)

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

// TestCreateMailboxReplayWindow reconstructs the window a create used
// to duplicate a folder across, and holds it closed. A create records
// its resolved server id in the transaction after its backend call, so
// a run killed between the two leaves a mailbox on the server and a
// payload that never learned its id. The startup sweep requeues that
// row and the replay makes the call a second time, which is the one
// thing the dispatcher cannot avoid: a reclaimed row is byte-identical
// to a fresh one.
//
// The server is what closes it. RFC 8621 section 2 forbids two sibling
// mailboxes with the same parent and the same name, so the replay's
// create is refused and the dispatcher adopts the mailbox that refusal
// is about. The account ends with one folder and the intent completes.
func TestCreateMailboxReplayWindow(t *testing.T) {
	log := uerrtest.CaptureDefault(t)
	surfaced := uerrtest.Capture(t)
	w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	be := newFakeBackend()
	server := newMailboxServer(be)

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
	landed := server.serverID("Projects", "")

	if err := ReclaimOrphaned(context.Background(), w); err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	result, err := NewDispatcher(accountID, be, w, reads).DispatchOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("dispatch the reclaimed intent: %v", err)
	}

	if n := outboxCount(t, w, id); n != 0 {
		t.Fatalf("intent %d still queued after its replay", id)
	}
	if len(result.Failures) != 0 {
		t.Fatalf("Failures = %+v, want none: the replay reconciled against the mailbox already there", result.Failures)
	}
	if n := server.mailboxes(); n != 1 {
		t.Errorf("the account holds %d mailboxes, want 1: the replay made a second Projects folder", n)
	}
	if n := server.createCalls(); n != 2 {
		t.Errorf("CreateMailbox calls = %d, want 2: the replay has to make the call to learn the mailbox is there", n)
	}
	// The adoption is a state transition and the only record it
	// happened, so it is logged, at the level a recovery takes. Nothing
	// reached the user, so nothing went through uerr's seam either.
	adoption := "outbox: adopted the mailbox a refused create already made"
	if !strings.Contains(log.String(), adoption) || !strings.Contains(log.String(), landed) {
		t.Errorf("log = %q, want an %q line naming %s", log.String(), adoption, landed)
	}
	if strings.Contains(log.String(), "level=ERROR") {
		t.Errorf("log = %q, want no error line: the intent completed", log.String())
	}
	if lines := uerrtest.Lines(t, surfaced); len(lines) != 0 {
		t.Errorf("uerr lines = %v, want none: an adoption surfaces nothing to the user", lines)
	}
}

// TestCreateMailboxReplayWindowInABatch holds the same window on the
// other path a create reaches the server by. An offline create-folder-
// then-move dispatches as one ApplyBatch request, the message updates
// naming the mailbox by the create's creation id, so a run killed
// after that request and before the transaction recording the new id
// leaves the same row to replay. The server refuses the replayed
// create, the dispatcher adopts the mailbox it names, and the moves
// are filed against it in a second request rather than into a second
// folder.
func TestCreateMailboxReplayWindowInABatch(t *testing.T) {
	w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
	msgID := seedMessage(t, w, accountID, src, "msg-1")

	be := newFakeBackend()
	server := newMailboxServer(be)
	filed := map[string]string{}
	applyCalls := 0
	var refileDest string
	be.MailSource.ApplyBatchFunc = func(ctx context.Context, muts []backend.Mutation) (backend.BatchResult, error) {
		applyCalls++
		if applyCalls == 2 {
			// The refile request: the create's own mutation is gone, so
			// this is the message update naming msg-1's destination by
			// resolved server id rather than the failed creation id's
			// back-reference. Read off the request itself, not filed,
			// since filed is pre-seeded with the killed run's own effect
			// and would pass whether or not this request repeats it.
			for _, mut := range muts {
				patch, ok := mut.Fields.(backend.MessagePatch)
				if ok && len(patch.MailboxIDs) > 0 {
					refileDest = patch.MailboxIDs[0]
				}
			}
		}
		return applyAgainst(ctx, server, filed, muts)
	}

	now := time.Now()
	createID, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, now)
	if err != nil {
		t.Fatalf("enqueue create: %v", err)
	}
	_, moveIDs, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{msgID}, 0, createID, be, false, now)
	if err != nil {
		t.Fatalf("enqueue move: %v", err)
	}

	// The killed run, reconstructed: its batch reached the server, so
	// the mailbox exists and the message is filed into it, and it died
	// before the transaction that writes the new id into the payload.
	strandDispatching(t, w, createID)
	for _, id := range moveIDs {
		strandDispatching(t, w, id)
	}
	landed, err := server.create(context.Background(), "Projects", "")
	if err != nil {
		t.Fatalf("create the mailbox the killed run created: %v", err)
	}
	filed["msg-1"] = landed

	if err := ReclaimOrphaned(context.Background(), w); err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	result, err := NewDispatcher(accountID, be, w, reads).DispatchOnce(context.Background(), now)
	if err != nil {
		t.Fatalf("dispatch the reclaimed batch: %v", err)
	}

	if len(result.Failures) != 0 {
		t.Fatalf("Failures = %+v, want none", result.Failures)
	}
	if len(result.Delivered) != 1+len(moveIDs) {
		t.Fatalf("Delivered = %+v, want the create and every move", result.Delivered)
	}
	if n := server.mailboxes(); n != 1 {
		t.Errorf("the account holds %d mailboxes, want 1: the replayed batch made a second Projects folder", n)
	}
	if applyCalls != 2 {
		t.Fatalf("ApplyBatch calls = %d, want 2: the refused create's moves are refiled in a second request", applyCalls)
	}
	if refileDest != landed {
		t.Errorf("the refile named dest = %q, want %q, the mailbox the batch was adopted onto", refileDest, landed)
	}
	for _, id := range append([]int64{createID}, moveIDs...) {
		if n := outboxCount(t, w, id); n != 0 {
			t.Errorf("intent %d is still in the outbox, want it delivered", id)
		}
	}
}

// applyAgainst is one ApplyBatch request against server: every mailbox
// create in it, then every message update, each filed into the mailbox
// its patch names. A patch naming a creation id the request did not
// resolve fails on its own, which is what a server does with a
// back-reference to a create it refused.
func applyAgainst(ctx context.Context, server *mailboxServer, filed map[string]string, muts []backend.Mutation) (backend.BatchResult, error) {
	res := backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}
	for _, mut := range muts {
		if mut.Kind != backend.ObjectKindMailbox {
			continue
		}
		box, _ := mut.Fields.(backend.MailboxCreate)
		id, err := server.create(ctx, box.Name, box.ParentID)
		if err != nil {
			res.Failed[mut.CreationID] = backend.Failure{Class: uerr.ClassServer, Cause: err}
			continue
		}
		res.Created[mut.CreationID] = id
	}
	for _, mut := range muts {
		if mut.Kind != backend.ObjectKindMessage {
			continue
		}
		patch, _ := mut.Fields.(backend.MessagePatch)
		dest := patch.MailboxIDs[0]
		if ref, isRef := strings.CutPrefix(dest, "#"); isRef {
			resolved, made := res.Created[ref]
			if !made {
				res.Failed[mut.ID] = backend.Failure{
					Class: uerr.ClassServer,
					Cause: errors.New("creation id " + ref + " never resolved"),
				}
				continue
			}
			dest = resolved
		}
		filed[mut.ID] = dest
	}
	return res, nil
}

// TestCreateMailboxAdoptsOnlyOneMatch covers the reconcile's refusal to
// guess. A server that refuses a name as a duplicate and then reports
// two mailboxes of that name under one parent has broken the very rule
// it enforced, and binding the intent to either of them files the
// user's mail into a folder nobody named. The intent fails instead,
// terminally and through the seam every user-visible failure reaches
// the log by, since every later attempt earns the same refusal and
// this is its only chance to be reported.
func TestCreateMailboxAdoptsOnlyOneMatch(t *testing.T) {
	surfaced := uerrtest.Capture(t)
	w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	be := newFakeBackend()
	be.MailSource.CreateMailboxFunc = func(context.Context, string, string) (string, error) {
		return "", fmt.Errorf("rejected: %w", backend.ErrMailboxNameExists)
	}
	be.MailSource.FindMailboxesFunc = func(context.Context, string, string) ([]string, error) {
		return []string{"mbx-1", "mbx-2"}, nil
	}

	id, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, time.Now())
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	result, err := NewDispatcher(accountID, be, w, reads).DispatchOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if len(result.Failures) != 1 {
		t.Fatalf("Failures = %+v, want exactly one", result.Failures)
	}
	f := result.Failures[0]
	if f.Retrying {
		t.Error("Retrying = true, want false: every later attempt earns the same refusal")
	}
	if !strings.Contains(f.Detail, "Projects") || !strings.Contains(f.Detail, "2") {
		t.Errorf("Detail = %q, want it naming the mailbox asked for and how many matched", f.Detail)
	}
	if n := outboxCount(t, w, id); n != 0 {
		t.Errorf("intent %d is still in the outbox, want it dropped after its report", id)
	}

	lines := uerrtest.Lines(t, surfaced)
	if len(lines) != 1 {
		t.Fatalf("uerr lines = %v, want exactly one: the row is dropped after this pass", lines)
	}
	if got, _ := lines[0]["cause"].(string); !strings.Contains(got, "Projects") {
		t.Errorf("the logged cause = %q, want it naming the mailbox that could not be resolved", got)
	}
}

// TestCreateMailboxKeepsRetryingAFailedLookup separates the lookup
// call failing from the lookup answering ambiguously. A connection
// that dropped during the reconcile says nothing about the account, so
// the intent waits for another pass rather than being given up on.
func TestCreateMailboxKeepsRetryingAFailedLookup(t *testing.T) {
	w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	be := newFakeBackend()
	be.MailSource.CreateMailboxFunc = func(context.Context, string, string) (string, error) {
		return "", fmt.Errorf("rejected: %w", backend.ErrMailboxNameExists)
	}
	be.MailSource.FindMailboxesFunc = func(context.Context, string, string) ([]string, error) {
		return nil, backend.Failure{Class: uerr.ClassConnection, Cause: errors.New("connection dropped")}
	}

	id, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, time.Now())
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	result, err := NewDispatcher(accountID, be, w, reads).DispatchOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if len(result.Failures) != 1 {
		t.Fatalf("Failures = %+v, want exactly one", result.Failures)
	}
	if f := result.Failures[0]; !f.Retrying || f.Class != uerr.ClassConnection {
		t.Errorf("Failure = %+v, want a retrying %v", f, uerr.ClassConnection)
	}
	if state, _ := outboxState(t, w, id); state != "queued" {
		t.Errorf("intent %d state = %s, want queued", id, state)
	}
}

// TestApplyBatchAdoptsOnlyOneMatch is
// TestCreateMailboxAdoptsOnlyOneMatch for the batch path: a create
// refused inside an ApplyBatch call that also carries the moves
// destined for it. An ambiguous reconcile is terminal for the whole
// batch, the create and every move alike, since a move whose
// destination never resolved has nothing left to file into.
func TestApplyBatchAdoptsOnlyOneMatch(t *testing.T) {
	w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
	filed := []int64{
		seedMessage(t, w, accountID, src, "msg-1"),
		seedMessage(t, w, accountID, src, "msg-2"),
	}

	be := newFakeBackend()
	be.MailSource.ApplyBatchFunc = func(_ context.Context, muts []backend.Mutation) (backend.BatchResult, error) {
		res := backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}
		for _, mut := range muts {
			if mut.Kind == backend.ObjectKindMailbox {
				res.Failed[mut.CreationID] = fmt.Errorf("rejected: %w", backend.ErrMailboxNameExists)
			}
		}
		return res, nil
	}
	be.MailSource.FindMailboxesFunc = func(context.Context, string, string) ([]string, error) {
		return []string{"mbx-1", "mbx-2"}, nil
	}

	now := time.Now()
	createID, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, now)
	if err != nil {
		t.Fatalf("enqueue create: %v", err)
	}
	var moveIDs []int64
	for _, msgID := range filed {
		_, ids, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{msgID}, 0, createID, be, false, now)
		if err != nil {
			t.Fatalf("enqueue dependent move: %v", err)
		}
		moveIDs = append(moveIDs, ids...)
	}

	result, err := NewDispatcher(accountID, be, w, reads).DispatchOnce(context.Background(), now)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	claimed := append([]int64{createID}, moveIDs...)
	if len(result.Failures) != len(claimed) {
		t.Fatalf("Failures = %+v, want one per intent in the batch", result.Failures)
	}
	for _, f := range result.Failures {
		if f.Retrying {
			t.Errorf("intent %d Retrying = true, want false: an ambiguous match repeats on every attempt", f.IntentID)
		}
	}
	for _, id := range claimed {
		if n := outboxCount(t, w, id); n != 0 {
			t.Errorf("intent %d is still in the outbox, want it dropped after its report", id)
		}
	}
}

// TestApplyBatchKeepsRetryingAFailedLookup is
// TestCreateMailboxKeepsRetryingAFailedLookup for the batch path: a
// connection dropped during the reconcile says nothing about the
// account, so the create and every move destined for it wait for
// another pass rather than being given up on.
func TestApplyBatchKeepsRetryingAFailedLookup(t *testing.T) {
	w, reads := storetest.OpenStore(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
	filed := []int64{
		seedMessage(t, w, accountID, src, "msg-1"),
		seedMessage(t, w, accountID, src, "msg-2"),
	}

	be := newFakeBackend()
	be.MailSource.ApplyBatchFunc = func(_ context.Context, muts []backend.Mutation) (backend.BatchResult, error) {
		res := backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}
		for _, mut := range muts {
			if mut.Kind == backend.ObjectKindMailbox {
				res.Failed[mut.CreationID] = fmt.Errorf("rejected: %w", backend.ErrMailboxNameExists)
			}
		}
		return res, nil
	}
	be.MailSource.FindMailboxesFunc = func(context.Context, string, string) ([]string, error) {
		return nil, backend.Failure{Class: uerr.ClassConnection, Cause: errors.New("connection dropped")}
	}

	now := time.Now()
	createID, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, now)
	if err != nil {
		t.Fatalf("enqueue create: %v", err)
	}
	var moveIDs []int64
	for _, msgID := range filed {
		_, ids, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{msgID}, 0, createID, be, false, now)
		if err != nil {
			t.Fatalf("enqueue dependent move: %v", err)
		}
		moveIDs = append(moveIDs, ids...)
	}

	result, err := NewDispatcher(accountID, be, w, reads).DispatchOnce(context.Background(), now)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	claimed := append([]int64{createID}, moveIDs...)
	if len(result.Failures) != len(claimed) {
		t.Fatalf("Failures = %+v, want one per intent in the batch", result.Failures)
	}
	for _, f := range result.Failures {
		if !f.Retrying || f.Class != uerr.ClassConnection {
			t.Errorf("intent %d Failure = %+v, want a retrying %v", f.IntentID, f, uerr.ClassConnection)
		}
	}
	for _, id := range claimed {
		if state, _ := outboxState(t, w, id); state != "queued" {
			t.Errorf("intent %d state = %s, want queued", id, state)
		}
	}
}
