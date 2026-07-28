package outbox

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/backend/backendtest"
	"github.com/glw907/poplar/internal/store"
)

// seedAccount inserts one account row and returns its id.
func seedAccount(t *testing.T, w *store.Writer) int64 {
	t.Helper()
	var id int64
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		res, err := tx.Exec(`INSERT INTO account (slug, backend_kind, address) VALUES (?, ?, ?)`, "acct", "jmap", "geoff@example.com")
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return id
}

// seedMailbox inserts one mailbox row (with a server id, unless
// serverID is empty) and returns its internal id.
func seedMailbox(t *testing.T, w *store.Writer, accountID int64, name, serverID string) int64 {
	t.Helper()
	var id int64
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		var res sql.Result
		var err error
		if serverID == "" {
			res, err = tx.Exec(`INSERT INTO mailbox (account_id, name) VALUES (?, ?)`, accountID, name)
		} else {
			res, err = tx.Exec(`INSERT INTO mailbox (account_id, name, server_id) VALUES (?, ?, ?)`, accountID, name, serverID)
		}
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		t.Fatalf("seed mailbox: %v", err)
	}
	return id
}

// seedMessage inserts one message row with serverID and places it in
// mailboxID, returning the message's internal id.
func seedMessage(t *testing.T, w *store.Writer, accountID, mailboxID int64, serverID string) int64 {
	t.Helper()
	var id int64
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`INSERT INTO message (account_id, server_id, received_at) VALUES (?, ?, ?)`,
			accountID, serverID, time.Now().Unix(),
		)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO message_mailbox (message_id, mailbox_id, received_at) VALUES (?, ?, ?)`, id, mailboxID, time.Now().Unix())
		return err
	})
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}
	return id
}

// outboxCount returns how many outbox rows exist for id.
func outboxCount(t *testing.T, w *store.Writer, id int64) int {
	t.Helper()
	var n int
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT COUNT(*) FROM outbox WHERE id = ?`, id).Scan(&n)
	})
	if err != nil {
		t.Fatalf("count outbox row %d: %v", id, err)
	}
	return n
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

// readPayload returns id's raw payload column.
func readPayload(t *testing.T, w *store.Writer, id int64) []byte {
	t.Helper()
	var payload []byte
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT payload FROM outbox WHERE id = ?`, id).Scan(&payload)
	})
	if err != nil {
		t.Fatalf("read payload %d: %v", id, err)
	}
	return payload
}

// readUndoGroup returns id's undo_group column.
func readUndoGroup(t *testing.T, w *store.Writer, id int64) string {
	t.Helper()
	var group string
	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT COALESCE(undo_group, '') FROM outbox WHERE id = ?`, id).Scan(&group)
	})
	if err != nil {
		t.Fatalf("read undo group %d: %v", id, err)
	}
	return group
}

// newFakeBackend returns a Fake backend with the given per-account
// server limits, ready for its Mail source's fields to be scripted.
func newFakeBackend() *backendtest.Fake {
	return &backendtest.Fake{Caps: backend.Capabilities{Limits: backend.ServerLimits{MaxObjectsInSet: 100}}}
}
