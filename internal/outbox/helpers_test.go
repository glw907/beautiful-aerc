package outbox

import (
	"context"
	"database/sql"
	"fmt"
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

// occupyWriter holds the store's single writer goroutine inside one
// transaction for d and returns once that transaction has started. A
// write enqueued during the window waits for the lane rather than
// being admitted at once, so a caller carrying an already-cancelled
// context loses the wait and never reaches the store.
func occupyWriter(t *testing.T, w *store.Writer, d time.Duration) {
	t.Helper()

	started := make(chan struct{})
	go func() {
		_ = w.ApplyInteractive(context.Background(), func(*sql.Tx) error {
			close(started)
			time.Sleep(d)
			return nil
		})
	}()
	<-started
}

// newFakeBackend returns a Fake backend with a 100-object
// MaxObjectsInSet limit, ready for its Mail source's fields to be
// scripted. A test wanting a different limit overrides Caps.Limits
// directly on the returned value.
func newFakeBackend() *backendtest.Fake {
	return &backendtest.Fake{Caps: backend.Capabilities{Limits: backend.ServerLimits{MaxObjectsInSet: 100}}}
}

// mailboxServer is the half of a mail server a create test has to
// model for its assertion to mean anything: the sibling-uniqueness
// rule of RFC 8621 section 2, "There MUST NOT be two sibling Mailboxes
// with both the same parent and the same name", which both servers
// poplar is exercised against enforce by refusing the second create. A
// CreateMailboxFunc that always succeeds lets a replay pass a test the
// account it modelled would have ended up holding two folders in.
type mailboxServer struct {
	mu      sync.Mutex
	ids     map[string]string
	creates int
}

// newMailboxServer scripts be's CreateMailbox and FindMailboxes
// against an empty account and returns the server behind them.
func newMailboxServer(be *backendtest.Fake) *mailboxServer {
	s := &mailboxServer{ids: map[string]string{}}
	be.MailSource.CreateMailboxFunc = s.create
	be.MailSource.FindMailboxesFunc = s.find
	return s
}

func (s *mailboxServer) create(_ context.Context, name, parentID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.creates++
	if _, taken := s.ids[siblingKey(name, parentID)]; taken {
		return "", fmt.Errorf("rejected: %w", backend.ErrMailboxNameExists)
	}
	id := fmt.Sprintf("mbx-%d", len(s.ids)+1)
	s.ids[siblingKey(name, parentID)] = id
	return id, nil
}

// find answers on an exact name and parent, which is what
// backend.Mail's contract promises whatever a wire filter matches.
func (s *mailboxServer) find(_ context.Context, name, parentID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id, held := s.ids[siblingKey(name, parentID)]; held {
		return []string{id}, nil
	}
	return nil, nil
}

// serverID returns the id s assigned name under parentID, or the empty
// string when it holds no such mailbox.
func (s *mailboxServer) serverID(name, parentID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ids[siblingKey(name, parentID)]
}

// mailboxes returns how many mailboxes the account holds, which is
// what a duplicate-folder assertion is actually about. Counting
// CreateMailbox calls instead passes a replay that made a second
// folder.
func (s *mailboxServer) mailboxes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.ids)
}

// createCalls returns how many CreateMailbox calls reached s,
// refusals included.
func (s *mailboxServer) createCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.creates
}

func siblingKey(name, parentID string) string { return parentID + "/" + name }
