package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNotPending is returned by CancelOps when at least one named row has
// already left OpPending. No row is deleted on this error.
var ErrNotPending = errors.New("cache: at least one op is not pending")

// CancelOps deletes the named outbox rows iff every one is in OpPending.
// Used by the undo-send window to revoke a not-yet-dispatched outbound.
// Linked draft rows are not touched: the caller relies on the draft
// staying available for compose-restore. Empty input is a no-op.
func (a *Account) CancelOps(ctx context.Context, opIDs []int64) error {
	if len(opIDs) == 0 {
		return nil
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ph := strings.Repeat("?,", len(opIDs))
	ph = ph[:len(ph)-1]
	args := make([]any, len(opIDs))
	for i, id := range opIDs {
		args[i] = id
	}

	rows, err := tx.QueryContext(ctx,
		fmt.Sprintf(`SELECT id, status FROM outbox WHERE id IN (%s)`, ph),
		args...)
	if err != nil {
		return err
	}

	seen := 0
	for rows.Next() {
		var id int64
		var status OpStatus
		if err := rows.Scan(&id, &status); err != nil {
			rows.Close()
			return err
		}
		if status != OpPending {
			rows.Close()
			return ErrNotPending
		}
		seen++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if seen != len(opIDs) {
		// Some IDs were not found; treat as not-pending so the caller
		// knows the op set is no longer fully under its control.
		return ErrNotPending
	}

	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM outbox WHERE id IN (%s)`, ph),
		args...); err != nil {
		return err
	}
	return tx.Commit()
}
