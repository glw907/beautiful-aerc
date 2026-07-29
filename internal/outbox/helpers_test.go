package outbox

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// seedAccount inserts one account row and returns its id.
func seedAccount(t *testing.T, w *store.Writer) int64 {
	t.Helper()
	return storetest.Insert(t, w,
		`INSERT INTO account (slug, backend_kind, address) VALUES (?, ?, ?)`,
		"acct", "jmap", "geoff@example.com")
}

// seedMailbox inserts one mailbox row (with a server id, unless
// serverID is empty) and returns its internal id.
func seedMailbox(t *testing.T, w *store.Writer, accountID int64, name, serverID string) int64 {
	t.Helper()
	if serverID == "" {
		return storetest.Insert(t, w, `INSERT INTO mailbox (account_id, name) VALUES (?, ?)`, accountID, name)
	}
	return storetest.Insert(t, w,
		`INSERT INTO mailbox (account_id, name, server_id) VALUES (?, ?, ?)`,
		accountID, name, serverID)
}

// seedMessage inserts one message row with serverID and places it in
// mailboxID, returning the message's internal id.
func seedMessage(t *testing.T, w *store.Writer, accountID, mailboxID int64, serverID string) int64 {
	t.Helper()
	id := storetest.Insert(t, w,
		`INSERT INTO message (account_id, server_id, received_at) VALUES (?, ?, ?)`,
		accountID, serverID, time.Now().Unix())
	storetest.Insert(t, w,
		`INSERT INTO message_mailbox (message_id, mailbox_id, received_at) VALUES (?, ?, ?)`,
		id, mailboxID, time.Now().Unix())
	return id
}

// outboxCount returns how many outbox rows exist for id.
func outboxCount(t *testing.T, w *store.Writer, id int64) int {
	t.Helper()
	return storetest.ScanValue[int](t, w, `SELECT COUNT(*) FROM outbox WHERE id = ?`, id)
}

// outboxState returns id's state and attempt_count. It fails the test
// if id no longer exists.
func outboxState(t *testing.T, w *store.Writer, id int64) (state string, attempts int) {
	t.Helper()
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT state, attempt_count FROM outbox WHERE id = ?`, id).Scan(&state, &attempts)
	})
	if err != nil {
		t.Fatalf("read outbox row %d: %v", id, err)
	}
	return state, attempts
}

// dispatchingCount returns how many of accountID's outbox rows are
// still in the dispatching state.
func dispatchingCount(t *testing.T, w *store.Writer, accountID int64) int {
	t.Helper()
	return storetest.ScanValue[int](t, w,
		`SELECT COUNT(*) FROM outbox WHERE account_id = ? AND state = 'dispatching'`, accountID)
}

// readPayload returns id's raw payload column.
func readPayload(t *testing.T, w *store.Writer, id int64) []byte {
	t.Helper()
	return storetest.ScanValue[[]byte](t, w, `SELECT payload FROM outbox WHERE id = ?`, id)
}

// readUndoGroup returns id's undo_group column.
func readUndoGroup(t *testing.T, w *store.Writer, id int64) string {
	t.Helper()
	return storetest.ScanValue[string](t, w, `SELECT COALESCE(undo_group, '') FROM outbox WHERE id = ?`, id)
}

// strandDispatching moves id to dispatching and leaves it there,
// modelling the claim transaction a run committed before it died
// inside the backend call that followed.
func strandDispatching(t *testing.T, w *store.Writer, id int64) {
	t.Helper()
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		return claimRow(tx, id)
	})
	if err != nil {
		t.Fatalf("strand outbox row %d: %v", id, err)
	}
}

// captureSlog redirects slog's process-wide default logger to an
// in-memory buffer for the rest of the test, restoring the previous
// default on cleanup.
func captureSlog(t *testing.T) *logBuffer {
	t.Helper()

	buf := &logBuffer{}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return buf
}

// logBuffer is captureSlog's destination, guarded because the store's
// writer goroutine logs on its own schedule while the test goroutine
// reads what has arrived.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newFakeBackend returns a Fake backend with a 100-object
// MaxObjectsInSet limit, ready for its Mail source's fields to be
// scripted. A test wanting a different limit overrides Caps.Limits
// directly on the returned value.
func newFakeBackend() *backendtest.Fake {
	return &backendtest.Fake{Caps: backend.Capabilities{Limits: backend.ServerLimits{MaxObjectsInSet: 100}}}
}
