package cache

import (
	"context"
	"time"
)

// RescheduleOp updates the scheduled_for of an outbox row that is
// still pending and not yet eligible for pickup. Returns ErrNotPending
// when the row has advanced or already passed its scheduled time.
func (a *Account) RescheduleOp(ctx context.Context, opID int64, newScheduledFor int64) error {
	res, err := a.db.ExecContext(ctx, `
        UPDATE outbox
           SET scheduled_for = ?
         WHERE id = ?
           AND status = ?
           AND scheduled_for IS NOT NULL
           AND scheduled_for > ?`,
		newScheduledFor, opID, OpPending, time.Now().UnixNano())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotPending
	}
	return nil
}
