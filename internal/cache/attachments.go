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

// Attachments returns metadata for non-body parts of uid. On cache
// miss the backend is consulted and the rows persisted. On cache
// hit no backend roundtrip occurs.
//
// A zero-length result is not cached: empty cannot be distinguished
// from "not yet populated" without a marker, and the cost of an
// occasional re-fetch on truly attachment-free messages is lower
// than a schema column for the marker.
func (a *Account) Attachments(ctx context.Context, uid mail.UID) ([]mail.Attachment, error) {
	rows, err := a.lookupAttachments(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("attachments %s: lookup: %w", uid, err)
	}
	if len(rows) > 0 {
		return rows, nil
	}
	if a.Backend == nil {
		return nil, errors.New("cache: no backend")
	}
	atts, err := a.Backend.Attachments(uid)
	if err != nil {
		return nil, err
	}
	if len(atts) == 0 {
		return nil, nil
	}
	if err := a.storeAttachments(ctx, uid, atts); err != nil {
		// Store failure is non-fatal: caller still has valid metadata.
		_ = err
	}
	return atts, nil
}

// lookupAttachments returns cached metadata rows for uid, ordered by
// id. Empty slice on miss; never returns nil error with non-nil rows
// on a true miss.
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

// storeAttachments writes metadata rows for uid. Caller has already
// determined the cache was empty; this is the populate path. Errors
// if uid has no row in messages.
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

// time import retained for FetchAttachment in Task 7.
var _ = time.Now
