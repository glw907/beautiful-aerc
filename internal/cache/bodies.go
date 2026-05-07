package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

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

// Population path. The header row must already exist. storeBody
// errors if not. Evicts oldest-by-sent-date when maxSize > 0.
func (a *Account) storeBody(ctx context.Context, uid mail.UID, body []byte) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		newSize := int64(len(body))
		// Size backstop: compute total once. Short-circuit if no eviction needed.
		if a.maxSize > 0 {
			var total int64
			if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(length(bytes)), 0) FROM bodies`).Scan(&total); err != nil {
				return fmt.Errorf("store body %s: sum bytes: %w", uid, err)
			}
			if total+newSize > a.maxSize {
				target := a.maxSize - newSize
				if target < 0 {
					target = 0
				}
				if _, _, err := a.evictBySize(ctx, tx, total, target); err != nil {
					return err
				}
			}
		}
		// Single INSERT...SELECT: resolves message FK and upserts in one statement.
		res, err := tx.ExecContext(ctx, `
            INSERT INTO bodies (message, bytes, fetched_at)
            SELECT id, ?, ? FROM messages WHERE protocol_id = ?
            ON CONFLICT(message) DO UPDATE SET
              bytes      = excluded.bytes,
              fetched_at = excluded.fetched_at`,
			body, time.Now().UnixNano(), string(uid))
		if err != nil {
			return fmt.Errorf("store body %s: insert: %w", uid, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("store body %s: unknown message uid", uid)
		}
		return nil
	})
}

// evictBySize removes oldest-by-sent-date bodies until the total body bytes
// are at or below target. total is the current SUM(length(bytes)) passed in
// by the caller (computed before the new insert). Returns the number of rows
// removed and bytes freed. The caller holds an open tx.
func (a *Account) evictBySize(ctx context.Context, tx *sql.Tx, total, target int64) (rows int, freed int64, err error) {
	const batchSize = 32
	const pickQ = `
        SELECT b.message, length(b.bytes)
        FROM bodies b
        JOIN messages m ON m.id = b.message
        ORDER BY m.sent_at ASC NULLS LAST
        LIMIT ?`
	for total > target {
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
			// Nothing left to evict but still over target: the incoming
			// body is oversized. Return what we have. The store proceeds.
			return rows, freed, nil
		}
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM bodies WHERE message IN (`+sqlPlaceholders(len(ids))+`)`, args...); err != nil {
			return rows, freed, fmt.Errorf("evict: delete: %w", err)
		}
		rows += len(ids)
		freed += batchFreed
		total -= batchFreed
	}
	return rows, freed, nil
}

// EvictByAge deletes body rows whose fetched_at is older than cutoff.
// Returns the number of rows removed and total bytes freed. Used by
// the `poplar cache evict --older-than` CLI. Not invoked automatically.
func (a *Account) EvictByAge(ctx context.Context, cutoff time.Time) (rows int, freed int64, err error) {
	err = a.tx(ctx, func(tx *sql.Tx) error {
		rs, err := tx.QueryContext(ctx,
			`DELETE FROM bodies WHERE fetched_at < ? RETURNING length(bytes)`,
			cutoff.UnixNano())
		if err != nil {
			return fmt.Errorf("evict by age: %w", err)
		}
		defer rs.Close()
		for rs.Next() {
			var sz int64
			if err := rs.Scan(&sz); err != nil {
				return fmt.Errorf("evict by age: scan: %w", err)
			}
			freed += sz
			rows++
		}
		return rs.Err()
	})
	return rows, freed, err
}
