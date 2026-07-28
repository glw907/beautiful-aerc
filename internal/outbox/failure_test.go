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

			_, ids, err := EnqueueMoveMessagesBulk(context.Background(), w, accountID, []int64{msgID}, dest, 0, 10, false, time.Now())
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
