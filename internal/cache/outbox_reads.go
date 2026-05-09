package cache

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// OutboxDepth counts outbox rows by status, feeding the status bar.
type OutboxDepth struct {
	Pending   int
	Executing int
	Failed    int
	Conflict  int
}

// OutboxGroup is one row of the Q overlay's grouped summary, keyed
// by (kind, folder, status). NextAt holds the earliest next_eligible_at
// within the group and is populated only for Failed rows.
type OutboxGroup struct {
	Kind   OpKind
	Folder string
	Status OpStatus
	Count  int
	NextAt sql.NullInt64
}

// ConflictRow is one row of the ! overlay. ErrorKind and ErrorMessage
// come from the outbox.error JSON payload.
type ConflictRow struct {
	ID           int64
	Kind         OpKind
	Folder       string
	ProtocolID   string
	ErrorKind    string
	ErrorMessage string
	Attempts     int
	EnqueuedAt   time.Time
}

// OutboxDepth returns counts grouped by status.
func (a *Account) OutboxDepth(ctx context.Context) (OutboxDepth, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM outbox GROUP BY status`)
	if err != nil {
		return OutboxDepth{}, fmt.Errorf("outbox depth: %w", err)
	}
	defer rows.Close()
	var d OutboxDepth
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return OutboxDepth{}, err
		}
		switch OpStatus(status) {
		case OpPending:
			d.Pending = n
		case OpExecuting:
			d.Executing = n
		case OpFailed:
			d.Failed = n
		case OpConflict:
			d.Conflict = n
		}
	}
	return d, rows.Err()
}

// OutboxSummary returns one OutboxGroup per (kind, folder, status),
// ordered by status (executing, pending, failed, conflict), then
// kind, then folder. Folder is the Move destination from args.Dest
// and is empty for other kinds.
func (a *Account) OutboxSummary(ctx context.Context) ([]OutboxGroup, error) {
	const q = `
        SELECT o.kind,
               CASE WHEN o.kind = 'move'
                    THEN COALESCE(json_extract(o.args, '$.Dest'), '')
                    ELSE '' END AS folder,
               o.status, COUNT(*),
               MIN(o.next_eligible_at)
        FROM outbox o
        GROUP BY o.kind, folder, o.status
        ORDER BY
          CASE o.status
            WHEN 'executing' THEN 0
            WHEN 'pending'   THEN 1
            WHEN 'failed'    THEN 2
            WHEN 'conflict'  THEN 3
            ELSE 4
          END,
          o.kind,
          folder`
	rows, err := a.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("outbox summary: %w", err)
	}
	defer rows.Close()
	var out []OutboxGroup
	for rows.Next() {
		var g OutboxGroup
		var kind, status string
		if err := rows.Scan(&kind, &g.Folder, &status, &g.Count, &g.NextAt); err != nil {
			return nil, err
		}
		g.Kind = OpKind(kind)
		g.Status = OpStatus(status)
		out = append(out, g)
	}
	return out, rows.Err()
}

// OutboxConflicts returns conflict-state rows oldest-first. The
// outbox.error JSON written by encodeErr is decoded into
// ErrorKind/ErrorMessage for the UI.
func (a *Account) OutboxConflicts(ctx context.Context) ([]ConflictRow, error) {
	const q = `
        SELECT o.id, o.kind, COALESCE(f.name, ''),
               COALESCE((SELECT m.protocol_id FROM messages m WHERE m.id = o.message), ''),
               COALESCE(o.error, ''),
               o.attempts,
               o.enqueued_at
        FROM outbox o
        LEFT JOIN folders f ON f.id = o.folder
        WHERE o.status = ?
        ORDER BY o.enqueued_at ASC, o.id ASC`
	rows, err := a.db.QueryContext(ctx, q, OpConflict)
	if err != nil {
		return nil, fmt.Errorf("outbox conflicts: %w", err)
	}
	defer rows.Close()
	var out []ConflictRow
	for rows.Next() {
		var r ConflictRow
		var kind, errPayload string
		var enqueuedNS int64
		if err := rows.Scan(&r.ID, &kind, &r.Folder, &r.ProtocolID,
			&errPayload, &r.Attempts, &enqueuedNS); err != nil {
			return nil, err
		}
		r.Kind = OpKind(kind)
		r.ErrorKind, r.ErrorMessage = decodeErrorPayload(errPayload)
		r.EnqueuedAt = time.Unix(0, enqueuedNS)
		out = append(out, r)
	}
	return out, rows.Err()
}

// errorPayload mirrors the JSON shape encodeErr writes in drainer.go.
type errorPayload struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func decodeErrorPayload(s string) (kind, msg string) {
	if s == "" {
		return "", ""
	}
	var p errorPayload
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return "unknown", s
	}
	return p.Kind, p.Message
}

// OutboxRow is one entry in the user-facing scheduled-outbox view.
type OutboxRow struct {
	ID           int64
	Kind         OpKind
	Folder       string
	To           []string
	Subject      string
	ScheduledFor time.Time
	Status       OpStatus
	Attempts     int
	LastError    string
	Draft        *DraftRow
}

// OutboxScheduled returns pending and failed outbox rows ordered by
// scheduled_for ascending. Rows with NULL scheduled_for sort last.
// Linked draft rows are joined via outbox.draft_id.
func (a *Account) OutboxScheduled(ctx context.Context) ([]OutboxRow, error) {
	const q = `
        SELECT o.id, o.kind, COALESCE(f.name, ''),
               o.args, o.payload,
               COALESCE(o.scheduled_for, 0),
               o.status, o.attempts, COALESCE(o.error, ''),
               d.draft_id, COALESCE(d.server_uid, ''), COALESCE(d.server_folder, ''),
               d.payload, d.dirty, d.created_at, d.updated_at,
               COALESCE(d.last_pushed_at, 0)
          FROM outbox o
          LEFT JOIN folders f ON f.id = o.folder
          LEFT JOIN drafts d  ON d.draft_id = o.draft_id
         WHERE o.status IN (?, ?)
         ORDER BY CASE WHEN o.scheduled_for IS NULL THEN 1 ELSE 0 END,
                  o.scheduled_for ASC,
                  o.id ASC`
	rows, err := a.db.QueryContext(ctx, q, OpPending, OpFailed)
	if err != nil {
		return nil, fmt.Errorf("outbox scheduled: %w", err)
	}
	defer rows.Close()

	var out []OutboxRow
	for rows.Next() {
		var r OutboxRow
		var kind, status, errPayload string
		var argsJSON, payload []byte
		var scheduledNS int64
		var draftID, serverUID, serverFolder sql.NullString
		var draftPayload []byte
		var dirty, created, updated, pushed sql.NullInt64
		if err := rows.Scan(&r.ID, &kind, &r.Folder, &argsJSON, &payload,
			&scheduledNS, &status, &r.Attempts, &errPayload,
			&draftID, &serverUID, &serverFolder, &draftPayload,
			&dirty, &created, &updated, &pushed); err != nil {
			return nil, err
		}
		r.Kind = OpKind(kind)
		r.Status = OpStatus(status)
		_, r.LastError = decodeErrorPayload(errPayload)
		if scheduledNS != 0 {
			r.ScheduledFor = time.Unix(0, scheduledNS)
		}
		r.To, r.Subject = decodeRowMeta(r.Kind, argsJSON, payload)
		if draftID.Valid {
			d := &DraftRow{
				DraftID:      draftID.String,
				ServerUID:    mail.UID(serverUID.String),
				ServerFolder: serverFolder.String,
				Payload:      draftPayload,
				Dirty:        dirty.Int64 != 0,
			}
			if created.Valid {
				d.CreatedAt = time.Unix(0, created.Int64)
			}
			if updated.Valid {
				d.UpdatedAt = time.Unix(0, updated.Int64)
			}
			if pushed.Valid && pushed.Int64 != 0 {
				d.LastPushedAt = time.Unix(0, pushed.Int64)
			}
			r.Draft = d
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// decodeRowMeta extracts To addresses from args JSON (Send only) and
// the Subject header from the first 4 KB of payload.
func decodeRowMeta(kind OpKind, argsJSON, payload []byte) (to []string, subject string) {
	if kind == KindSend {
		var sa SendArgs
		if err := json.Unmarshal(argsJSON, &sa); err == nil {
			to = sa.Envelope.Rcpts
		}
	}
	subject = extractSubject(payload)
	return
}

// extractSubject reads the Subject header from the first 4 KB of a
// MIME payload. Folded continuation lines are joined automatically
// by net/textproto.
func extractSubject(payload []byte) string {
	const headerCap = 4096
	if len(payload) > headerCap {
		payload = payload[:headerCap]
	}
	br := bufio.NewReader(bytes.NewReader(payload))
	tp := textproto.NewReader(br)
	hdr, err := tp.ReadMIMEHeader()
	if err != nil && err != io.EOF {
		return ""
	}
	return hdr.Get("Subject")
}
