// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// OpArgs is the sealed sum of queueable operations.
// Each implementation is JSON-serializable for on-disk storage in
// outbox.args. SendArgs and AppendArgs are placeholders; QueueOp
// rejects them.
type OpArgs interface{ opKind() OpKind }

type (
	MoveArgs    struct{ Dest string } // dest = canonical folder name
	FlagArgs    struct {
		Flag mail.Flag
		Set  bool
	}
	DestroyArgs struct{} // irreversible
	SendArgs    struct{}
	AppendArgs  struct{}
)

func (MoveArgs) opKind() OpKind    { return KindMove }
func (FlagArgs) opKind() OpKind    { return KindFlag }
func (DestroyArgs) opKind() OpKind { return KindDestroy }
func (SendArgs) opKind() OpKind    { return KindSend }
func (AppendArgs) opKind() OpKind  { return KindAppend }

// QueueOp atomically inserts an outbox row and applies the
// optimistic UI flip to the message row. On commit it signals the
// drainer (a no-op when the drainer is not running). When msgUID is
// empty the op is folder-scoped (e.g. send/append); cache I writers
// always pass a non-empty UID for move/flag/destroy.
func (a *Account) QueueOp(ctx context.Context, folder string, msgUID mail.UID, args OpArgs) (int64, error) {
	if args == nil {
		return 0, fmt.Errorf("queue: nil args")
	}
	body, err := json.Marshal(args)
	if err != nil {
		return 0, fmt.Errorf("encode args: %w", err)
	}
	folderID, err := a.folderID(folder)
	if err != nil {
		return 0, err
	}
	var opID int64
	err = a.tx(ctx, func(tx *sql.Tx) error {
		var msgID sql.NullInt64
		if msgUID != "" {
			var id int64
			if err := tx.QueryRow(`SELECT id FROM messages WHERE protocol_id = ?`, string(msgUID)).Scan(&id); err != nil {
				return fmt.Errorf("resolve message %s: %w", msgUID, err)
			}
			msgID = sql.NullInt64{Int64: id, Valid: true}
			if err := applyOptimisticTx(tx, id, args); err != nil {
				return err
			}
		}
		res, err := tx.Exec(`
            INSERT INTO outbox (folder, message, kind, args, enqueued_at, status, attempts, next_eligible_at)
            VALUES (?, ?, ?, ?, ?, ?, 0, NULL)`,
			folderID, msgID, string(args.opKind()), string(body), time.Now().UnixNano(), OpPending)
		if err != nil {
			return fmt.Errorf("insert outbox: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		opID = id
		return nil
	})
	if err != nil {
		return 0, err
	}
	a.signalDrainer()
	return opID, nil
}

// applyOptimisticTx writes the optimistic UI hint for one op against
// one message row. Move/Destroy hide the source; Flag updates ui_flags.
func applyOptimisticTx(tx *sql.Tx, msgID int64, args OpArgs) error {
	switch v := args.(type) {
	case MoveArgs, DestroyArgs:
		_ = v
		_, err := tx.Exec(`UPDATE messages SET ui_hide = 1 WHERE id = ?`, msgID)
		return err
	case FlagArgs:
		bit := uint32(v.Flag)
		stmt := `UPDATE messages SET ui_flags = ui_flags | ? WHERE id = ?`
		if !v.Set {
			stmt = `UPDATE messages SET ui_flags = ui_flags & ~? WHERE id = ?`
		}
		_, err := tx.Exec(stmt, bit, msgID)
		return err
	}
	return nil
}

// outboxRow is the in-memory shape of one outbox entry.
type outboxRow struct {
	ID         int64
	FolderID   int64
	FolderName string
	MessageID  sql.NullInt64
	ProtocolID sql.NullString
	Kind       string
	ArgsJSON   string
	Attempts   int
}

// nextOutboxRow returns the next eligible op or sql.ErrNoRows if
// none. Eligibility = pending, or failed past its next_eligible_at
// window. The outbox_pickup index covers the predicate so the scan
// is O(log n).
func (a *Account) nextOutboxRow(now time.Time) (*outboxRow, error) {
	const q = `
        SELECT o.id, o.folder, f.name, o.message,
               COALESCE((SELECT m.protocol_id FROM messages m WHERE m.id = o.message), ''),
               o.kind, o.args, o.attempts
        FROM outbox o
        JOIN folders f ON f.id = o.folder
        WHERE o.status = ?
           OR (o.status = ? AND (o.next_eligible_at IS NULL OR o.next_eligible_at <= ?))
        ORDER BY o.id LIMIT 1`
	var row outboxRow
	err := a.db.QueryRow(q, OpPending, OpFailed, now.UnixNano()).Scan(
		&row.ID, &row.FolderID, &row.FolderName, &row.MessageID,
		&row.ProtocolID, &row.Kind, &row.ArgsJSON, &row.Attempts)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// markExecuting flips a row to executing and bumps attempts.
func (a *Account) markExecuting(opID int64) error {
	_, err := a.db.Exec(
		`UPDATE outbox SET status = ?, last_attempt = ?, attempts = attempts + 1 WHERE id = ?`,
		OpExecuting, time.Now().UnixNano(), opID)
	return err
}

// finishOp transitions an executing row to a terminal state.
// nextEligibleAt is the unix-nanos backoff deadline for failed ops;
// pass 0 for terminal (done/conflict) transitions.
func (a *Account) finishOp(opID int64, status OpStatus, errPayload string, nextEligibleAt int64) error {
	if nextEligibleAt == 0 {
		_, err := a.db.Exec(
			`UPDATE outbox SET status = ?, error = ?, next_eligible_at = NULL WHERE id = ?`,
			status, errPayload, opID)
		return err
	}
	_, err := a.db.Exec(
		`UPDATE outbox SET status = ?, error = ?, next_eligible_at = ? WHERE id = ?`,
		status, errPayload, nextEligibleAt, opID)
	return err
}
