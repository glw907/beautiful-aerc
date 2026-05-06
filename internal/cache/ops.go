// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// OpArgs is the sealed sum of queueable operations. Each
// implementation is JSON-serializable for on-disk storage in
// outbox.args. Send/Append carry only metadata; their MIME
// payload lives in outbox.payload.
type OpArgs interface{ opKind() OpKind }

type (
	MoveArgs struct{ Dest string } // dest = canonical folder name
	FlagArgs struct {
		Flag mail.Flag
		Set  bool
	}
	DestroyArgs struct{} // irreversible
	// SendArgs carries the SMTP-level envelope. The MIME body
	// lives in outbox.payload. The destination Sent folder is the
	// outbox row's folder (informational on JMAP, target on IMAP).
	SendArgs struct{ Envelope mail.Envelope }
	// AppendArgs carries the IMAP APPEND flags. The destination
	// folder is the outbox row's folder. The MIME body lives in
	// outbox.payload.
	AppendArgs struct{ Flag mail.Flag }
	// PushDraftArgs carries the local draft identifier and the
	// server UID of the previous server-side image. The assembled
	// MIME lives in outbox.payload. When PrevServerUID is non-empty
	// the backend destroys the prior image as part of the same op,
	// keeping exactly one server copy per draft.
	PushDraftArgs struct {
		DraftID       string
		PrevServerUID mail.UID
	}
)

func (MoveArgs) opKind() OpKind      { return KindMove }
func (FlagArgs) opKind() OpKind      { return KindFlag }
func (DestroyArgs) opKind() OpKind   { return KindDestroy }
func (SendArgs) opKind() OpKind      { return KindSend }
func (AppendArgs) opKind() OpKind    { return KindAppend }
func (PushDraftArgs) opKind() OpKind { return KindPushDraft }

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

// insertFolderOp inserts a folder-scoped outbox row carrying a MIME
// payload. Shared by QueueSend and QueueAppend. These ops have no
// message-row state to mirror, so there is no optimistic UI flip.
func (a *Account) insertFolderOp(ctx context.Context, folder string, args OpArgs, payload []byte) (int64, error) {
	if args == nil {
		return 0, fmt.Errorf("queue: nil args")
	}
	if len(payload) == 0 {
		return 0, fmt.Errorf("queue: empty payload for %s", args.opKind())
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
		res, err := tx.Exec(`
            INSERT INTO outbox (folder, message, kind, args, payload, enqueued_at, status, attempts, next_eligible_at)
            VALUES (?, NULL, ?, ?, ?, ?, ?, 0, NULL)`,
			folderID, string(args.opKind()), string(body), payload,
			time.Now().UnixNano(), OpPending)
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

// QueueSend enqueues a Send op carrying mime as the assembled
// payload. sentFolder names the canonical Sent folder for the
// account (informational on JMAP, the IMAP target Append will
// reuse on follow-up). Drainer dispatch calls Backend.Send.
func (a *Account) QueueSend(ctx context.Context, sentFolder string, env mail.Envelope, mime []byte) (int64, error) {
	return a.insertFolderOp(ctx, sentFolder, SendArgs{Envelope: env}, mime)
}

// QueueAppend enqueues an Append op writing mime to folder with
// flag. Drainer dispatch calls Backend.Append.
func (a *Account) QueueAppend(ctx context.Context, folder string, flag mail.Flag, mime []byte) (int64, error) {
	return a.insertFolderOp(ctx, folder, AppendArgs{Flag: flag}, mime)
}

// QueuePushDraft enqueues a PushDraft op carrying mime as the
// assembled payload. prevUID is empty on first push and holds the
// prior server_uid on subsequent pushes. The backend destroys the
// prior image as part of the same op when prevUID is non-empty.
func (a *Account) QueuePushDraft(ctx context.Context, draftID, folder string, mime []byte, prevUID mail.UID) (int64, error) {
	return a.insertFolderOp(ctx, folder,
		PushDraftArgs{DraftID: draftID, PrevServerUID: prevUID}, mime)
}

// QueueOutbound enqueues outbound mail through the outbox. JMAP
// backends place the Sent copy atomically inside Send, so one op
// suffices. IMAP requires a separate Append for the Sent copy,
// so two ops are queued in order.
func (a *Account) QueueOutbound(ctx context.Context, sentFolder string, env mail.Envelope, mime []byte) error {
	if _, err := a.QueueSend(ctx, sentFolder, env, mime); err != nil {
		return err
	}
	if a.Backend.IsJMAP() {
		return nil
	}
	_, err := a.QueueAppend(ctx, sentFolder, mail.FlagSeen, mime)
	return err
}

// applyOptimisticTx writes the optimistic UI hint for one op against
// one message row. Move/Destroy hide the source; Flag updates ui_flags.
func applyOptimisticTx(tx *sql.Tx, msgID int64, args OpArgs) error {
	switch v := args.(type) {
	case MoveArgs, DestroyArgs:
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
	Payload    []byte
}

// nextOutboxRow returns the next eligible op or sql.ErrNoRows if
// none. Eligibility = pending, or failed past its next_eligible_at
// window. The outbox_pickup index covers the predicate so the scan
// is O(log n).
func (a *Account) nextOutboxRow(now time.Time) (*outboxRow, error) {
	const q = `
        SELECT o.id, o.folder, f.name, o.message,
               COALESCE((SELECT m.protocol_id FROM messages m WHERE m.id = o.message), ''),
               o.kind, o.args, o.attempts, o.payload
        FROM outbox o
        JOIN folders f ON f.id = o.folder
        WHERE o.status = ?
           OR (o.status = ? AND (o.next_eligible_at IS NULL OR o.next_eligible_at <= ?))
        ORDER BY o.id LIMIT 1`
	var row outboxRow
	err := a.db.QueryRow(q, OpPending, OpFailed, now.UnixNano()).Scan(
		&row.ID, &row.FolderID, &row.FolderName, &row.MessageID,
		&row.ProtocolID, &row.Kind, &row.ArgsJSON, &row.Attempts, &row.Payload)
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

// ErrNotConflict is returned by RetryOp/DiscardOp when the targeted
// op is not in the conflict state. UI callers treat this as benign:
// the row was resolved by some other path. Refresh and continue.
var ErrNotConflict = errors.New("cache: op is not in conflict state")

// revertOptimisticTx mirrors applyOptimisticTx: it undoes the UI flip
// applied at QueueOp time so a discard leaves the cache reflecting
// what the server actually has. Send and Append have no message-row
// state to mirror, so the discard simply deletes the outbox row.
func revertOptimisticTx(tx *sql.Tx, msgID int64, args OpArgs) error {
	switch v := args.(type) {
	case MoveArgs, DestroyArgs:
		_, err := tx.Exec(`UPDATE messages SET ui_hide = 0 WHERE id = ?`, msgID)
		return err
	case FlagArgs:
		bit := uint32(v.Flag)
		// Revert: if the forward op set the flag, clear it. Otherwise set it.
		stmt := `UPDATE messages SET ui_flags = ui_flags & ~? WHERE id = ?`
		if !v.Set {
			stmt = `UPDATE messages SET ui_flags = ui_flags | ? WHERE id = ?`
		}
		_, err := tx.Exec(stmt, bit, msgID)
		return err
	case SendArgs, AppendArgs, PushDraftArgs:
		return nil
	}
	return fmt.Errorf("revertOptimisticTx: unknown args %T", args)
}

// RetryOp transitions a conflicted op back to pending and signals
// the drainer. attempts resets to 0: user-initiated retry grants
// a fresh budget so an auth-failure with attempts >= max does not
// re-enter conflict on the very next failure.
//
// Returns ErrNotConflict if the row is not currently in the conflict
// state (treat as benign: someone resolved it via another path).
func (a *Account) RetryOp(ctx context.Context, opID int64) error {
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var status string
		if err := tx.QueryRow(`SELECT status FROM outbox WHERE id = ?`, opID).Scan(&status); err != nil {
			return fmt.Errorf("retry: lookup op %d: %v", opID, err)
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
			return fmt.Errorf("retry: requeue op %d: %v", opID, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	a.signalDrainer()
	return nil
}

// DiscardOp reverts the optimistic UI flip and deletes the outbox
// row in one transaction. The conflicted op never reached the server,
// so only local cleanup is needed. Send and Append rows have no
// message-row state to revert. Their discard is a straight delete.
//
// Returns ErrNotConflict if the row is not currently in conflict.
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
