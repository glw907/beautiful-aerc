package outbox

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/uerr"
)

// ReclaimOrphaned returns every outbox row left in dispatching to
// queued and reports how many it moved. A row enters dispatching in
// DispatchOnce's claim transaction and leaves it in the finalize
// transaction that follows the backend call, so a process that dies
// between those two commits leaves the row behind in dispatching,
// where selectEligible never selects it again.
//
// The instance lock is what makes an unconditional sweep correct:
// poplar refuses to start while another instance holds the lock over
// this store, so once this process owns it no live dispatcher can own
// a dispatching row. Every such row is orphaned by definition, which
// leaves no timestamp heuristic to tune and no ambiguity to resolve.
// Dispatching a reclaimed intent again is safe because replay is
// idempotent per kind, the guarantee TestIdempotentReplay covers.
//
// Call it once at startup, after the instance lock is held and before
// any Dispatcher runs.
func ReclaimOrphaned(ctx context.Context, w *store.Writer) (int, error) {
	var reclaimed int
	err := w.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		var err error
		reclaimed, err = requeueDispatching(tx)
		return err
	})
	if err != nil {
		return 0, err
	}
	// A reclaimed intent is the only surviving evidence that the run
	// before this one died partway through a dispatch.
	if reclaimed > 0 {
		_ = uerr.New("outbox.reclaim", nil, uerr.ClassStoreLocal,
			fmt.Errorf("requeued %d intent(s) left dispatching by a run that ended mid-dispatch", reclaimed))
	}
	return reclaimed, nil
}
