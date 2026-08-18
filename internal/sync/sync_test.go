package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
	"github.com/glw907/poplar/internal/uerr"
)

// TestSyncKindCommitsOneChunkPerTransaction asserts a changes-since
// page reaches the store bulkChunk records per transaction rather
// than one transaction per page, ADR-0003 revision 2's admission
// ceiling. It measures transaction scope rather than elapsed time:
// record 123 carries a payload that disagrees with the page it
// arrived on, which fails the transaction it lands in and rolls that
// one back, so what the store still holds afterwards is exactly what
// committed before the failure. Every number here is the test's own,
// not bulkChunk read back: a page committed whole, or a chunk grown
// past 123 records, leaves nothing behind and fails this.
func TestSyncKindCommitsOneChunkPerTransaction(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	page := make([]backend.Record, 0, 130)
	for i := range cap(page) {
		page = append(page, backend.Record{
			ID:     fmt.Sprintf("srv-%03d", i),
			Fields: backend.MessageFields{Subject: "changed"},
		})
	}
	// A mailbox payload on a message page is what upsertRecord rejects.
	page[123].Fields = backend.MailboxFields{}

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{NewToken: "page-1", Created: page}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	if err := worker.SyncKind(context.Background(), backend.ObjectKindMessage); err == nil {
		t.Fatal("SyncKind succeeded, want the rejected payload to fail its chunk")
	}

	if got := countRows(t, w, "message", accountID); got != 100 {
		t.Errorf("committed message rows = %d, want 100: records 0-99 commit in two transactions of their own before 123 fails a third", got)
	}

	wm, err := loadWatermark(context.Background(), w, accountID, backend.ObjectKindMessage, mailCollection)
	if err != nil {
		t.Fatalf("loadWatermark: %v", err)
	}
	if wm.ServerStateToken != "" {
		t.Errorf("watermark token = %q, want it unset: a page that failed partway must not advance it", wm.ServerStateToken)
	}
}

// TestSyncKindSavesTheWatermarkAfterThePage asserts the watermark
// commits on its own after the page's chunks rather than riding with
// the last of them, so no single transaction holds both a chunk and
// the position that would let the next cycle skip it. The store's
// revision counter advances once per committed transaction, so what
// it gains over the cycle is the transaction count: scope again, not
// elapsed time.
func TestSyncKindSavesTheWatermarkAfterThePage(t *testing.T) {
	w := storetest.OpenWriter(t, store.DefaultWriterConfig())
	accountID := seedAccount(t, w)

	page := make([]backend.Record, 0, 101)
	for i := range cap(page) {
		page = append(page, backend.Record{
			ID:     fmt.Sprintf("srv-%03d", i),
			Fields: backend.MessageFields{Subject: "changed"},
		})
	}

	var be backendtest.Fake
	be.MailSource.ChangesFunc = func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return backend.ChangeSet{NewToken: "page-1", Created: page}, nil
	}

	worker := NewWorker(accountID, &be, w, testConfig())
	before := w.Revision().Current()
	if err := worker.SyncKind(context.Background(), backend.ObjectKindMessage); err != nil {
		t.Fatalf("SyncKind: %v", err)
	}
	committed := w.Revision().Current() - before

	// One transaction loads the watermark, three write the 101 records
	// 50 at a time, one saves the watermark.
	const want = store.Revision(1 + 3 + 1)
	if committed != want {
		t.Errorf("transactions committed = %d, want %d: 101 records write 50 at a time and the watermark follows on its own", committed, want)
	}
	if got := countRows(t, w, "message", accountID); got != 101 {
		t.Errorf("message rows = %d, want 101", got)
	}
}

// TestPagingStopsOnANonAdvancingState is JT-14 against poplar's own
// consumer of the paging contract rather than against a server. A
// page claiming more changes carries the token the next request will
// send, so a server answering with the token it was just given puts
// the loop in a tight round trip re-fetching and re-applying that same
// page, writing a watermark every turn, for as long as the process
// lives. Both loops that page Changes are covered: the ordinary cycle
// and the resync.
func TestPagingStopsOnANonAdvancingState(t *testing.T) {
	// A page of one, so what the assertions below count is round trips
	// rather than records.
	page := []backend.Record{{ID: "srv-1", Fields: backend.MessageFields{Subject: "changed"}}}

	// The malicious fixture's own escape hatch: a guard that regressed
	// would otherwise hang the suite rather than fail it.
	const runaway = 20
	stuck := func(calls *int) func(context.Context, backend.ObjectKind, string, int) (backend.ChangeSet, error) {
		return func(_ context.Context, _ backend.ObjectKind, token string, _ int) (backend.ChangeSet, error) {
			*calls++
			if *calls > runaway {
				return backend.ChangeSet{}, fmt.Errorf("the paging loop made %d calls with token %q", *calls, token)
			}
			return backend.ChangeSet{NewToken: "stuck", HasMore: true, Created: page}, nil
		}
	}

	t.Run("three pages drive the loop to completion", func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		var be backendtest.Fake
		calls := 0
		be.MailSource.ChangesFunc = func(_ context.Context, _ backend.ObjectKind, _ string, _ int) (backend.ChangeSet, error) {
			calls++
			return backend.ChangeSet{
				NewToken: fmt.Sprintf("page-%d", calls),
				HasMore:  calls < 3,
				Created:  []backend.Record{{ID: fmt.Sprintf("srv-%d", calls), Fields: backend.MessageFields{Subject: "changed"}}},
			}, nil
		}

		worker := NewWorker(accountID, &be, w, testConfig())
		if err := worker.SyncKind(context.Background(), backend.ObjectKindMessage); err != nil {
			t.Fatalf("SyncKind over three advancing pages: %v", err)
		}
		if calls != 3 {
			t.Errorf("Changes calls = %d, want 3: the loop stopped short of the last page", calls)
		}
		if got := countRows(t, w, "message", accountID); got != 3 {
			t.Errorf("message rows = %d, want 3 (one per page)", got)
		}
	})

	t.Run("the same token forever is terminal for the cycle", func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		var be backendtest.Fake
		calls := 0
		be.MailSource.ChangesFunc = stuck(&calls)

		worker := NewWorker(accountID, &be, w, testConfig())
		err := worker.SyncKind(context.Background(), backend.ObjectKindMessage)
		if !errors.Is(err, errStateNotAdvancing) {
			t.Fatalf("SyncKind() error = %v, want it to report the non-advancing state", err)
		}
		if class, _ := classifyErr(err); class != uerr.ClassServer {
			t.Errorf("class = %v, want ClassServer: the server is the one contradicting itself", class)
		}
		// The first page's token is what the second call would send.
		if calls != 2 {
			t.Errorf("Changes calls = %d, want 2: the repeat is what proves the token did not move", calls)
		}
	})

	t.Run("a resync's paging loop too", func(t *testing.T) {
		w := storetest.OpenWriter(t, store.DefaultWriterConfig())
		accountID := seedAccount(t, w)

		var be backendtest.Fake
		calls := 0
		next := stuck(&calls)
		be.MailSource.ChangesFunc = func(ctx context.Context, kind backend.ObjectKind, token string, limit int) (backend.ChangeSet, error) {
			// The first call sends the cycle down the resync path, which
			// starts its own paging loop from an empty token.
			if calls == 0 {
				calls++
				return backend.ChangeSet{}, backend.ErrStateReset
			}
			return next(ctx, kind, token, limit)
		}

		worker := NewWorker(accountID, &be, w, testConfig())
		if err := worker.SyncKind(context.Background(), backend.ObjectKindMessage); !errors.Is(err, errStateNotAdvancing) {
			t.Fatalf("SyncKind() error = %v, want the resync to report the non-advancing state", err)
		}
		if calls != 3 {
			t.Errorf("Changes calls = %d, want 3 (the reset, then the repeat)", calls)
		}
	})
}

// testConfig returns DefaultConfig with InteractiveQuiet shrunk, so a
// test's own fixture writes (always run on the interactive lane)
// never leave runBulkChunks waiting out the full production quiet
// window before a scripted sync cycle's write is admitted.
func testConfig() Config {
	cfg := DefaultConfig()
	cfg.InteractiveQuiet = time.Millisecond
	return cfg
}

// seedAccount inserts one account row and returns its id.
func seedAccount(t *testing.T, w *store.Writer) int64 {
	t.Helper()

	return storetest.Insert(t, w,
		`INSERT INTO account (slug, backend_kind, address) VALUES (?, ?, ?)`,
		"test", "jmap", "user@example.com")
}

// seedMailbox inserts one mailbox row under accountID and returns its
// id.
func seedMailbox(t *testing.T, w *store.Writer, accountID int64, serverID, name string) int64 {
	t.Helper()

	return storetest.Insert(t, w,
		`INSERT INTO mailbox (account_id, server_id, name) VALUES (?, ?, ?)`,
		accountID, serverID, name)
}

// countRows returns the row count of table (a fixed test-owned
// literal, never caller input) filtered by account_id = accountID.
func countRows(t *testing.T, w *store.Writer, table string, accountID int64) int {
	t.Helper()

	return storetest.ScanValue[int](t, w, `SELECT count(*) FROM `+table+` WHERE account_id = ?`, accountID)
}

// seedMessage inserts one message row directly (bypassing sync's own
// apply path, since these tests need fixtures the apply path doesn't
// produce, such as an origin = 'local' draft) and returns its id.
// An empty serverID stores a NULL server_id.
func seedMessage(t *testing.T, w *store.Writer, accountID int64, serverID, origin, subject string) int64 {
	t.Helper()

	var sid any
	if serverID != "" {
		sid = serverID
	}
	return storetest.Insert(t, w,
		`INSERT INTO message (account_id, server_id, received_at, subject, origin) VALUES (?, ?, ?, ?, ?)`,
		accountID, sid, 1000, subject, origin)
}

func seedBody(t *testing.T, w *store.Writer, messageID int64, content string) {
	t.Helper()

	storetest.Insert(t, w,
		`INSERT INTO body (message_id, content, fetched_at) VALUES (?, ?, ?)`,
		messageID, []byte(content), 1000)
}

func seedOutbox(t *testing.T, w *store.Writer, accountID, messageID int64) {
	t.Helper()

	storetest.Insert(t, w,
		`INSERT INTO outbox (account_id, kind, payload, created_at) VALUES (?, ?, ?, ?)`,
		accountID, "send", fmt.Sprintf(`{"message_id":%d}`, messageID), 1000)
}

// messageByServerID returns serverID's internal id and subject within
// accountID.
func messageByServerID(t *testing.T, w *store.Writer, accountID int64, serverID string) (id int64, subject string) {
	t.Helper()

	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT id, subject FROM message WHERE account_id = ? AND server_id = ?`, accountID, serverID).Scan(&id, &subject)
	})
	if err != nil {
		t.Fatalf("message by server id %q: %v", serverID, err)
	}
	return id, subject
}

func messageExistsByServerID(t *testing.T, w *store.Writer, accountID int64, serverID string) bool {
	t.Helper()

	return storetest.ScanValue[int](t, w, `SELECT count(*) FROM message WHERE account_id = ? AND server_id = ?`, accountID, serverID) > 0
}

func messageExistsByID(t *testing.T, w *store.Writer, id int64) bool {
	t.Helper()

	return storetest.ScanValue[int](t, w, `SELECT count(*) FROM message WHERE id = ?`, id) > 0
}

func bodyExistsForMessage(t *testing.T, w *store.Writer, messageID int64) bool {
	t.Helper()

	return storetest.ScanValue[int](t, w, `SELECT count(*) FROM body WHERE message_id = ?`, messageID) > 0
}

func countOutboxRows(t *testing.T, w *store.Writer, accountID int64) int {
	t.Helper()

	return storetest.ScanValue[int](t, w, `SELECT count(*) FROM outbox WHERE account_id = ?`, accountID)
}

// mailboxIDsForServerID returns the mailbox ids "m1"'s message is
// currently associated with; every test fixture using this helper
// names its message "m1".
func mailboxIDsForServerID(t *testing.T, w *store.Writer, accountID int64) []int64 {
	t.Helper()

	var ids []int64
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		rows, err := tx.Query(
			`SELECT mm.mailbox_id FROM message_mailbox mm JOIN message m ON m.id = mm.message_id WHERE m.account_id = ? AND m.server_id = ?`,
			accountID, "m1",
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("mailbox ids for \"m1\": %v", err)
	}
	return ids
}

// unreadForMessage returns the unread flag "m1"'s message carries
// within mailboxID; like mailboxIDsForServerID, every fixture using
// this helper names its message "m1".
func unreadForMessage(t *testing.T, w *store.Writer, accountID, mailboxID int64) bool {
	t.Helper()

	return storetest.ScanValue[int](t, w,
		`SELECT mm.unread FROM message_mailbox mm JOIN message m ON m.id = mm.message_id WHERE m.account_id = ? AND m.server_id = ? AND mm.mailbox_id = ?`,
		accountID, "m1", mailboxID) != 0
}

// storeModel returns accountID's current message rows as a
// server_id-to-subject map, the shape TestConvergence compares
// against its in-memory model of the scripted server.
func storeModel(t *testing.T, w *store.Writer, accountID int64) map[string]string {
	t.Helper()

	got := map[string]string{}
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT server_id, subject FROM message WHERE account_id = ?`, accountID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id, subject string
			if err := rows.Scan(&id, &subject); err != nil {
				return err
			}
			got[id] = subject
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("store model: %v", err)
	}
	return got
}
