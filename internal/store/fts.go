package store

import (
	"database/sql"

	"github.com/glw907/poplar/internal/uerr"
)

// RebuildIndex regenerates message_fts from message's current rows,
// discarding whatever the index held before. It issues FTS5's own
// 'rebuild' command against the external-content table. message_fts
// is derived state (SR-1). A caller with reason to doubt it, poplar's
// --rebuild-index flag or a failed integrity check, rebuilds the
// whole index rather than reconciling term by term.
func RebuildIndex(db *sql.DB) error {
	if _, err := db.Exec(`INSERT INTO message_fts(message_fts) VALUES ('rebuild')`); err != nil {
		return uerr.New("store.rebuild-index", nil, uerr.ClassStoreLocal, err)
	}
	return nil
}
