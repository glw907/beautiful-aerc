// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// OutboxDepth is the per-status count of outbox rows. Feeds the
// status-bar depth segment.
type OutboxDepth struct {
	Pending   int
	Executing int
	Failed    int
	Conflict  int
}

// OutboxGroup is one row of the Q overlay's grouped summary. One
// row per (kind, folder, status) tuple. NextAt carries the earliest
// next_eligible_at within the group; populated only for Failed.
type OutboxGroup struct {
	Kind   OpKind
	Folder string
	Status OpStatus
	Count  int
	NextAt sql.NullInt64
}

// ConflictRow is one row of the ! overlay. ErrorKind/ErrorMessage
// are decoded from the outbox.error JSON payload.
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

// OutboxDepth returns counts grouped by status. Empty outbox returns
// the zero value.
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

// errorPayload is the JSON shape written by encodeErr in drainer.go.
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
