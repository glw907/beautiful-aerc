package sync

import (
	"context"
	"database/sql"
	"time"

	"github.com/glw907/poplar/internal/store"
)

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
