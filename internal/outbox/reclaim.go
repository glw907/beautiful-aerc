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
// Replay is idempotent for every kind. KindCreateMailbox is the one
// that has to earn it. A create records the new mailbox's server id in
// the transaction after its backend call, so a run killed between the
// two leaves a row this sweep requeues and a mailbox the server has
// already made, and nothing tells that row apart from one whose create
// never reached the server. So the replay makes the call again and the
// server refuses it, RFC 8621 section 2 forbidding two sibling
// mailboxes with the same parent and the same name. dispatchCreateMailbox
// adopts the mailbox that refusal is about, which leaves the account
// with one folder either way. TestIdempotentReplay covers the replay
// that reads its id off its own payload; TestCreateMailboxReplayWindow
// covers the one that has to ask the server.
//
// That claim is scoped to the four kinds this build dispatches, not a
// standing property of the outbox. A future kind whose server call
// cannot be safely repeated needs its own answer before this sweep can
// say the same of it: a send intent is the concrete case, since no
// server refuses a second identical EmailSubmission/set the way a
// mailbox create's own refusal gives dispatchCreateMailbox something
// to reconcile against.
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
