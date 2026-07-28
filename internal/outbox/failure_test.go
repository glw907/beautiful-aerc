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
					return backend.BatchResult{}, backend.MutationFailure{Class: tt.class, Cause: cause}
				}
				failed := map[string]error{}
				for _, m := range muts {
					failed[m.ID] = backend.MutationFailure{Class: tt.class, Cause: cause}
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
				failed[m.ID] = backend.MutationFailure{Class: uerr.ClassServer, Cause: errors.New("boom")}
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
				return backend.BatchResult{}, backend.MutationFailure{Class: uerr.ClassConnection, Cause: errors.New("connection dropped")}
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
