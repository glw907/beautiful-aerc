package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
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
