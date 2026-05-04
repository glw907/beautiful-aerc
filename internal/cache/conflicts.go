// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotConflict is returned by RetryOp/DiscardOp when the targeted
// op is not in the conflict state. UI callers treat this as benign:
// the row was resolved by some other path; refresh and continue.
var ErrNotConflict = errors.New("cache: op is not in conflict state")

// revertOptimisticTx mirrors applyOptimisticTx: it undoes the UI flip
// applied at QueueOp time so a discard leaves the cache reflecting
// what the server actually has. SendArgs and AppendArgs are placeholder
// op kinds with no optimistic UI state, so they error out.
func revertOptimisticTx(tx *sql.Tx, msgID int64, args OpArgs) error {
	switch v := args.(type) {
	case MoveArgs, DestroyArgs:
		_, err := tx.Exec(`UPDATE messages SET ui_hide = 0 WHERE id = ?`, msgID)
		return err
	case FlagArgs:
		bit := uint32(v.Flag)
		// Revert: if forward was Set, clear; if forward was clear, set.
		stmt := `UPDATE messages SET ui_flags = ui_flags & ~? WHERE id = ?`
		if !v.Set {
			stmt = `UPDATE messages SET ui_flags = ui_flags | ? WHERE id = ?`
		}
		_, err := tx.Exec(stmt, bit, msgID)
		return err
	case SendArgs, AppendArgs:
		return fmt.Errorf("revertOptimisticTx: %T not supported", args)
	}
	return fmt.Errorf("revertOptimisticTx: unknown args %T", args)
}

// RetryOp transitions a conflicted op back to pending and signals
// the drainer. attempts is reset to 0 — user-initiated retry grants
// a fresh budget so an auth-failure with attempts >= max doesn't
// re-enter conflict on the very next failure.
//
// Returns ErrNotConflict if the row is not currently in the conflict
// state (treat as benign: someone resolved it via another path).
func (a *Account) RetryOp(ctx context.Context, opID int64) error {
	var signal bool
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRow(`SELECT status FROM outbox WHERE id = ?`, opID).Scan(&status); err != nil {
			return fmt.Errorf("retry: read status: %w", err)
		}
		if OpStatus(status) != OpConflict {
			return ErrNotConflict
		}
		_, err := tx.Exec(`
            UPDATE outbox
            SET status = ?, attempts = 0, next_eligible_at = NULL, error = ''
            WHERE id = ?`,
			OpPending, opID)
		if err != nil {
			return fmt.Errorf("retry: update: %w", err)
		}
		signal = true
		return nil
	})
	if err != nil {
		return err
	}
	if signal {
		a.signalDrainer()
	}
	return nil
}

// DiscardOp reverts the optimistic UI flip and deletes the outbox
// row, all in one transaction. The conflicted op never reached the
// server, so no remote reversal is needed — only local cleanup.
//
// Returns ErrNotConflict if the row is not currently in conflict.
// Send and Append placeholders cannot be discarded via this path —
// revertOptimisticTx has no semantics for them.
func (a *Account) DiscardOp(ctx context.Context, opID int64) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		var status, kind, argsJSON string
		var msgID sql.NullInt64
		err := tx.QueryRow(
			`SELECT status, kind, args, message FROM outbox WHERE id = ?`, opID).
			Scan(&status, &kind, &argsJSON, &msgID)
		if err != nil {
			return fmt.Errorf("discard: read row: %w", err)
		}
		if OpStatus(status) != OpConflict {
			return ErrNotConflict
		}
		args, err := decodeArgs(kind, argsJSON)
		if err != nil {
			return fmt.Errorf("discard: decode args: %w", err)
		}
		if msgID.Valid {
			if err := revertOptimisticTx(tx, msgID.Int64, args); err != nil {
				return fmt.Errorf("discard: revert: %w", err)
			}
		}
		if _, err := tx.Exec(`DELETE FROM outbox WHERE id = ?`, opID); err != nil {
			return fmt.Errorf("discard: delete: %w", err)
		}
		return nil
	})
}
