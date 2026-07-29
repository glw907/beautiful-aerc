package store

import (
	"context"
	"database/sql"
)

// RebuildIndex regenerates message_fts from message's current rows,
// discarding whatever the index held before. It issues FTS5's own
// 'rebuild' command against the external-content table, on the
// writer's bulk lane rather than a bare *sql.DB: a full-index rebuild
// is a whole-table scan, the same shape as the sync engine's own bulk
// writes, and running it outside the writer would break the
// single-writer discipline every other mutation in this package holds
// to. message_fts is derived state (SR-1). A caller with reason to
// doubt it, poplar's --rebuild-index flag or a failed integrity check,
// rebuilds the whole index rather than reconciling term by term.
func RebuildIndex(ctx context.Context, w *Writer) error {
	return w.Apply(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO message_fts(message_fts) VALUES ('rebuild')`)
		return err
	})
}
