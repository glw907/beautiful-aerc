package cache

import (
	"context"
	"database/sql"
	"errors"

	"github.com/glw907/poplar/internal/mail"
)

// nextUnfetchedUID returns the newest message UID without a stored
// body, or ok=false when every cached message has bytes. The query
// is the implicit work queue for the backfill worker: sent_at DESC
// puts new mail at the top, eviction restores rows naturally.
func (a *Account) nextUnfetchedUID(ctx context.Context) (mail.UID, bool, error) {
	var pid string
	err := a.db.QueryRowContext(ctx, `
		SELECT m.protocol_id
		FROM messages m
		LEFT JOIN bodies b ON b.message = m.id
		WHERE b.bytes IS NULL
		ORDER BY m.sent_at DESC
		LIMIT 1
	`).Scan(&pid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return mail.UID(pid), true, nil
}
