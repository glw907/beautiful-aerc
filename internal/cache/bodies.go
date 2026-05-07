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

// storeBody upserts a body row. The header row must already exist.
// Evicts oldest-by-sent-date when maxSize > 0.
func (a *Account) storeBody(ctx context.Context, uid mail.UID, body []byte) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		newSize := int64(len(body))
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

// evictBySize removes oldest-by-sent-date bodies until the body
// total drops to target. total is the caller's pre-insert SUM(bytes).
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
			// Incoming body is oversized. The store proceeds with what we have.
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

// EvictByAge deletes body rows whose fetched_at is older than
// cutoff. Used by `poplar cache evict --older-than`. Not automatic.
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
