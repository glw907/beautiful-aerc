package sync

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

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

// unreadForServerID returns serverID's unread flag within mailboxID.
func unreadForServerID(t *testing.T, w *store.Writer, accountID int64, serverID string, mailboxID int64) bool {
	t.Helper()

	return storetest.ScanValue[int](t, w,
		`SELECT mm.unread FROM message_mailbox mm JOIN message m ON m.id = mm.message_id WHERE m.account_id = ? AND m.server_id = ? AND mm.mailbox_id = ?`,
		accountID, serverID, mailboxID) != 0
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
