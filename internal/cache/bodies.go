// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// lookupBody reads a cached body for uid. Returns (bytes, true, nil)
// on hit, (nil, false, nil) on miss, (nil, false, err) on db error.
// No last_accessed update — Cache II is lazy-population only.
func (a *Account) lookupBody(ctx context.Context, uid mail.UID) ([]byte, bool, error) {
	const q = `
        SELECT b.bytes
        FROM bodies b
        JOIN messages m ON m.id = b.message
        WHERE m.protocol_id = ?`
	var buf []byte
	err := a.db.QueryRowContext(ctx, q, string(uid)).Scan(&buf)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("lookup body %s: %w", uid, err)
	}
	return buf, true, nil
}

// storeBody writes body bytes into the bodies table for uid. The
// caller has already cache-missed; this is the population path.
// Returns an error if uid has no row in messages (caller bug — the
// header row is established by SyncFolder/upsertMessages first).
//
// Cache II policy: no automatic eviction here. Phase 4 wires the
// max-size backstop into this same path.
func (a *Account) storeBody(ctx context.Context, uid mail.UID, body []byte) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		var msgID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM messages WHERE protocol_id = ?`, string(uid)).Scan(&msgID)
		if err != nil {
			return fmt.Errorf("store body %s: lookup message: %w", uid, err)
		}
		_, err = tx.ExecContext(ctx, `
            INSERT INTO bodies (message, bytes, fetched_at) VALUES (?, ?, ?)
            ON CONFLICT(message) DO UPDATE SET
              bytes      = excluded.bytes,
              fetched_at = excluded.fetched_at`,
			msgID, body, time.Now().UnixNano())
		if err != nil {
			return fmt.Errorf("store body %s: insert: %w", uid, err)
		}
		return nil
	})
}
