package outbox

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/uerr"
)

// openWriterAt opens a migrated store at path and returns a Writer
// over it. A recovery test needs the store's own path and closes its
// writer mid-test to release the file, neither of which
// storetest.OpenWriter offers.
func openWriterAt(t *testing.T, path string) *store.Writer {
	t.Helper()

	db, err := store.OpenWriteConn(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	w, err := store.NewWriter(db, store.DefaultWriterConfig())
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	return w
}

// resyncMailboxes upserts mailboxes in the order given, the shape a
// baseline resync page reaches the store in: order matters here,
// since a rebuilt store mints its ids in arrival order.
func resyncMailboxes(t *testing.T, w *store.Writer, accountID int64, mailboxes ...store.MailboxUpsert) {
	t.Helper()

	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		for _, m := range mailboxes {
			if err := store.UpsertMailbox(tx, accountID, m); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("resync mailboxes: %v", err)
	}
}

// TestRecoveredRenameReachesItsOwnMailbox is SY-8's misrouting gate.
// A rename queued against a mailbox row must still reach that
// mailbox's own server id after --recover rebuilds the store and the
// next sync repopulates it. Discarding the mailbox rows while keeping
// the intent leaves the payload's internal key naming whatever
// mailbox the resync mints under that id, and the rename lands on a
// different folder with no error and no log line.
func TestRecoveredRenameReachesItsOwnMailbox(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "store.db")
	w := openWriterAt(t, path)

	accountID := seedAccount(t, w)
	seedMailbox(t, w, accountID, "Archive", "mbx-archive")
	seedMailbox(t, w, accountID, "Receipts", "mbx-receipts")
	travelID := seedMailbox(t, w, accountID, "Travel", "mbx-travel")

	now := time.Now()
	if _, _, err := EnqueueRenameMailbox(ctx, w, accountID, travelID, "Trips", now); err != nil {
		t.Fatalf("EnqueueRenameMailbox: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	if _, err := store.Recover(ctx, path); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	w = openWriterAt(t, path)
	defer func() { _ = w.Close() }()

	resyncMailboxes(t, w, accountID,
		store.MailboxUpsert{ServerID: "mbx-travel", Name: "Travel"},
		store.MailboxUpsert{ServerID: "mbx-archive", Name: "Archive"},
		store.MailboxUpsert{ServerID: "mbx-receipts", Name: "Receipts"},
	)

	var renamedID, renamedName string
	be := newFakeBackend()
	be.MailSource.RenameMailboxFunc = func(_ context.Context, id, name string) error {
		renamedID, renamedName = id, name
		return nil
	}

	res, err := NewDispatcher(accountID, be, w).DispatchOnce(ctx, now)
	if err != nil {
		t.Fatalf("DispatchOnce: %v", err)
	}
	if len(res.Failures) != 0 {
		t.Fatalf("Failures = %+v, want none", res.Failures)
	}
	if renamedID != "mbx-travel" {
		t.Errorf("renamed mailbox %q, want %q: the recovered intent reached a different folder", renamedID, "mbx-travel")
	}
	if renamedName != "Trips" {
		t.Errorf("renamed to %q, want %q", renamedName, "Trips")
	}
}

// TestRecoveredRenameOfDeletedMailboxFails is the other half of SY-8's
// misrouting gate: a preserved mailbox the server has since dropped is
// swept by the resync, and the intent naming it must fail into
// ClassNotFound rather than reach any mailbox at all.
func TestRecoveredRenameOfDeletedMailboxFails(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "store.db")
	w := openWriterAt(t, path)

	accountID := seedAccount(t, w)
	seedMailbox(t, w, accountID, "Archive", "mbx-archive")
	travelID := seedMailbox(t, w, accountID, "Travel", "mbx-travel")

	now := time.Now()
	if _, _, err := EnqueueRenameMailbox(ctx, w, accountID, travelID, "Trips", now); err != nil {
		t.Fatalf("EnqueueRenameMailbox: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	if _, err := store.Recover(ctx, path); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	w = openWriterAt(t, path)
	defer func() { _ = w.Close() }()

	resyncMailboxes(t, w, accountID, store.MailboxUpsert{ServerID: "mbx-archive", Name: "Archive"})
	sweepStaleMailboxes(t, w, accountID, map[string]bool{"mbx-archive": true})

	be := newFakeBackend()
	be.MailSource.RenameMailboxFunc = func(_ context.Context, id, _ string) error {
		t.Errorf("RenameMailbox called for %q, want no call: the mailbox is gone", id)
		return nil
	}

	res, err := NewDispatcher(accountID, be, w).DispatchOnce(ctx, now)
	if err != nil {
		t.Fatalf("DispatchOnce: %v", err)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("Failures = %+v, want exactly one", res.Failures)
	}
	if got := res.Failures[0]; got.Class != uerr.ClassNotFound || got.Retrying {
		t.Errorf("failure = %+v, want ClassNotFound and no retry", got)
	}
}

// sweepStaleMailboxes drops accountID's mailbox rows whose server id
// is absent from keep, the deletion half of a baseline resync.
func sweepStaleMailboxes(t *testing.T, w *store.Writer, accountID int64, keep map[string]bool) {
	t.Helper()

	err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
		stale, err := store.StaleMailboxIDs(tx, accountID, keep)
		if err != nil {
			return err
		}
		for _, id := range stale {
			if err := store.DeleteMailboxByID(tx, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("sweep stale mailboxes: %v", err)
	}
}
