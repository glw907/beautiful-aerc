package outbox

import (
	"context"
	"database/sql"
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

// TestFailureClasses covers SY-4's closed failure enum end to end
// through the dispatcher: each class's retry disposition (every class
// but not-found stays queued for another attempt) and throttled's
// distinct surfacing as a warn state, never an error toast.
func TestFailureClasses(t *testing.T) {
	tests := []struct {
		name         string
		class        uerr.Class
		topLevel     bool // a whole-call failure (simulating a dropped connection) rather than a per-mutation one
		wantRetrying bool
		wantWarn     bool
	}{
		{name: "auth", class: uerr.ClassAuth, wantRetrying: true},
		{name: "auth refresh failed", class: uerr.ClassAuthRefreshFailed, wantRetrying: true},
		{name: "not found", class: uerr.ClassNotFound, wantRetrying: false},
		{name: "connection", class: uerr.ClassConnection, topLevel: true, wantRetrying: true},
		{name: "server", class: uerr.ClassServer, wantRetrying: true},
		{name: "throttled", class: uerr.ClassThrottled, wantRetrying: true, wantWarn: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := storetest.OpenWriter(t, store.DefaultWriterConfig())
			accountID := seedAccount(t, w)
			src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
			dest := seedMailbox(t, w, accountID, "Archive", "mbx-dest")
			msgID := seedMessage(t, w, accountID, src, "msg-1")

			cause := errors.New("boom")
			be := newFakeBackend()
			be.MailSource.ApplyBatchFunc = func(_ context.Context, muts []backend.Mutation) (backend.BatchResult, error) {
				if tt.topLevel {
					return backend.BatchResult{}, backend.Failure{Class: tt.class, Cause: cause}
				}
				failed := map[string]error{}
				for _, m := range muts {
					failed[m.ID] = backend.Failure{Class: tt.class, Cause: cause}
				}
				return backend.BatchResult{Created: map[string]string{}, Failed: failed}, nil
			}
			dispatcher := NewDispatcher(accountID, be, w)

			_, ids, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{msgID}, dest, 0, be, false, time.Now())
			if err != nil {
				t.Fatalf("enqueue: %v", err)
			}
			result, err := dispatcher.DispatchOnce(context.Background(), time.Now())
			if err != nil {
				t.Fatalf("dispatch: %v", err)
			}

			if len(result.Failures) != 1 {
				t.Fatalf("Failures = %+v, want exactly one", result.Failures)
			}
			f := result.Failures[0]
			if f.Class != tt.class {
				t.Errorf("Class = %v, want %v", f.Class, tt.class)
			}
			if f.Detail != cause.Error() {
				t.Errorf("Detail = %q, want %q", f.Detail, cause.Error())
			}
			if f.Retrying != tt.wantRetrying {
				t.Errorf("Retrying = %v, want %v", f.Retrying, tt.wantRetrying)
			}
			if f.Warn != tt.wantWarn {
				t.Errorf("Warn = %v, want %v", f.Warn, tt.wantWarn)
			}

			wantCount := 0
			if tt.wantRetrying {
				wantCount = 1
			}
			if n := outboxCount(t, w, ids[0]); n != wantCount {
				t.Errorf("outbox row present = %v, want present = %v", n == 1, wantCount == 1)
			}
			if tt.wantRetrying {
				if state, attempts := outboxState(t, w, ids[0]); state != "queued" || attempts != 1 {
					t.Errorf("state = %s attempts = %d, want queued/1", state, attempts)
				}
			}
		})
	}
}

// TestNoIntentStrandsInDispatching covers DispatchOnce's
// postcondition, whatever the pass decides for each claimed row: no
// row is left in dispatching once it returns. selectEligible reads
// queued rows only, so a row stranded in dispatching is invisible to
// every later pass, and the user's action is lost without a trace:
// never dispatched, never reverted, never retried, never surfaced.
func TestNoIntentStrandsInDispatching(t *testing.T) {
	t.Run("every intent delivered", func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)
		src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
		dest := seedMailbox(t, w, accountID, "Archive", "mbx-dest")
		msgID := seedMessage(t, w, accountID, src, "msg-0")

		be := newFakeBackend()
		be.MailSource.ApplyBatchFunc = func(_ context.Context, _ []backend.Mutation) (backend.BatchResult, error) {
			return backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}, nil
		}
		if _, _, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{msgID}, dest, 0, be, false, time.Now()); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		if _, err := NewDispatcher(accountID, be, w).DispatchOnce(context.Background(), time.Now()); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if n := dispatchingCount(t, w, accountID); n != 0 {
			t.Errorf("rows still dispatching = %d, want 0", n)
		}
	})

	t.Run("retriable failure requeues", func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)
		src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
		dest := seedMailbox(t, w, accountID, "Archive", "mbx-dest")
		msgID := seedMessage(t, w, accountID, src, "msg-0")

		be := newFakeBackend()
		be.MailSource.ApplyBatchFunc = func(_ context.Context, muts []backend.Mutation) (backend.BatchResult, error) {
			failed := map[string]error{}
			for _, m := range muts {
				failed[m.ID] = backend.Failure{Class: uerr.ClassServer, Cause: errors.New("boom")}
			}
			return backend.BatchResult{Created: map[string]string{}, Failed: failed}, nil
		}
		if _, _, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{msgID}, dest, 0, be, false, time.Now()); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		if _, err := NewDispatcher(accountID, be, w).DispatchOnce(context.Background(), time.Now()); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if n := dispatchingCount(t, w, accountID); n != 0 {
			t.Errorf("rows still dispatching = %d, want 0", n)
		}
	})

	// A connection failure stops the pass while a create's dependent
	// moves are still pending in its batch. Those moves were claimed
	// but never attempted, so the pass owes them a revert like every
	// other row it claimed and never reached.
	t.Run("connection failure with a pending batch", func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)
		src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
		dest := seedMailbox(t, w, accountID, "Archive", "mbx-dest")
		doomed := seedMessage(t, w, accountID, src, "msg-0")
		filed := []int64{
			seedMessage(t, w, accountID, src, "msg-1"),
			seedMessage(t, w, accountID, src, "msg-2"),
		}

		be := newFakeBackend()
		applyCalls := 0
		be.MailSource.ApplyBatchFunc = func(_ context.Context, _ []backend.Mutation) (backend.BatchResult, error) {
			applyCalls++
			if applyCalls == 1 {
				return backend.BatchResult{}, backend.Failure{Class: uerr.ClassConnection, Cause: errors.New("connection dropped")}
			}
			return backend.BatchResult{Created: map[string]string{}, Failed: map[string]error{}}, nil
		}

		now := time.Now()
		if _, _, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{doomed}, dest, 0, be, false, now); err != nil {
			t.Fatalf("enqueue the failing move: %v", err)
		}
		createID, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, now)
		if err != nil {
			t.Fatalf("enqueue create: %v", err)
		}
		var batchedMoves []int64
		for _, msgID := range filed {
			_, ids, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{msgID}, 0, createID, be, false, now)
			if err != nil {
				t.Fatalf("enqueue dependent move: %v", err)
			}
			batchedMoves = append(batchedMoves, ids...)
		}

		if _, err := NewDispatcher(accountID, be, w).DispatchOnce(context.Background(), now); err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if applyCalls != 1 {
			t.Fatalf("ApplyBatch calls = %d, want 1 (the connection failure stops the pass)", applyCalls)
		}
		if n := dispatchingCount(t, w, accountID); n != 0 {
			t.Errorf("rows still dispatching = %d, want 0", n)
		}
		for _, id := range batchedMoves {
			if state, attempts := outboxState(t, w, id); state != "queued" || attempts != 0 {
				t.Errorf("batched move %d state = %s attempts = %d, want queued/0", id, state, attempts)
			}
		}
	})
}

// TestFinalizeFailureRequeuesClaimedIntents covers the finalize
// transaction failing outright. Every claimed row is in dispatching,
// where selectEligible never looks again, and ReclaimOrphaned runs
// once at startup, so without a recovery inside the pass a single
// failed commit costs the queue every claimed intent for the rest of
// the process run.
func TestFinalizeFailureRequeuesClaimedIntents(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	mailboxID := seedMailbox(t, w, accountID, "Old", "mbx-1")

	renames := 0
	be := newFakeBackend()
	be.MailSource.RenameMailboxFunc = func(_ context.Context, _, _ string) error {
		renames++
		return nil
	}

	id, _, err := EnqueueRenameMailbox(context.Background(), w, accountID, mailboxID, "Family", time.Now())
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	blockDeletes(t, w)

	dispatcher := NewDispatcher(accountID, be, w)
	result, err := dispatcher.DispatchOnce(context.Background(), time.Now())
	if err == nil {
		t.Fatal("dispatch = nil, want the finalize failure")
	}
	if len(result.Delivered) != 0 || len(result.Failures) != 0 {
		t.Errorf("result = %+v, want it empty: none of its writes landed", result)
	}
	if state, _ := outboxState(t, w, id); state != "queued" {
		t.Fatalf("intent %d state = %s, want queued: it is unreachable in dispatching", id, state)
	}

	unblockDeletes(t, w)
	if _, err := dispatcher.DispatchOnce(context.Background(), time.Now()); err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	if n := outboxCount(t, w, id); n != 0 {
		t.Errorf("intent %d survived its second dispatch, want it delivered", id)
	}
	if renames != 2 {
		t.Errorf("RenameMailbox calls = %d, want 2: the requeued intent is dispatched again", renames)
	}
}

// TestFinalizeRecoveryOutlivesACancelledContext covers the way a
// per-pass timeout normally expires: during the backend calls, so the
// finalize transaction fails because its context is already done. The
// recovery that returns each claimed row to queued must not carry that
// same context, or it fails for the identical reason and the rows
// strand in dispatching until the next process start, which is exactly
// the case the recovery exists for.
func TestFinalizeRecoveryOutlivesACancelledContext(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	mailboxID := seedMailbox(t, w, accountID, "Old", "mbx-1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	be := newFakeBackend()
	be.MailSource.RenameMailboxFunc = func(context.Context, string, string) error {
		cancel()
		// The writer is busy for the whole finalize step, so the
		// cancelled context loses the lane race deterministically
		// rather than half the time.
		occupyWriter(t, w, 100*time.Millisecond)
		return nil
	}

	id, _, err := EnqueueRenameMailbox(context.Background(), w, accountID, mailboxID, "Family", time.Now())
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if _, err := NewDispatcher(accountID, be, w).DispatchOnce(ctx, time.Now()); err == nil {
		t.Fatal("DispatchOnce = nil, want the cancelled finalize reported")
	}
	if state, _ := outboxState(t, w, id); state != "queued" {
		t.Errorf("intent %d state = %s, want queued: the recovery died with the context it was recovering from", id, state)
	}
}

// TestFinalizeRecoveryRequeuesACreateWhoseMailboxLanded covers what
// the recovery owes a create the server has already acted on. It used
// to leave that row in dispatching, because returning it to queued
// made the account a second folder of the same name on the very next
// pass. Nothing is owed to it now: the replay's create is refused for
// the mailbox already there and the dispatcher adopts it, so leaving
// the row where selectEligible cannot see it would cost the intent for
// the rest of the run and buy nothing.
func TestFinalizeRecoveryRequeuesACreateWhoseMailboxLanded(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	renamed := seedMailbox(t, w, accountID, "Old", "mbx-old")

	be := newFakeBackend()
	server := newMailboxServer(be)
	be.MailSource.RenameMailboxFunc = func(context.Context, string, string) error { return nil }

	now := time.Now()
	createID, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, now)
	if err != nil {
		t.Fatalf("enqueue the create: %v", err)
	}
	if _, _, err := EnqueueRenameMailbox(context.Background(), w, accountID, renamed, "Family", now); err != nil {
		t.Fatalf("enqueue the rename: %v", err)
	}

	// The create's payload write fails, so its own row is headed for a
	// requeue. The delivered rename's delete is what fails the finalize
	// transaction and takes that requeue down with it.
	blockPayloadWrites(t, w)
	blockDeletes(t, w)

	dispatcher := NewDispatcher(accountID, be, w)
	if _, err := dispatcher.DispatchOnce(context.Background(), now); err == nil {
		t.Fatal("DispatchOnce = nil, want the finalize failure")
	}
	if state, _ := outboxState(t, w, createID); state != "queued" {
		t.Fatalf("intent %d state = %s, want queued: its replay reconciles rather than duplicating", createID, state)
	}
	if n := server.mailboxes(); n != 1 {
		t.Fatalf("the account holds %d mailboxes after the first pass, want 1", n)
	}

	unblockDeletes(t, w)
	unblockPayloadWrites(t, w)
	// Past the backoff the requeue wrote, so the row is eligible again.
	if _, err := dispatcher.DispatchOnce(context.Background(), now.Add(time.Minute)); err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	if n := outboxCount(t, w, createID); n != 0 {
		t.Errorf("intent %d is still in the outbox, want it delivered", createID)
	}
	if n := server.mailboxes(); n != 1 {
		t.Errorf("the account holds %d mailboxes, want 1: the replay made a second Projects folder", n)
	}
}

// TestFinalizeRecoveryReplaysTheRequeue covers what the recovery owes
// a row the failed finalize was going to requeue, column by column.
// Returned to queued with its expired next_attempt_at and its attempt
// count untouched, it retries on the next pass with no backoff at all,
// so a finalize failure that persists turns every pass into another
// immediate attempt against the live backend; returned without its
// failure class, it earns a fresh uerr.New on every one of them.
// failure_detail is the one column the replay leaves alone, since the
// recovery runs only after a transaction has already failed and a
// detail is the one value in the statement with no bound on its size.
func TestFinalizeRecoveryReplaysTheRequeue(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	deliveredID := seedMailbox(t, w, accountID, "Old", "mbx-1")
	failingID := seedMailbox(t, w, accountID, "Older", "mbx-2")

	be := newFakeBackend()
	be.MailSource.RenameMailboxFunc = func(_ context.Context, id, _ string) error {
		if id == "mbx-2" {
			return backend.Failure{Class: uerr.ClassServer, Cause: errors.New("boom")}
		}
		return nil
	}

	now := time.Now()
	if _, _, err := EnqueueRenameMailbox(context.Background(), w, accountID, deliveredID, "Family", now); err != nil {
		t.Fatalf("enqueue the delivered rename: %v", err)
	}
	failing, _, err := EnqueueRenameMailbox(context.Background(), w, accountID, failingID, "Archive", now)
	if err != nil {
		t.Fatalf("enqueue the failing rename: %v", err)
	}
	// The delivered rename's delete is what fails the finalize
	// transaction, taking the failing rename's requeue down with it.
	blockDeletes(t, w)

	if _, err := NewDispatcher(accountID, be, w).DispatchOnce(context.Background(), now); err == nil {
		t.Fatal("DispatchOnce = nil, want the finalize failure")
	}

	state, attempts := outboxState(t, w, failing)
	if state != "queued" || attempts != 1 {
		t.Errorf("intent %d state = %s attempts = %d, want queued/1", failing, state, attempts)
	}
	next := storetest.ScanValue[int64](t, w, `SELECT next_attempt_at FROM outbox WHERE id = ?`, failing)
	if want := now.Add(backoff(0)).Unix(); next != want {
		t.Errorf("next_attempt_at = %d, want %d: the recovered row retries without the backoff its requeue was going to write", next, want)
	}
	class := storetest.ScanValue[sql.NullString](t, w, `SELECT failure_class FROM outbox WHERE id = ?`, failing)
	if class.String != uerr.ClassServer.String() {
		t.Errorf("failure_class = %q, want %q: without it every later pass logs the same failure again", class.String, uerr.ClassServer.String())
	}
	detail := storetest.ScanValue[sql.NullString](t, w, `SELECT failure_detail FROM outbox WHERE id = ?`, failing)
	if detail.Valid {
		t.Errorf("failure_detail = %q, want it untouched: the recovery writes no unbounded value", detail.String)
	}
}

// TestFinalizeRecoveryFailureNamesEveryStrandedIntent covers the
// recovery failing too. It is one transaction over every claimed row,
// so a store failure inside it costs all of them their revert: each
// stays in dispatching, where selectEligible never looks again and only
// the next start's sweep can reach it. DispatchOnce owes its caller
// that error, and the log owes the operator the ids, since nothing else
// records which intents the run lost.
func TestFinalizeRecoveryFailureNamesEveryStrandedIntent(t *testing.T) {
	log := uerrtest.CaptureDefault(t)
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	deliveredID := seedMailbox(t, w, accountID, "Old", "mbx-1")
	failingID := seedMailbox(t, w, accountID, "Older", "mbx-2")

	be := newFakeBackend()
	be.MailSource.RenameMailboxFunc = func(_ context.Context, id, _ string) error {
		if id == "mbx-2" {
			return backend.Failure{Class: uerr.ClassServer, Cause: errors.New("boom")}
		}
		return nil
	}

	now := time.Now()
	delivered, _, err := EnqueueRenameMailbox(context.Background(), w, accountID, deliveredID, "Family", now)
	if err != nil {
		t.Fatalf("enqueue the delivered rename: %v", err)
	}
	failing, _, err := EnqueueRenameMailbox(context.Background(), w, accountID, failingID, "Archive", now)
	if err != nil {
		t.Fatalf("enqueue the failing rename: %v", err)
	}
	// The delivered rename's delete fails the finalize transaction, and
	// the failing rename's attempt-count bump then fails the recovery
	// that was going to return both rows to queued.
	blockDeletes(t, w)
	blockRequeueBackoff(t, w)

	if _, err := NewDispatcher(accountID, be, w).DispatchOnce(context.Background(), now); err == nil {
		t.Fatal("DispatchOnce = nil, want the finalize failure joined with the failed recovery")
	}

	if n := dispatchingCount(t, w, accountID); n != 2 {
		t.Errorf("rows still dispatching = %d, want 2: the recovery is all or nothing over every claimed row", n)
	}
	want := fmt.Sprintf("[%d %d]", delivered, failing)
	if !strings.Contains(log.String(), "outbox: claimed intents stranded in dispatching") || !strings.Contains(log.String(), want) {
		t.Errorf("log = %q, want a stranded-intent line naming %s", log.String(), want)
	}
}

// blockDeletes makes every outbox delete fail, the shape a finalize
// transaction takes when the store refuses its write.
func blockDeletes(t *testing.T, w *store.Writer) {
	t.Helper()
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TRIGGER block_outbox_delete BEFORE DELETE ON outbox
			BEGIN SELECT RAISE(ABORT, 'simulated finalize failure'); END`)
		return err
	})
	if err != nil {
		t.Fatalf("install the delete-blocking trigger: %v", err)
	}
}

// unblockDeletes drops blockDeletes's trigger.
func unblockDeletes(t *testing.T, w *store.Writer) {
	t.Helper()
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`DROP TRIGGER block_outbox_delete`)
		return err
	})
	if err != nil {
		t.Fatalf("drop the delete-blocking trigger: %v", err)
	}
}

// blockRequeueBackoff makes the attempt-count bump a requeue writes
// fail, leaving revertRow's bare state write alone: the shape a store
// failure specific to one row of the finalize recovery takes.
func blockRequeueBackoff(t *testing.T, w *store.Writer) {
	t.Helper()
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TRIGGER block_outbox_requeue BEFORE UPDATE OF attempt_count ON outbox
			BEGIN SELECT RAISE(ABORT, 'simulated requeue failure'); END`)
		return err
	})
	if err != nil {
		t.Fatalf("install the requeue-blocking trigger: %v", err)
	}
}

// TestUnknownKindIsReportedAndDropped covers an intent whose kind this
// build cannot dispatch. Nothing can ever deliver it, so a retriable
// classification retries it every 30 seconds for the life of the
// store, logged once and then silent.
func TestUnknownKindIsReportedAndDropped(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	var id int64
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		var txErr error
		id, txErr = insertRow(tx, accountID, Kind("teleport-message"), []byte(`{}`), "grp", 0, time.Now(), time.Now())
		return txErr
	})
	if err != nil {
		t.Fatalf("insert the unknown-kind row: %v", err)
	}

	result, err := NewDispatcher(accountID, newFakeBackend(), w).DispatchOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("Failures = %+v, want exactly one", result.Failures)
	}
	f := result.Failures[0]
	if f.Retrying {
		t.Error("Retrying = true, want false: no later pass can dispatch a kind this build has no case for")
	}
	if f.Class == uerr.ClassServer {
		t.Error("Class = server, want a local class: no server was involved")
	}
	if n := outboxCount(t, w, id); n != 0 {
		t.Errorf("intent %d is still in the outbox, want it dropped after its report", id)
	}
}

// TestUnknownKindSurvivesAFinalizeFailure covers what a terminal
// disposition means and what it does not. It marks a failure no later
// attempt can answer differently, and it says nothing about where the
// row belongs when the finalize transaction itself fails: every
// claimed row is owed the revert.
// Left in dispatching this one is invisible to selectEligible for the
// rest of the run, and no later pass can report it.
func TestUnknownKindSurvivesAFinalizeFailure(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	var id int64
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		var txErr error
		id, txErr = insertRow(tx, accountID, Kind("teleport-message"), []byte(`{}`), "grp", 0, time.Now(), time.Now())
		return txErr
	})
	if err != nil {
		t.Fatalf("insert the unknown-kind row: %v", err)
	}
	blockDeletes(t, w)

	dispatcher := NewDispatcher(accountID, newFakeBackend(), w)
	if _, err := dispatcher.DispatchOnce(context.Background(), time.Now()); err == nil {
		t.Fatal("DispatchOnce = nil, want the finalize failure")
	}
	if state, _ := outboxState(t, w, id); state != "queued" {
		t.Fatalf("intent %d state = %s, want queued: no server action to protect from a replay", id, state)
	}

	unblockDeletes(t, w)
	result, err := dispatcher.DispatchOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("Failures = %+v, want exactly one: the second pass reports what the first could not", result.Failures)
	}
	if n := outboxCount(t, w, id); n != 0 {
		t.Errorf("intent %d is still in the outbox, want it dropped after its report", id)
	}
}

// TestCreateMailboxPersistFailureRetriesOntoTheSameMailbox covers the
// window between a mailbox the server has already made and the
// transaction that records its id, entered by a store-write failure
// rather than by a crash. The store write is what failed, so the
// failure is a local one, and it is worth retrying: the retry's create
// is refused for the mailbox already there and the dispatcher adopts
// it, so the intent completes against the folder the first attempt
// made instead of being given up on beside it.
func TestCreateMailboxPersistFailureRetriesOntoTheSameMailbox(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	be := newFakeBackend()
	server := newMailboxServer(be)

	id, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, time.Now())
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	blockPayloadWrites(t, w)

	dispatcher := NewDispatcher(accountID, be, w)
	result, err := dispatcher.DispatchOnce(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("Failures = %+v, want exactly one", result.Failures)
	}
	f := result.Failures[0]
	if f.Class != uerr.ClassStoreLocal {
		t.Errorf("Class = %v, want %v: the store write failed, not the server", f.Class, uerr.ClassStoreLocal)
	}
	if !f.Retrying {
		t.Error("Retrying = false, want true: the retry reconciles against the mailbox already made")
	}
	if state, _ := outboxState(t, w, id); state != "queued" {
		t.Fatalf("intent %d state = %s, want queued", id, state)
	}

	unblockPayloadWrites(t, w)
	if _, err := dispatcher.DispatchOnce(context.Background(), time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	if n := outboxCount(t, w, id); n != 0 {
		t.Errorf("intent %d is still in the outbox, want it delivered", id)
	}
	if n := server.mailboxes(); n != 1 {
		t.Errorf("the account holds %d mailboxes, want 1: the retry made a second Projects folder", n)
	}
	if n := server.createCalls(); n != 2 {
		t.Errorf("CreateMailbox calls = %d, want 2: the retry has to make the call to learn the mailbox is there", n)
	}
}

// blockPayloadWrites makes the payload update a create persists its
// resolved server id through fail, leaving every other outbox write
// alone.
func blockPayloadWrites(t *testing.T, w *store.Writer) {
	t.Helper()
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			CREATE TRIGGER block_outbox_payload BEFORE UPDATE ON outbox
			WHEN NEW.payload IS NOT OLD.payload
			BEGIN SELECT RAISE(ABORT, 'simulated payload write failure'); END`)
		return err
	})
	if err != nil {
		t.Fatalf("install the payload-blocking trigger: %v", err)
	}
}

// unblockPayloadWrites drops blockPayloadWrites's trigger.
func unblockPayloadWrites(t *testing.T, w *store.Writer) {
	t.Helper()
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`DROP TRIGGER block_outbox_payload`)
		return err
	})
	if err != nil {
		t.Fatalf("drop the payload-blocking trigger: %v", err)
	}
}

// TestEachClaimedIntentGetsOneFinalizeWrite pins the other half of
// that postcondition: one finalize write per claimed row, never two.
// A row whose create's batch already decided it, and which the pass's
// stopped branch then decides a second time, is written twice, and the
// combined effect is correct only because revertRow touches state
// alone and lands after requeueRow. The end state hides that, so this
// counts the writes themselves.
func TestEachClaimedIntentGetsOneFinalizeWrite(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	src := seedMailbox(t, w, accountID, "Inbox", "mbx-src")
	filed := []int64{
		seedMessage(t, w, accountID, src, "msg-1"),
		seedMessage(t, w, accountID, src, "msg-2"),
	}

	be := newFakeBackend()
	be.MailSource.ApplyBatchFunc = func(_ context.Context, _ []backend.Mutation) (backend.BatchResult, error) {
		return backend.BatchResult{}, backend.Failure{Class: uerr.ClassConnection, Cause: errors.New("connection dropped")}
	}

	now := time.Now()
	createID, _, err := EnqueueCreateMailbox(context.Background(), w, accountID, "Projects", 0, 0, now)
	if err != nil {
		t.Fatalf("enqueue create: %v", err)
	}
	claimed := []int64{createID}
	for _, msgID := range filed {
		_, ids, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{msgID}, 0, createID, be, false, now)
		if err != nil {
			t.Fatalf("enqueue dependent move: %v", err)
		}
		claimed = append(claimed, ids...)
	}

	recordFinalizeWrites(t, w)
	if _, err := NewDispatcher(accountID, be, w).DispatchOnce(context.Background(), now); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	for _, id := range claimed {
		if n := finalizeWriteCount(t, w, id); n != 1 {
			t.Errorf("finalize writes for intent %d = %d, want 1", id, n)
		}
	}
}

// recordFinalizeWrites installs triggers appending one row to a
// scratch table for every finalize write DispatchOnce makes: a delete,
// or a state write returning a row to queued. The payload guard keeps
// a back-reference patch, which rewrites a still-queued row's payload
// mid-pass, out of the count.
func recordFinalizeWrites(t *testing.T, w *store.Writer) {
	t.Helper()
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE finalize_write (intent_id INTEGER NOT NULL)`); err != nil {
			return err
		}
		if _, err := tx.Exec(`
			CREATE TRIGGER record_finalize_state AFTER UPDATE ON outbox
			WHEN NEW.state = 'queued' AND NEW.payload IS OLD.payload
			BEGIN INSERT INTO finalize_write (intent_id) VALUES (NEW.id); END`); err != nil {
			return err
		}
		_, err := tx.Exec(`
			CREATE TRIGGER record_finalize_delete AFTER DELETE ON outbox
			BEGIN INSERT INTO finalize_write (intent_id) VALUES (OLD.id); END`)
		return err
	})
	if err != nil {
		t.Fatalf("install finalize-write triggers: %v", err)
	}
}

// finalizeWriteCount returns how many finalize writes the triggers
// recorded for id.
func finalizeWriteCount(t *testing.T, w *store.Writer, id int64) int {
	t.Helper()
	var n int
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT COUNT(*) FROM finalize_write WHERE intent_id = ?`, id).Scan(&n)
	})
	if err != nil {
		t.Fatalf("count finalize writes for %d: %v", id, err)
	}
	return n
}

// TestShouldLogFailure covers report's log-dedup gate (ADR-0013
// revision 2): a first failure and a class change both log, a
// repeated failure of the same class does not, and an unretriable
// failure always logs, since its row is deleted once this pass
// finalizes and this is its only chance to reach the log.
func TestShouldLogFailure(t *testing.T) {
	tests := []struct {
		name      string
		lastClass string
		class     uerr.Class
		retry     bool
		want      bool
	}{
		{name: "first failure logs", lastClass: "", class: uerr.ClassConnection, retry: true, want: true},
		{name: "repeated same class does not log", lastClass: uerr.ClassConnection.String(), class: uerr.ClassConnection, retry: true, want: false},
		{name: "class change logs", lastClass: uerr.ClassConnection.String(), class: uerr.ClassAuth, retry: true, want: true},
		{name: "unretriable always logs", lastClass: uerr.ClassNotFound.String(), class: uerr.ClassNotFound, retry: false, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldLogFailure(tt.lastClass, tt.class, tt.retry); got != tt.want {
				t.Errorf("shouldLogFailure(%q, %v, %v) = %v, want %v", tt.lastClass, tt.class, tt.retry, got, tt.want)
			}
		})
	}
}

// TestReportUnhandledDispositionLogsAndTakesTheTerminalVerb covers
// the disposition switch's default case, reachable only by a mistake
// this build has no name for. It has to lose loudly rather than
// silently: a fourth disposition falling through to the retry branch
// would requeue a row forever, and falling through to a bare
// finalizeAction{} would delete it under a class and detail that say
// nothing happened.
func TestReportUnhandledDispositionLogsAndTakesTheTerminalVerb(t *testing.T) {
	log := uerrtest.CaptureDefault(t)
	d := &Dispatcher{}
	c := claimed{row: row{id: 42}}
	o := outcome{c: c, disposition: disposition(99), class: uerr.ClassServer, detail: "boom"}

	var result Result
	action := d.report(&result, o)

	if action.verb != finalizeDelete {
		t.Errorf("verb = %v, want finalizeDelete: an unnamed disposition must not requeue forever", action.verb)
	}
	if len(result.Failures) != 1 || result.Failures[0].Retrying {
		t.Errorf("Failures = %+v, want one non-retrying failure", result.Failures)
	}
	if !strings.Contains(log.String(), "outbox: unhandled disposition") || !strings.Contains(log.String(), "intent=42") {
		t.Errorf("log = %q, want a line naming the unhandled disposition and intent 42", log.String())
	}
}
