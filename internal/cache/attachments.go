package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// Attachments returns metadata for non-body parts of uid. On cache
// miss the backend is consulted and the rows persisted. Zero-length
// results are not cached, since empty is indistinguishable from
// "not populated yet" without a schema marker. Attachment-free
// messages take a re-fetch on each call.
func (a *Account) Attachments(ctx context.Context, uid mail.UID) ([]mail.Attachment, error) {
	rows, err := a.lookupAttachments(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("attachments %s: lookup: %w", uid, err)
	}
	if len(rows) > 0 {
		return rows, nil
	}
	if !a.Connected() {
		return nil, ErrNotConnected
	}
	atts, err := a.Backend.Attachments(uid)
	if err != nil {
		return nil, err
	}
	if len(atts) == 0 {
		return nil, nil
	}
	if err := a.storeAttachments(ctx, uid, atts); err != nil {
		return nil, fmt.Errorf("attachments %s: store: %w", uid, err)
	}
	return atts, nil
}

func (a *Account) lookupAttachments(ctx context.Context, uid mail.UID) ([]mail.Attachment, error) {
	const q = `
        SELECT a.part_id, a.filename, a.mime_type, a.size, a.content_id, a.disposition
        FROM attachments a
        JOIN messages m ON m.id = a.message
        WHERE m.protocol_id = ?
        ORDER BY a.id ASC`
	rs, err := a.db.QueryContext(ctx, q, string(uid))
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []mail.Attachment
	for rs.Next() {
		var (
			att  mail.Attachment
			disp string
			size int64
		)
		if err := rs.Scan(&att.PartID, &att.Filename, &att.MIMEType, &size, &att.ContentID, &disp); err != nil {
			return nil, err
		}
		att.Size = uint32(size)
		d, err := mail.ParseDisposition(disp)
		if err != nil {
			return nil, fmt.Errorf("attachments %s: invalid disposition %q in row", uid, disp)
		}
		att.Disposition = d
		out = append(out, att)
	}
	return out, rs.Err()
}

// storeAttachments writes the metadata rows for uid. Errors if the
// uid has no row in messages.
func (a *Account) storeAttachments(ctx context.Context, uid mail.UID, atts []mail.Attachment) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		var msgID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM messages WHERE protocol_id = ?`, string(uid)).Scan(&msgID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("store attachments %s: unknown message uid", uid)
		}
		if err != nil {
			return fmt.Errorf("store attachments %s: resolve message: %w", uid, err)
		}
		for _, att := range atts {
			_, err := tx.ExecContext(ctx, `
                INSERT INTO attachments
                  (message, part_id, filename, mime_type, size, content_id, disposition)
                VALUES (?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT (message, part_id) DO UPDATE SET
                  filename    = excluded.filename,
                  mime_type   = excluded.mime_type,
                  size        = excluded.size,
                  content_id  = excluded.content_id,
                  disposition = excluded.disposition`,
				msgID, att.PartID, att.Filename, att.MIMEType, int64(att.Size),
				att.ContentID, att.Disposition.String())
			if err != nil {
				return fmt.Errorf("store attachments %s part %s: %w", uid, att.PartID, err)
			}
		}
		return nil
	})
}

// FetchAttachment returns bytes for partID on uid. Cache miss →
// backend → store under the attachment-size backstop → return.
func (a *Account) FetchAttachment(ctx context.Context, uid mail.UID, partID string) ([]byte, error) {
	if buf, ok, err := a.lookupAttachmentBytes(ctx, uid, partID); err != nil {
		return nil, fmt.Errorf("fetch attachment %s/%s: lookup: %w", uid, partID, err)
	} else if ok {
		return buf, nil
	}
	if !a.Connected() {
		return nil, ErrNotConnected
	}
	a.BeginAttachmentDownload()
	defer a.EndAttachmentDownload()
	body, err := a.Backend.FetchAttachment(uid, partID)
	if err != nil {
		return nil, err
	}
	if storeErr := a.storeAttachmentBytes(ctx, uid, partID, body); storeErr != nil {
		a.log.Warn("cache: storeAttachmentBytes", "uid", uid, "part", partID, "err", storeErr)
	}
	return body, nil
}

// lookupAttachmentBytes misses on absent row or NULL bytes.
func (a *Account) lookupAttachmentBytes(ctx context.Context, uid mail.UID, partID string) ([]byte, bool, error) {
	const q = `
        SELECT a.bytes
        FROM attachments a
        JOIN messages m ON m.id = a.message
        WHERE m.protocol_id = ? AND a.part_id = ?`
	var buf []byte
	err := a.db.QueryRowContext(ctx, q, string(uid), partID).Scan(&buf)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if buf == nil {
		return nil, false, nil
	}
	return buf, true, nil
}

// storeAttachmentBytes writes bytes onto the (uid, partID) row,
// evicting older rows under the attachment-size budget. Call
// Attachments() first to populate the metadata row.
func (a *Account) storeAttachmentBytes(ctx context.Context, uid mail.UID, partID string, body []byte) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		newSize := int64(len(body))
		if a.maxAttachmentSize > 0 {
			var total int64
			if err := tx.QueryRowContext(ctx,
				`SELECT COALESCE(SUM(length(bytes)), 0) FROM attachments WHERE bytes IS NOT NULL`).Scan(&total); err != nil {
				return fmt.Errorf("store attachment %s/%s: sum: %w", uid, partID, err)
			}
			if total+newSize > a.maxAttachmentSize {
				target := a.maxAttachmentSize - newSize
				if target < 0 {
					target = 0
				}
				if _, _, err := a.evictAttachmentBytesBySize(ctx, tx, total, target); err != nil {
					return err
				}
			}
		}
		res, err := tx.ExecContext(ctx, `
            UPDATE attachments
               SET bytes      = ?,
                   fetched_at = ?
             WHERE part_id = ?
               AND message  = (SELECT id FROM messages WHERE protocol_id = ?)`,
			body, time.Now().UnixNano(), partID, string(uid))
		if err != nil {
			return fmt.Errorf("store attachment %s/%s: update: %w", uid, partID, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("store attachment %s/%s: unknown row (call Attachments first)", uid, partID)
		}
		return nil
	})
}

// evictAttachmentBytesBySize clears bytes (keeps metadata) on the
// oldest-by-sent-date rows until total is at or below target.
func (a *Account) evictAttachmentBytesBySize(ctx context.Context, tx *sql.Tx, total, target int64) (rows int, freed int64, err error) {
	const batchSize = 32
	const pickQ = `
        SELECT a.id, length(a.bytes)
        FROM attachments a
        JOIN messages m ON m.id = a.message
        WHERE a.bytes IS NOT NULL
        ORDER BY m.sent_at ASC NULLS LAST
        LIMIT ?`
	for total > target {
		rs, err := tx.QueryContext(ctx, pickQ, batchSize)
		if err != nil {
			return rows, freed, fmt.Errorf("evict attach: pick batch: %w", err)
		}
		var ids []int64
		var batchFreed int64
		remaining := total - target
		for rs.Next() {
			var id, sz int64
			if err := rs.Scan(&id, &sz); err != nil {
				rs.Close()
				return rows, freed, fmt.Errorf("evict attach: scan: %w", err)
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
			return rows, freed, nil
		}
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE attachments SET bytes = NULL, fetched_at = NULL WHERE id IN (`+sqlPlaceholders(len(ids))+`)`,
			args...); err != nil {
			return rows, freed, fmt.Errorf("evict attach: clear: %w", err)
		}
		rows += len(ids)
		freed += batchFreed
		total -= batchFreed
	}
	return rows, freed, nil
}
