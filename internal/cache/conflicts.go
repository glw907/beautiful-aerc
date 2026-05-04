// SPDX-License-Identifier: MIT

package cache

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotConflict is returned by RetryOp/DiscardOp when the targeted
// op is not in the conflict state. UI callers treat this as benign:
// the row was resolved by some other path; refresh and continue.
var ErrNotConflict = errors.New("cache: op is not in conflict state")

// revertOptimisticTx is the local-state mirror of applyOptimisticTx.
// It reverses the optimistic UI flip applied at QueueOp time so that
// after a Discard the cache reflects what the server actually has.
// SendArgs/AppendArgs (Pass 9) return an error — those op kinds carry
// no optimistic UI state in the cache I schema.
func revertOptimisticTx(tx *sql.Tx, msgID int64, args OpArgs) error {
	switch v := args.(type) {
	case MoveArgs, DestroyArgs:
		_ = v
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
