package store

import (
	"context"
	"database/sql"
	"testing"
)

// TestStoreRevisionMonotonic proves the writer's revision counter
// never regresses across commits, and that a read result taken before
// a commit is identifiable as stale next to one taken after: its
// Revision compares less, with no re-query needed.
func TestStoreRevisionMonotonic(t *testing.T) {
	w, path := newTestWriter(t, DefaultWriterConfig())
	pool, err := NewReadPool(path, DefaultReadPoolSize, w.Revision())
	if err != nil {
		t.Fatalf("NewReadPool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	ctx := context.Background()
	seedAccountAndMailbox(t, w)

	before, err := pool.ListMailboxForward(ctx, 1, MailboxCursor{}, 10)
	if err != nil {
		t.Fatalf("ListMailboxForward before insert: %v", err)
	}

	insertMessage(t, w, 1, 100)

	after, err := pool.ListMailboxForward(ctx, 1, MailboxCursor{}, 10)
	if err != nil {
		t.Fatalf("ListMailboxForward after insert: %v", err)
	}

	if after.Revision <= before.Revision {
		t.Fatalf("revision after commit = %d, want > revision before commit %d", after.Revision, before.Revision)
	}
	if len(after.Rows) != len(before.Rows)+1 {
		t.Fatalf("rows after commit = %d, want %d", len(after.Rows), len(before.Rows)+1)
	}

	insertMessage(t, w, 2, 200)

	latest, err := pool.ListMailboxForward(ctx, 1, MailboxCursor{}, 10)
	if err != nil {
		t.Fatalf("ListMailboxForward after second insert: %v", err)
	}
	if latest.Revision <= after.Revision {
		t.Fatalf("revision after second commit = %d, want > %d", latest.Revision, after.Revision)
	}

	// after is now stale next to latest: a caller can tell without
	// re-querying, purely by comparing the two Revision values.
	if after.Revision >= latest.Revision {
		t.Fatalf("after.Revision (%d) is not identifiable as stale next to latest.Revision (%d)", after.Revision, latest.Revision)
	}
}

// seedAccountAndMailbox inserts account 1 and mailbox 1 through w, the
// fixture every read test in this package builds a mailbox on.
func seedAccountAndMailbox(t *testing.T, w *Writer) {
	t.Helper()

	err := w.submit(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO account (id, slug, backend_kind, address) VALUES (1, 'a', 'jmap', 'a@example.com')`); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO mailbox (id, account_id, name) VALUES (1, 1, 'Inbox')`)
		return err
	})
	if err != nil {
		t.Fatalf("seed account and mailbox: %v", err)
	}
}

// insertMessage inserts a message with the given id and received_at
// into mailbox 1 through w, in one committed transaction.
func insertMessage(t *testing.T, w *Writer, id, receivedAt int64) {
	t.Helper()

	err := w.submit(context.Background(), func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO message (id, account_id, received_at) VALUES (?, 1, ?)`, id, receivedAt); err != nil {
			return err
		}
		_, err := tx.Exec(`INSERT INTO message_mailbox (message_id, mailbox_id, received_at) VALUES (?, 1, ?)`, id, receivedAt)
		return err
	})
	if err != nil {
		t.Fatalf("insert message %d: %v", id, err)
	}
}
