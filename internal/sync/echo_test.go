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
// round-trip into a re-apply, and that a third-party change batched
// into the same page as the echo still lands: once
// NoteDispatchedState has recorded a token and the record id it
// produced, a push-triggered cycle that resolves to that same token
// still advances the watermark, applies every other record the page
// carries, and skips only the noted id. A later, distinct state
// change applies normally.
func TestSelfEchoSuppressed(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{
			NewToken: "dispatched-1",
			Created: []backend.Record{
				{ID: "m1", Fields: map[string]any{"subject": "hello"}},
				{ID: "m3", Fields: map[string]any{"subject": "third party"}},
			},
		}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	worker.NoteDispatchedState(backend.ObjectKindMessage, "dispatched-1", []string{"m1"})

	if err := worker.SyncKind(context.Background(), backend.ObjectKindMessage); err != nil {
		t.Fatalf("SyncKind: %v", err)
	}

	if messageExistsByServerID(t, w, accountID, "m1") {
		t.Fatal("m1 (the echo) applied, want it suppressed")
	}
	if !messageExistsByServerID(t, w, accountID, "m3") {
		t.Fatal("m3 (a third party's change batched into the same page) not applied, want it to land")
	}
	if got := countRows(t, w, "message", accountID); got != 1 {
		t.Fatalf("message rows = %d, want 1: only the echoed record is suppressed", got)
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
	if got := countRows(t, w, "message", accountID); got != 2 {
		t.Fatalf("message rows = %d, want 2: an un-echoed change must still apply", got)
	}
}
