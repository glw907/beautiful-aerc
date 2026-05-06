// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// DraftRow is the in-memory shape of one drafts row. ServerUID is
// empty until the first successful PushDraft op. LastPushedAt is the
// zero time until then.
type DraftRow struct {
	DraftID      string
	ServerUID    mail.UID
	ServerFolder string
	Payload      []byte
	Dirty        bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastPushedAt time.Time
}

// UpsertDraft writes payload for draftID, marking dirty and bumping
// updated_at. Creates the row on first call. Last writer wins.
// Caller is the compose autosave timer.
func (a *Account) UpsertDraft(ctx context.Context, draftID string, payload []byte) error {
	if draftID == "" {
		return fmt.Errorf("upsert draft: empty draftID")
	}
	now := time.Now().UnixNano()
	return a.tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`
            INSERT INTO drafts (draft_id, payload, dirty, created_at, updated_at)
            VALUES (?, ?, 1, ?, ?)
            ON CONFLICT(draft_id) DO UPDATE SET
                payload    = excluded.payload,
                dirty      = 1,
                updated_at = excluded.updated_at`,
			draftID, payload, now, now)
		return err
	})
}

// LoadDraft returns the payload for draftID, or sql.ErrNoRows when absent.
func (a *Account) LoadDraft(ctx context.Context, draftID string) ([]byte, error) {
	var payload []byte
	err := a.db.QueryRowContext(ctx,
		`SELECT payload FROM drafts WHERE draft_id = ?`, draftID).Scan(&payload)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

// ListDrafts returns every drafts row, oldest-first by created_at.
// The App reads these to project local-only drafts into the
// Drafts-folder message-list view.
func (a *Account) ListDrafts(ctx context.Context) ([]DraftRow, error) {
	rows, err := a.db.QueryContext(ctx, `
        SELECT draft_id, COALESCE(server_uid, ''), COALESCE(server_folder, ''),
               payload, dirty, created_at, updated_at,
               COALESCE(last_pushed_at, 0)
        FROM drafts
        ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DraftRow
	for rows.Next() {
		var r DraftRow
		var serverUID, serverFolder string
		var dirtyInt int
		var created, updated, pushed int64
		if err := rows.Scan(&r.DraftID, &serverUID, &serverFolder,
			&r.Payload, &dirtyInt, &created, &updated, &pushed); err != nil {
			return nil, err
		}
		r.ServerUID = mail.UID(serverUID)
		r.ServerFolder = serverFolder
		r.Dirty = dirtyInt != 0
		r.CreatedAt = time.Unix(0, created)
		r.UpdatedAt = time.Unix(0, updated)
		if pushed != 0 {
			r.LastPushedAt = time.Unix(0, pushed)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteDraft removes the local row. Caller is responsible for
// queueing a Destroy op against any server image in the same logical
// flow (handled by the App's Discard / Send paths).
func (a *Account) DeleteDraft(ctx context.Context, draftID string) error {
	_, err := a.db.ExecContext(ctx, `DELETE FROM drafts WHERE draft_id = ?`, draftID)
	return err
}

// MarkDraftPushed records a successful PushDraft. The drainer calls
// this in its post-success path; clears dirty and records the server
// coordinates.
func (a *Account) MarkDraftPushed(ctx context.Context, draftID string, serverUID mail.UID, serverFolder string) error {
	_, err := a.db.ExecContext(ctx, `
        UPDATE drafts
        SET server_uid     = ?,
            server_folder  = ?,
            dirty          = 0,
            last_pushed_at = ?
        WHERE draft_id = ?`,
		string(serverUID), serverFolder, time.Now().UnixNano(), draftID)
	return err
}
