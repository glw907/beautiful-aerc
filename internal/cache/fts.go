package cache

import (
	"context"
	"database/sql"

	"github.com/glw907/poplar/internal/mail"
)

// writeFTSHeadersTx rewrites the messages_fts row for msgID with current
// header values, preserving any body text already indexed. FTS5
// contentless tables don't support UPSERT or column-level UPDATE, so we
// DELETE+INSERT inside the caller's transaction.
func writeFTSHeadersTx(ctx context.Context, tx *sql.Tx, msgID int64, m *mail.MessageInfo) error {
	var body sql.NullString
	row := tx.QueryRowContext(ctx, `SELECT body FROM messages_fts WHERE rowid = ?`, msgID)
	if err := row.Scan(&body); err != nil && err != sql.ErrNoRows {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM messages_fts WHERE rowid = ?`, msgID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
        INSERT INTO messages_fts(rowid, subject, from_addr, to_addr, cc_addr, body)
        VALUES (?, ?, ?, ?, ?, ?)`,
		msgID, m.Subject, m.From, m.To, m.Cc, body.String)
	return err
}

// writeFTSBodyTx swaps the body column on the messages_fts row for
// msgID, preserving header columns. FTS5 doesn't support column-level
// UPDATE on this table shape. The missing-row branch covers older
// rows pre-dating v11.
func writeFTSBodyTx(ctx context.Context, tx *sql.Tx, msgID int64, body string) error {
	var subject, from, to, cc sql.NullString
	row := tx.QueryRowContext(ctx,
		`SELECT subject, from_addr, to_addr, cc_addr FROM messages_fts WHERE rowid = ?`, msgID)
	switch err := row.Scan(&subject, &from, &to, &cc); err {
	case nil:
		if _, err := tx.ExecContext(ctx, `DELETE FROM messages_fts WHERE rowid = ?`, msgID); err != nil {
			return err
		}
	case sql.ErrNoRows:
		// Fall through with empty headers.
	default:
		return err
	}
	_, err := tx.ExecContext(ctx, `
        INSERT INTO messages_fts(rowid, subject, from_addr, to_addr, cc_addr, body)
        VALUES (?, ?, ?, ?, ?, ?)`,
		msgID, subject.String, from.String, to.String, cc.String, body)
	return err
}
