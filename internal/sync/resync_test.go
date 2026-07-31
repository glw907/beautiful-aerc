package sync

import (
	"context"
	"fmt"
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
				{ID: "srv-keep", Fields: backend.MessageFields{Subject: "new subject"}},
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
				Created:  []backend.Record{{ID: "m1", Fields: backend.MessageFields{Subject: "first page"}}},
			}, nil
		}
		return backend.ChangeSet{
			NewToken: "page-2",
			Created:  []backend.Record{{ID: "m2", Fields: backend.MessageFields{Subject: "second page"}}},
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

// TestFullResyncCommitsOneChunkPerTransaction asserts a baseline page
// reaches the store 50 records per transaction rather than one
// transaction per page, ADR-0003 revision 2's admission ceiling. It
// measures transaction scope rather than elapsed time: record 123
// carries a payload that disagrees with the page it arrived on, which
// fails the transaction it lands in and rolls that one back, so what
// the store still holds afterwards is exactly what committed before
// the failure. Every number here is the test's own, not resyncChunk
// read back: a page committed whole, or a chunk grown past 123
// records, leaves nothing behind and fails this. The watermark stays
// unset, the same crash-safety argument TestSyncKindCommitsOneChunkPerTransaction
// proves for a changes-since page: a resync that stops mid-page must
// not tell the next cycle it finished.
func TestFullResyncCommitsOneChunkPerTransaction(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	page := make([]backend.Record, 0, 130)
	for i := range cap(page) {
		page = append(page, backend.Record{
			ID:     fmt.Sprintf("srv-%03d", i),
			Fields: backend.MessageFields{Subject: "baseline"},
		})
	}
	// A mailbox payload on a message page is what upsertRecord
	// rejects.
	page[123].Fields = backend.MailboxFields{}

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{NewToken: "resynced", Created: page}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	if err := worker.fullResync(context.Background(), backend.ObjectKindMessage); err == nil {
		t.Fatal("fullResync succeeded, want the rejected payload to fail its chunk")
	}

	if got := countRows(t, w, "message", accountID); got != 100 {
		t.Errorf("committed message rows = %d, want 100: records 0-99 commit in two transactions of their own before 123 fails a third", got)
	}

	wm, err := loadWatermark(context.Background(), w, accountID, backend.ObjectKindMessage, mailCollection)
	if err != nil {
		t.Fatalf("loadWatermark: %v", err)
	}
	if wm.ServerStateToken != "" {
		t.Errorf("watermark token = %q, want it unset: a resync that failed partway must not advance it", wm.ServerStateToken)
	}
}

// TestFullResyncChunksTheStaleDeletePass asserts both halves of the
// stale-delete pass are bounded the same way the upserts are: the
// scan pages by internal id across transactions of its own, and the
// deletes follow each page. The store's revision counter advances
// once per committed transaction, so what it gains over the resync is
// the transaction count: scope again, not elapsed time.
func TestFullResyncChunksTheStaleDeletePass(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	for i := range 101 {
		seedMessage(t, w, accountID, fmt.Sprintf("srv-%03d", i), "server", "going away")
	}

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{NewToken: "resynced"}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	before := w.Revision().Current()
	if err := worker.fullResync(context.Background(), backend.ObjectKindMessage); err != nil {
		t.Fatalf("fullResync: %v", err)
	}
	committed := w.Revision().Current() - before

	// Two transactions scan the 101 rows (one page holds them all at
	// staleScanPage width, the next finds none left and ends the
	// pass), three delete what the first found (bulkChunk at a time),
	// one saves the watermark.
	const want = store.Revision(2 + 3 + 1)
	if committed != want {
		t.Errorf("transactions committed = %d, want %d: 101 stale rows scan in one page and delete 50 at a time", committed, want)
	}
	if got := countRows(t, w, "message", accountID); got != 0 {
		t.Errorf("message rows left after the resync = %d, want 0", got)
	}
}

// TestFullResyncScansPastKeptRows attacks the stale scan's cursor: it
// has to advance over every row the page examined, not only the rows
// that page found stale. A whole page of survivors sits ahead of the
// stale rows here, so a cursor drawn from the stale ids alone stops
// the pass before it ever reaches them.
func TestFullResyncScansPastKeptRows(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	// 60 survivors, comfortably inside one scan page at staleScanPage
	// width: this proves the keep-based exclusion alone. The two tests
	// below prove the account and origin filter's cursor advances past
	// a page too big to fit inside one.
	const kept, gone = 60, 30

	var keptIDs []int64
	page := make([]backend.Record, 0, kept)
	for i := range kept {
		serverID := fmt.Sprintf("keep-%03d", i)
		keptIDs = append(keptIDs, seedMessage(t, w, accountID, serverID, "server", "staying"))
		page = append(page, backend.Record{
			ID:     serverID,
			Fields: backend.MessageFields{Subject: "staying"},
		})
	}
	for i := range gone {
		seedMessage(t, w, accountID, fmt.Sprintf("gone-%03d", i), "server", "going away")
	}

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{NewToken: "resynced", Created: page}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	if err := worker.fullResync(context.Background(), backend.ObjectKindMessage); err != nil {
		t.Fatalf("fullResync: %v", err)
	}

	if got := countRows(t, w, "message", accountID); got != kept {
		t.Errorf("message rows after the resync = %d, want %d: every gone-* row sits behind a full page of survivors", got, kept)
	}
	for _, id := range keptIDs {
		if !messageExistsByID(t, w, id) {
			t.Fatalf("kept message %d deleted by the resync", id)
		}
	}
}

// TestFullResyncScansPastLocalOriginRows attacks the stale scan's
// cursor from the filter's side rather than keep's: the cursor has to
// advance over a row the account and origin filter discards, not only
// a row keep excludes. A resync scoped to origin = 'server' rows
// never treats a draft as a deletion candidate, so a page that is
// entirely drafts finds nothing stale either, and a cursor that only
// advances past a matching row reads that as the end of the scan. A
// compose burst or a restore leaves exactly this shape in a
// single-account inbox today: a run of drafts ahead of the stale rows
// in id order.
func TestFullResyncScansPastLocalOriginRows(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	// More drafts than one scan page holds, so the first page fails
	// the origin filter start to finish.
	const drafts, gone = staleScanPage + 5, 30

	for i := range drafts {
		seedMessage(t, w, accountID, "", "local", fmt.Sprintf("draft %d", i))
	}
	for i := range gone {
		seedMessage(t, w, accountID, fmt.Sprintf("gone-%03d", i), "server", "going away")
	}

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{NewToken: "resynced"}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	if err := worker.fullResync(context.Background(), backend.ObjectKindMessage); err != nil {
		t.Fatalf("fullResync: %v", err)
	}

	if got := countRows(t, w, "message", accountID); got != drafts {
		t.Errorf("message rows after the resync = %d, want %d: every gone-* row sits behind a full page of drafts", got, drafts)
	}
}

// TestFullResyncScansPastOtherAccountRows is
// TestFullResyncScansPastLocalOriginRows's multi-account twin: a page
// that fails the filter on account_id rather than origin exposes the
// same cursor shape.
func TestFullResyncScansPastOtherAccountRows(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	otherID := storetest.Insert(t, w,
		`INSERT INTO account (slug, backend_kind, address) VALUES (?, ?, ?)`,
		"test-other", "jmap", "other@example.com")

	// More of the other account's rows than one scan page holds, so
	// the first page fails the account filter start to finish.
	const otherRows, gone = staleScanPage + 5, 30

	for i := range otherRows {
		seedMessage(t, w, otherID, fmt.Sprintf("other-%03d", i), "server", "another account")
	}
	for i := range gone {
		seedMessage(t, w, accountID, fmt.Sprintf("gone-%03d", i), "server", "going away")
	}

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{NewToken: "resynced"}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	if err := worker.fullResync(context.Background(), backend.ObjectKindMessage); err != nil {
		t.Fatalf("fullResync: %v", err)
	}

	if got := countRows(t, w, "message", accountID); got != 0 {
		t.Errorf("resynced account's message rows = %d, want 0: every gone-* row sits behind a full page of another account's rows", got)
	}
	if got := countRows(t, w, "message", otherID); got != otherRows {
		t.Errorf("other account's message rows = %d, want %d: a resync must never touch another account's rows", got, otherRows)
	}
}

// TestFullResyncMailboxScansPastOtherAccountRows closes
// StaleMailboxIDs's half of the finding the two message fixtures
// above prove for StaleMessageIDs: the same cursor-above-the-filter
// shape, exercised past one scan page for the first time. Mailbox
// carries no origin column, so the account filter alone stands in for
// both message filters.
func TestFullResyncMailboxScansPastOtherAccountRows(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)
	otherID := storetest.Insert(t, w,
		`INSERT INTO account (slug, backend_kind, address) VALUES (?, ?, ?)`,
		"test-other", "jmap", "other@example.com")

	const otherRows, gone = staleScanPage + 5, 30

	for i := range otherRows {
		seedMailbox(t, w, otherID, fmt.Sprintf("other-%03d", i), "Other")
	}
	for i := range gone {
		seedMailbox(t, w, accountID, fmt.Sprintf("gone-%03d", i), "Going")
	}

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{NewToken: "resynced"}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	if err := worker.fullResync(context.Background(), backend.ObjectKindMailbox); err != nil {
		t.Fatalf("fullResync: %v", err)
	}

	if got := countRows(t, w, "mailbox", accountID); got != 0 {
		t.Errorf("resynced account's mailbox rows = %d, want 0: every gone-* row sits behind a full page of another account's rows", got)
	}
	if got := countRows(t, w, "mailbox", otherID); got != otherRows {
		t.Errorf("other account's mailbox rows = %d, want %d: a resync must never touch another account's rows", got, otherRows)
	}
}
