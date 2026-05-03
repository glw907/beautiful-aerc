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
// If maxSize > 0, evicts oldest-by-sent-date bodies before insert
// so total bytes remain at or below maxSize.
func (a *Account) storeBody(ctx context.Context, uid mail.UID, body []byte) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		var msgID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM messages WHERE protocol_id = ?`, string(uid)).Scan(&msgID); err != nil {
			return fmt.Errorf("store body %s: lookup message: %w", uid, err)
		}
		// Size backstop: if maxSize > 0 and the new body would push
		// total over cap, evict by sent_at ASC until total + new fits.
		if a.maxSize > 0 {
			newSize := int64(len(body))
			target := a.maxSize - newSize
			if target < 0 {
				target = 0
			}
			if _, _, err := a.evictBySize(ctx, tx, target); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `
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

// evictBySize removes oldest-by-sent-date bodies until the total
// body bytes are at or below target. Returns the number of rows
// removed and total bytes freed. The caller holds an open tx.
func (a *Account) evictBySize(ctx context.Context, tx *sql.Tx, target int64) (rows int, freed int64, err error) {
	const batchSize = 32
	for {
		var total int64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(length(bytes)), 0) FROM bodies`).Scan(&total); err != nil {
			return rows, freed, fmt.Errorf("evict: sum bytes: %w", err)
		}
		if total <= target {
			return rows, freed, nil
		}
		// Pick the next batch of oldest-sent body rows to drop.
		const pickQ = `
            SELECT b.message, length(b.bytes)
            FROM bodies b
            JOIN messages m ON m.id = b.message
            ORDER BY COALESCE(m.sent_at, 0) ASC
            LIMIT ?`
		rs, err := tx.QueryContext(ctx, pickQ, batchSize)
		if err != nil {
			return rows, freed, fmt.Errorf("evict: pick batch: %w", err)
		}
		var ids []int64
		var batchFreed int64
		remaining := total - target
		for rs.Next() {
			var id, sz int64
			if err := rs.Scan(&id, &sz); err != nil {
				rs.Close()
				return rows, freed, fmt.Errorf("evict: scan: %w", err)
			}
			ids = append(ids, id)
			batchFreed += sz
			remaining -= sz
			if remaining <= 0 {
				break
			}
		}
		rs.Close()
		if len(ids) == 0 {
			// Nothing left to evict but still over target — caller has
			// an oversized incoming body. Return what we have; the
			// store will still proceed.
			return rows, freed, nil
		}
		// Build IN-clause placeholders.
		args := make([]any, len(ids))
		ph := make([]byte, 0, len(ids)*2-1)
		for i, id := range ids {
			if i > 0 {
				ph = append(ph, ',')
			}
			ph = append(ph, '?')
			args[i] = id
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM bodies WHERE message IN (`+string(ph)+`)`, args...); err != nil {
			return rows, freed, fmt.Errorf("evict: delete: %w", err)
		}
		rows += len(ids)
		freed += batchFreed
	}
}
