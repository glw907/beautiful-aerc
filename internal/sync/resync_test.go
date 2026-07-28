package sync

import (
	"context"
	"testing"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// TestFullResyncPreserves asserts a full resync's preserved set: an
// origin = 'local' draft, its body, and its outbox row survive
// untouched, while a surviving server-origin row keeps its internal
// id (never re-minted) and a server-origin row missing from the new
// listing is deleted.
func TestFullResyncPreserves(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	keepID := seedMessage(t, w, accountID, "srv-keep", "server", "old subject")
	seedMessage(t, w, accountID, "srv-gone", "server", "going away")
	localID := seedMessage(t, w, accountID, "", "local", "draft subject")
	seedBody(t, w, localID, "draft body")
	seedOutbox(t, w, accountID, localID)

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{
			NewToken: "resynced",
			Created: []backend.Record{
				{ID: "srv-keep", Fields: map[string]any{"subject": "new subject"}},
			},
		}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	if err := worker.fullResync(context.Background(), backend.ObjectKindMessage); err != nil {
		t.Fatalf("fullResync: %v", err)
	}

	gotID, gotSubject := messageByServerID(t, w, accountID, "srv-keep")
	if gotID != keepID {
		t.Fatalf("srv-keep internal id = %d, want %d: a surviving row must not be re-minted", gotID, keepID)
	}
	if gotSubject != "new subject" {
		t.Fatalf("srv-keep subject = %q, want %q", gotSubject, "new subject")
	}

	if messageExistsByServerID(t, w, accountID, "srv-gone") {
		t.Fatal("srv-gone still present, want it deleted: the server no longer reports it")
	}

	if !messageExistsByID(t, w, localID) {
		t.Fatal("local draft deleted by resync, want it preserved")
	}
	if !bodyExistsForMessage(t, w, localID) {
		t.Fatal("draft body deleted by resync, want it preserved")
	}
	if got := countOutboxRows(t, w, accountID); got != 1 {
		t.Fatalf("outbox rows = %d, want 1: an undispatched outbox row must survive a resync", got)
	}

	wm, err := loadWatermark(context.Background(), w, accountID, backend.ObjectKindMessage, mailCollection)
	if err != nil {
		t.Fatalf("loadWatermark: %v", err)
	}
	if wm.ServerStateToken != "resynced" {
		t.Fatalf("watermark token = %q, want %q", wm.ServerStateToken, "resynced")
	}
}

// TestFullResyncPagesIndependently asserts a baseline pull spanning
// multiple pages upserts each page as its own bulk-lane transaction
// rather than buffering every page before writing anything, while
// still computing the stale-delete pass over the union of every
// page's ids: a record introduced only on the second page survives,
// and a record present in neither page is deleted only once every
// page has been seen.
func TestFullResyncPagesIndependently(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	seedMessage(t, w, accountID, "srv-gone", "server", "going away")

	calls := 0
	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		calls++
		if calls == 1 {
			return backend.ChangeSet{
				NewToken: "page-1",
				HasMore:  true,
				Created:  []backend.Record{{ID: "m1", Fields: map[string]any{"subject": "first page"}}},
			}, nil
		}
		return backend.ChangeSet{
			NewToken: "page-2",
			Created:  []backend.Record{{ID: "m2", Fields: map[string]any{"subject": "second page"}}},
		}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	if err := worker.fullResync(context.Background(), backend.ObjectKindMessage); err != nil {
		t.Fatalf("fullResync: %v", err)
	}

	if calls != 2 {
		t.Fatalf("Changes calls = %d, want 2", calls)
	}
	if !messageExistsByServerID(t, w, accountID, "m1") {
		t.Fatal("m1 (first page) missing")
	}
	if !messageExistsByServerID(t, w, accountID, "m2") {
		t.Fatal("m2 (second page) missing")
	}
	if messageExistsByServerID(t, w, accountID, "srv-gone") {
		t.Fatal("srv-gone still present, want it deleted once every page has been seen")
	}

	wm, err := loadWatermark(context.Background(), w, accountID, backend.ObjectKindMessage, mailCollection)
	if err != nil {
		t.Fatalf("loadWatermark: %v", err)
	}
	if wm.ServerStateToken != "page-2" {
		t.Fatalf("watermark token = %q, want %q (the last page's token)", wm.ServerStateToken, "page-2")
	}
}
