package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// reindexMessage keeps message_fts in step with a message write that
// sets id's subject and searchText. Call it inside the same
// transaction as that write, before the write lands: message is
// message_fts's only content source (ADR-0002 revision 2), and FTS5's
// external-content delete command needs the row's prior subject and
// search_text to remove its old terms, values the write is about to
// overwrite everywhere else SQLite could still read them from.
//
// A row already present in message carries a message_fts entry by
// the store's own invariant (trg_message_fts_delete assumes it), so
// reindexMessage deletes that entry before inserting id's new one. A
// fresh id has no prior row, so there is nothing to delete: this is
// the insert path, including a message indexed before its body
// arrives and reindexed once the backfill fills search_text.
func reindexMessage(tx *sql.Tx, id int64, subject, searchText string) error {
	var oldSubject, oldSearchText string
	err := tx.QueryRow(`SELECT subject, search_text FROM message WHERE id = ?`, id).Scan(&oldSubject, &oldSearchText)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// fresh insert: nothing indexed yet to remove.
	case err != nil:
		return fmt.Errorf("read message %d for reindex: %w", id, err)
	default:
		if _, err := tx.Exec(`INSERT INTO message_fts(message_fts, rowid, subject, search_text) VALUES ('delete', ?, ?, ?)`,
			id, oldSubject, oldSearchText); err != nil {
			return fmt.Errorf("delete stale fts entry for message %d: %w", id, err)
		}
	}

	if _, err := tx.Exec(`INSERT INTO message_fts(rowid, subject, search_text) VALUES (?, ?, ?)`, id, subject, searchText); err != nil {
		return fmt.Errorf("insert fts entry for message %d: %w", id, err)
	}
	return nil
}

// RebuildIndex regenerates message_fts from message's current rows,
// discarding whatever the index held before. SR-1 makes this a
// derived-state operation: a caller with reason to doubt the index
// (poplar's --rebuild-index flag, a failed integrity-check) runs it
// rather than reconciling term by term.
func RebuildIndex(db *sql.DB) error {
	if _, err := db.Exec(`INSERT INTO message_fts(message_fts) VALUES ('rebuild')`); err != nil {
		return fmt.Errorf("rebuild message_fts: %w", err)
	}
	return nil
}
