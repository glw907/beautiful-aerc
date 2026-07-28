package sync

import (
	"context"
	"testing"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// TestSelfEchoSuppressed proves a dispatched mutation does not
// round-trip into a re-apply: once NoteDispatchedState has recorded a
// token as one the worker's own dispatch produced, a push-triggered
// cycle that resolves to that same token still advances the
// watermark but never writes the change set it carries. A later,
// distinct state change applies normally.
func TestSelfEchoSuppressed(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{
			NewToken: "dispatched-1",
			Created:  []backend.Record{{ID: "m1", Fields: map[string]any{"subject": "hello"}}},
		}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	worker.NoteDispatchedState(backend.ObjectKindMessage, "dispatched-1")

	if err := worker.SyncKind(context.Background(), backend.ObjectKindMessage); err != nil {
		t.Fatalf("SyncKind: %v", err)
	}

	if got := countRows(t, w, "message", accountID); got != 0 {
		t.Fatalf("message rows = %d, want 0: a self-produced state token must not re-apply its change set", got)
	}

	wm, err := loadWatermark(context.Background(), w, accountID, backend.ObjectKindMessage)
	if err != nil {
		t.Fatalf("loadWatermark: %v", err)
	}
	if wm.ServerStateToken != "dispatched-1" {
		t.Fatalf("watermark token = %q, want %q: the watermark advances even when the apply is suppressed", wm.ServerStateToken, "dispatched-1")
	}

	be.MailSource.ChangesFunc = func(_ context.Context, _ backend.ObjectKind, token string, _ int) (backend.ChangeSet, error) {
		if token != "dispatched-1" {
			t.Fatalf("Changes(token = %q), want %q", token, "dispatched-1")
		}
		return backend.ChangeSet{
			NewToken: "state-2",
			Created:  []backend.Record{{ID: "m2", Fields: map[string]any{"subject": "world"}}},
		}, nil
	}
	if err := worker.SyncKind(context.Background(), backend.ObjectKindMessage); err != nil {
		t.Fatalf("SyncKind: %v", err)
	}
	if got := countRows(t, w, "message", accountID); got != 1 {
		t.Fatalf("message rows = %d, want 1: an un-echoed change must still apply", got)
	}
}
