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

// OpArgs is the sealed sum of queueable operations. Implementations
// are JSON-encoded into outbox.args. Send/Append/PushDraft carry only
// metadata, with the MIME payload in outbox.payload.
type OpArgs interface{ opKind() OpKind }

type (
	MoveArgs struct{ Dest string }
	FlagArgs struct {
		Flag mail.Flag
		Set  bool
	}
	DestroyArgs struct{}
	SendArgs    struct{ Envelope mail.Envelope }
	AppendArgs  struct{ Flag mail.Flag }
	// PushDraftArgs identifies the local draft and the server-side
	// image (if any) it replaces. A non-empty PrevServerUID makes
	// the backend destroy the prior image atomically, keeping
	// exactly one server copy per draft.
	PushDraftArgs struct {
		DraftID       string
		PrevServerUID mail.UID
	}
	// ContactPutArgs carries the CardDAV addressing for a PUT. BookHref
	// is the collection; Href is the object path; IfMatch guards against
	// mid-air collisions (empty on creates).
	ContactPutArgs struct {
		BookHref string
		Href     string
		IfMatch  string
	}
	// ContactDeleteArgs carries the object path and optional ETag guard
	// for a CardDAV DELETE.
	ContactDeleteArgs struct {
		Href    string
		IfMatch string
	}
)

func (MoveArgs) opKind() OpKind          { return KindMove }
func (FlagArgs) opKind() OpKind          { return KindFlag }
func (DestroyArgs) opKind() OpKind       { return KindDestroy }
func (SendArgs) opKind() OpKind          { return KindSend }
func (AppendArgs) opKind() OpKind        { return KindAppend }
func (PushDraftArgs) opKind() OpKind     { return KindPushDraft }
func (ContactPutArgs) opKind() OpKind    { return KindContactPut }
func (ContactDeleteArgs) opKind() OpKind { return KindContactDelete }

// QueueOp inserts an outbox row and the optimistic UI flip in one
// transaction, then signals the drainer. An empty msgUID makes the
// op folder-scoped (used by send/append/push-draft).
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
// payload. These ops have no message-row state to mirror.
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

// QueueSend enqueues a Send op. sentFolder is informational on JMAP
// and reused as the Append target on IMAP follow-up.
func (a *Account) QueueSend(ctx context.Context, sentFolder string, env mail.Envelope, mime []byte) (int64, error) {
	return a.insertFolderOp(ctx, sentFolder, SendArgs{Envelope: env}, mime)
}

// QueueAppend enqueues an Append op writing mime to folder with flag.
func (a *Account) QueueAppend(ctx context.Context, folder string, flag mail.Flag, mime []byte) (int64, error) {
	return a.insertFolderOp(ctx, folder, AppendArgs{Flag: flag}, mime)
}

// QueuePushDraft enqueues a PushDraft op. A non-empty prevUID makes
// the backend destroy the prior server image in the same op.
func (a *Account) QueuePushDraft(ctx context.Context, draftID, folder string, mime []byte, prevUID mail.UID) (int64, error) {
	return a.insertFolderOp(ctx, folder,
		PushDraftArgs{DraftID: draftID, PrevServerUID: prevUID}, mime)
}

// QueueOutbound enqueues outbound mail through the outbox. JMAP
// places the Sent copy atomically inside Send. IMAP queues a
// separate Append for the Sent copy.
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

// applyOptimisticTx writes the UI hint for one op against one message
// row: Move/Destroy hide the source; Flag updates ui_flags.
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

// nextOutboxRow returns the next eligible op or sql.ErrNoRows.
// Eligibility = pending, or failed past its next_eligible_at window.
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

func (a *Account) markExecuting(opID int64) error {
	_, err := a.db.Exec(
		`UPDATE outbox SET status = ?, last_attempt = ?, attempts = attempts + 1 WHERE id = ?`,
		OpExecuting, time.Now().UnixNano(), opID)
	return err
}

// finishOp transitions an executing row. nextEligibleAt is the
// unix-nanos backoff deadline for failed ops. Pass 0 for done/conflict.
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

// ErrNotConflict is returned by RetryOp/DiscardOp when the row is
// not in conflict. Treat as benign and refresh.
var ErrNotConflict = errors.New("cache: op is not in conflict state")

// revertOptimisticTx mirrors applyOptimisticTx so a discard leaves
// the cache reflecting what the server actually has. Send/Append/
// PushDraft have no message-row state to mirror.
func revertOptimisticTx(tx *sql.Tx, msgID int64, args OpArgs) error {
	switch v := args.(type) {
	case MoveArgs, DestroyArgs:
		_, err := tx.Exec(`UPDATE messages SET ui_hide = 0 WHERE id = ?`, msgID)
		return err
	case FlagArgs:
		bit := uint32(v.Flag)
		stmt := `UPDATE messages SET ui_flags = ui_flags & ~? WHERE id = ?`
		if !v.Set {
			stmt = `UPDATE messages SET ui_flags = ui_flags | ? WHERE id = ?`
		}
		_, err := tx.Exec(stmt, bit, msgID)
		return err
	case SendArgs, AppendArgs, PushDraftArgs, ContactPutArgs, ContactDeleteArgs:
		return nil
	}
	return fmt.Errorf("revertOptimisticTx: unknown args %T", args)
}

// RetryOp requeues a conflicted op with a fresh attempts budget so a
// max-attempts conflict doesn't immediately re-conflict on first
// failure. Returns ErrNotConflict if the row was already resolved.
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
// row in one transaction. Returns ErrNotConflict if not in conflict.
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
