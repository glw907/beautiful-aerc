package outbox

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/glw907/poplar/internal/store"
)

// ReclaimOrphaned returns every outbox row left in dispatching to
// queued. A row enters dispatching in DispatchOnce's claim
// transaction and leaves it in the finalize transaction that follows
// the backend call, so a process that dies between those two commits
// leaves the row behind in dispatching, where selectEligible never
// selects it again.
//
// The instance lock is what makes an unconditional sweep correct:
// poplar refuses to start while another instance holds the lock over
// this store, so once this process owns it no live dispatcher can own
// a dispatching row. Every such row is orphaned by definition, which
// leaves no timestamp heuristic to tune and no ambiguity to resolve.
//
// Replay is idempotent for every kind except one window in
// KindCreateMailbox, which TestIdempotentReplay covers on the safe
// side and TestCreateMailboxReplayWindow pins on the unsafe one. A
// create becomes replay-safe when the transaction after its backend
// call records the new mailbox's server id in its payload. A run
// killed between the two leaves a row this sweep requeues and a
// mailbox the server has already made. Nothing on the backend seam
// tells that row apart from one whose create never reached the
// server, so the next dispatch creates the mailbox a second time.
//
// Call it once at startup, after the instance lock is held and before
// any Dispatcher runs.
func ReclaimOrphaned(ctx context.Context, w *store.Writer) error {
	var reclaimed int
	err := w.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		var err error
		reclaimed, err = requeueDispatching(tx)
		return err
	})
	if err != nil {
		return err
	}
	// A reclaimed intent is the only surviving evidence that the run
	// before this one died partway through a dispatch.
	if reclaimed > 0 {
		slog.Info("outbox: requeued intents left dispatching by a previous run", "count", reclaimed)
	}
	return nil
}
