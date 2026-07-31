package sync

import (
	"context"
	"database/sql"
	"slices"
	"time"

	"github.com/glw907/poplar/internal/store"
)

// bulkChunk is how many records one bulk-lane transaction may write,
// the bound that keeps a backend's page size out of a write
// transaction's size. A live pull against Fastmail, whose pages carry
// 500 records, logged 205-243 ms for a page written as one
// transaction: four to five times ADR-0003 revision 2's 50 ms
// admission ceiling. At that rate 50 records commit in 20-25 ms, half
// the ceiling, so a slower disk still fits, and the interactive lane
// gets a preemption point every 50 records rather than one per page.
// A changes-since page's upserts and a resync page's upserts and
// deletes are the same shape here, so all three write through it.
const bulkChunk = 50

// staleScanPage is how many rows one bulk-lane transaction may read
// while scanning for a resync's stale-delete pass, store.StaleMessageIDs
// and store.StaleMailboxIDs's LIMIT: a read costs the transaction
// nothing to roll back, so bulkChunk's write measurement does not
// bound it. Measured over 50k rows: at 500, 101 pages at a 2.79 ms
// worst page, 5.6% of the ceiling, against 1001 pages at 50. Each page
// is a full bulk-lane round trip gated by InteractiveQuiet, 1 s in
// production, so the page count is what sustained interactive use
// pays for, not the per-page cost.
const staleScanPage = 500

// runBulkChunks admits fn onto writer's bulk lane once no interactive
// commit has landed within quiet of now, ADR-0003 revision 2's
// backfill subordination policy: the lane's own queue depth cannot
// signal this, since it reads empty exactly when the writer is busy
// running the chunk ahead of it. Every caller that pages through a
// changes-since or resync batch runs each page's write through this,
// so a long sync never holds the bulk lane against interactive use.
func runBulkChunks(ctx context.Context, w *store.Writer, quiet time.Duration, fn func(*sql.Tx) error) error {
	for w.RecentInteractiveActivity(quiet) {
		select {
		case <-time.After(quiet):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return w.Apply(ctx, fn)
}

// applyChunked runs fn over items, bulkChunk of them per bulk-lane
// transaction. A page arrives at whatever size the backend pages at,
// which is the backend's business; how much of one a single
// transaction may hold is the admission ceiling's. A chunk that fails
// rolls back its own items alone, leaving every chunk before it
// committed.
func applyChunked[T any](ctx context.Context, w *Worker, items []T, fn func(*sql.Tx, T) error) error {
	for chunk := range slices.Chunk(items, bulkChunk) {
		err := runBulkChunks(ctx, w.writer, w.cfg.InteractiveQuiet, func(tx *sql.Tx) error {
			for _, item := range chunk {
				if err := fn(tx, item); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
