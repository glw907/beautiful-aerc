//go:build live

package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/keyring"
	"github.com/glw907/poplar/internal/outbox"
	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/store/storetest"
)

// TestLiveRunnerFillsStoreAndDispatchesATriageIntent runs the
// runner's real composition (connectLiveJMAP, ensureAccount,
// startEngines) against Geoff's live Fastmail account
// (FASTMAIL_API_TOKEN), gated the same way jmap's own live suite is:
// never in CI or make check, skipped where the token was never
// sourced. It proves mailboxes and messages land in the local store
// through the sync worker's poll path, then enqueues a triage move
// (LT-2's move verb) on an already-synced message, back into the
// mailbox it already occupies, and proves that intent reaches the
// server: the safest real exercise of the dispatch path against a
// live inbox, since it changes nothing about the message's final
// mailbox membership.
func TestLiveRunnerFillsStoreAndDispatchesATriageIntent(t *testing.T) {
	if _, err := keyring.Token(""); err != nil {
		t.Skipf("no fastmail token: %v", err)
	}

	w := storetest.OpenWriter(t, store.DefaultWriterConfig())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	be, key, err := connectLiveJMAP(ctx)
	if err != nil {
		t.Fatalf("connectLiveJMAP: %v", err)
	}
	accountID, err := ensureAccount(ctx, w, key)
	if err != nil {
		t.Fatalf("ensureAccount: %v", err)
	}

	wg := startEngines(ctx, accountID, be, w)
	stop := func() {
		cancel()
		wg.Wait()
	}
	defer stop()

	messageID, mailboxID := waitForAMessage(t, w, accountID, 2*time.Minute)

	_, intentIDs, err := outbox.EnqueueMoveMessagesBulk(ctx, w, accountID, []int64{messageID}, mailboxID, 0, be, false, time.Now())
	if err != nil {
		t.Fatalf("EnqueueMoveMessagesBulk: %v", err)
	}
	waitForDispatch(t, w, intentIDs[0], 30*time.Second)
}

// waitForAMessage polls the store for accountID's first synced
// message and the mailbox it currently occupies, failing the test if
// none lands within timeout.
func waitForAMessage(t *testing.T, w *store.Writer, accountID int64, timeout time.Duration) (messageID, mailboxID int64) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
			return tx.QueryRow(
				`SELECT mm.message_id, mm.mailbox_id FROM message_mailbox mm
				 JOIN message m ON m.id = mm.message_id
				 WHERE m.account_id = ? LIMIT 1`, accountID).Scan(&messageID, &mailboxID)
		})
		if err == nil {
			return messageID, mailboxID
		}
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("query a synced message: %v", err)
		}
		// A wide poll interval on the writer's interactive lane, not a
		// tight one: SyncKind's baseline pages ride the bulk lane, which
		// ADR-0003's InteractiveQuiet subordination defers behind any
		// recent interactive activity, this poll included.
		time.Sleep(5 * time.Second)
	}
	t.Fatalf("no message landed in the store within %s", timeout)
	return 0, 0
}

// waitForDispatch polls the outbox table for intentID's row until it
// is gone (DispatchOnce delivered or gave up on it) or timeout elapses.
func waitForDispatch(t *testing.T, w *store.Writer, intentID int64, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var n int
		err := w.ApplyInteractive(context.Background(), func(tx *sql.Tx) error {
			return tx.QueryRow(`SELECT COUNT(*) FROM outbox WHERE id = ?`, intentID).Scan(&n)
		})
		if err != nil {
			t.Fatalf("check outbox row %d: %v", intentID, err)
		}
		if n == 0 {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("outbox intent %d did not dispatch within %s", intentID, timeout)
}
