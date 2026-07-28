package outbox

import (
	"context"
	"database/sql"

	"github.com/glw907/poplar/internal/store"
)

// Undo annihilates every id still queued, inside one writer
// transaction, and reports which ones it caught. An id no longer
// queued (already claimed for dispatch, or already gone because it
// dispatched) is left untouched and absent from the result: ADR-0006
// revision 2's claim discipline makes this the same guarded statement
// the dispatcher's own claim uses, so the two can never both act on
// one row. A caller that gets back fewer ids than it asked for
// already dispatched the rest, and compensates by enqueueing the
// reverse mutation itself, using whatever prior state it holds (a
// Delivered entry's Move payload, for a move already reported back).
func Undo(ctx context.Context, w *store.Writer, ids []int64) ([]int64, error) {
	var annihilated []int64
	err := w.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		for _, id := range ids {
			ok, err := annihilate(tx, id)
			if err != nil {
				return err
			}
			if ok {
				annihilated = append(annihilated, id)
			}
		}
		return nil
	})
	return annihilated, err
}
