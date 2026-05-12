package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/contacts"
	"github.com/glw907/poplar/internal/mail"
)

// expBackoff returns initial * 2^(attempts-1) capped at max.
// attempts <= 1 returns initial.
func expBackoff(attempts int, initial, max time.Duration) time.Duration {
	if attempts <= 1 {
		return initial
	}
	d := initial << (attempts - 1)
	if d > max {
		return max
	}
	return d
}

// drainerConfig governs backoff and retry caps.
type drainerConfig struct {
	BackoffMin  time.Duration
	BackoffMax  time.Duration
	MaxAttempts int
	// Idle is the fallback poll cadence. drainSignal handles wakes
	// on QueueOp. The ticker only re-checks failed-row backoff
	// windows when no signal arrives.
	Idle time.Duration
}

func defaultDrainerConfig() drainerConfig {
	return drainerConfig{
		BackoffMin:  1 * time.Second,
		BackoffMax:  60 * time.Second,
		MaxAttempts: 10,
		Idle:        5 * time.Second,
	}
}

// StartDrainer launches the per-account outbox drainer. Crash-
// recovered `executing` rows are reset before the loop starts.
// The goroutine exits when Close is called.
func (a *Account) StartDrainer(ctx context.Context) error {
	if err := a.recoverExecuting(); err != nil {
		return fmt.Errorf("recover executing: %w", err)
	}
	cfg := defaultDrainerConfig()
	a.wg.Add(1)
	go a.drainLoop(ctx, cfg)
	return nil
}

// recoverExecuting resets `executing` rows after a crash. Idempotent
// kinds go back to pending. Non-idempotent kinds (send, append,
// push-draft) land in conflict with crashed-mid-execute.
// contact-put and contact-delete are idempotent (PUT/DELETE are safe
// to replay).
func (a *Account) recoverExecuting() error {
	if _, err := a.db.Exec(
		`UPDATE outbox SET status = ? WHERE status = ? AND kind IN (?,?,?,?,?)`,
		OpPending, OpExecuting, KindMove, KindFlag, KindDestroy, KindContactPut, KindContactDelete); err != nil {
		return err
	}
	_, err := a.db.Exec(
		`UPDATE outbox
         SET status = ?,
             error  = '{"kind":"crashed-mid-execute","message":"drainer restart"}'
         WHERE status = ?`,
		OpConflict, OpExecuting)
	return err
}

func (a *Account) drainLoop(ctx context.Context, cfg drainerConfig) {
	defer a.wg.Done()
	ticker := time.NewTicker(cfg.Idle)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stop:
			return
		case <-a.drainSignal:
		case <-ticker.C:
		}
		a.drainOnce(ctx, cfg)
	}
}

// drainOnce clears all eligible rows in a single sweep. It sets the
// burst counters on first pickup and resets them when the queue empties.
func (a *Account) drainOnce(ctx context.Context, cfg drainerConfig) {
	for {
		if ctx.Err() != nil {
			return
		}
		if a.drainerPaused.Load() {
			return
		}
		row, err := a.nextOutboxRow(time.Now())
		if errors.Is(err, sql.ErrNoRows) {
			a.burstTotal.Store(0)
			a.burstDone.Store(0)
			return
		}
		if err != nil {
			a.log.Error("drainer pickup error", "err", err)
			return
		}
		if a.burstTotal.Load() == 0 {
			if n := a.pendingCount(); n > 0 {
				a.burstTotal.Store(n)
				a.burstDone.Store(0)
			}
		}
		a.executeOne(ctx, row, cfg)
	}
}

// executeOne runs one op against the backend and writes its terminal
// state. CacheEvents fire only on terminal transitions, never on
// failed→retry.
func (a *Account) executeOne(ctx context.Context, row *outboxRow, cfg drainerConfig) {
	if err := a.markExecuting(row.ID); err != nil {
		a.log.Error("mark executing", "err", err)
		return
	}
	args, err := decodeArgs(row.Kind, row.ArgsJSON)
	if err != nil {
		a.logTerminal(a.finishOp(row.ID, OpConflict, encodeErr("args-decode", err), 0), row, "finishOp")
		a.publish(row, OpConflict, err)
		return
	}
	dispatchErr := a.dispatch(args, row)
	switch {
	case dispatchErr == nil:
		a.logTerminal(a.finalizeSuccess(ctx, row, args), row, "finalizeSuccess")
		a.publish(row, OpDone, nil)
	case errors.Is(dispatchErr, contacts.ErrAuth):
		a.logTerminal(a.finishOp(row.ID, OpConflict, encodeErr("auth-failure", dispatchErr), 0), row, "finishOp")
		a.publish(row, OpConflict, dispatchErr)
	case errors.Is(dispatchErr, contacts.ErrPreconditionFailed):
		a.logTerminal(a.finishOp(row.ID, OpConflict, encodeErr("precondition-failed", dispatchErr), 0), row, "finishOp")
		a.publish(row, OpConflict, dispatchErr)
	case errors.Is(dispatchErr, contacts.ErrNotFound):
		a.logTerminal(a.finalizeSuccess(ctx, row, args), row, "finalizeSuccess")
		a.publish(row, OpDone, nil)
	case errors.Is(dispatchErr, mail.ErrAuth):
		a.logTerminal(a.finishOp(row.ID, OpConflict, encodeErr("auth-failure", dispatchErr), 0), row, "finishOp")
		a.publish(row, OpConflict, dispatchErr)
	case errors.Is(dispatchErr, mail.ErrNotFound):
		a.logTerminal(a.finalizeSuccess(ctx, row, args), row, "finalizeSuccess")
		a.publish(row, OpDone, nil)
	case errors.Is(dispatchErr, ErrDraftSuperseded):
		a.logTerminal(a.finalizeSuccess(ctx, row, args), row, "finalizeSuccess")
		a.publishNote(row, OpDone, "draft superseded by another client")
	default:
		// mail.ErrConnection lands here. Backends drop the dead client
		// and redial on the next attempt.
		if cfg.MaxAttempts > 0 && row.Attempts+1 >= cfg.MaxAttempts {
			a.logTerminal(a.finishOp(row.ID, OpConflict, encodeErr("max-attempts-exceeded", dispatchErr), 0), row, "finishOp")
			a.publish(row, OpConflict, dispatchErr)
			return
		}
		nextAt := time.Now().Add(expBackoff(row.Attempts+1, cfg.BackoffMin, cfg.BackoffMax)).UnixNano()
		a.logTerminal(a.finishOp(row.ID, OpFailed, encodeErr("transient", dispatchErr), nextAt), row, "finishOp")
	}
}

// logTerminal records a terminal-state write failure. A non-nil err
// here leaves the outbox row stuck in OpExecuting, blocking sibling
// pickup. The error path is rare but corrosive when it happens.
func (a *Account) logTerminal(err error, row *outboxRow, op string) {
	if err == nil {
		return
	}
	a.log.Error("drainer "+op, "op", row.ID, "kind", row.Kind, "err", err)
}

// finalizeSuccess writes the post-success cache mutation: junction
// row swap for Move, message-row delete for Destroy, flag sync for
// Flag.
func (a *Account) finalizeSuccess(ctx context.Context, row *outboxRow, args OpArgs) error {
	if !row.MessageID.Valid {
		return a.finishOpDoneAndDeleteDraft(ctx, row.ID, row.DraftID)
	}
	return a.tx(ctx, func(tx *sql.Tx) error {
		switch v := args.(type) {
		case MoveArgs:
			if _, err := tx.Exec(`DELETE FROM message_mailboxes WHERE message = ? AND folder = ?`,
				row.MessageID.Int64, row.FolderID); err != nil {
				return err
			}
			var destID int64
			if err := tx.QueryRow(`SELECT id FROM folders WHERE name = ?`, v.Dest).Scan(&destID); err == nil {
				if _, err := tx.Exec(`INSERT OR IGNORE INTO message_mailboxes (message, folder) VALUES (?, ?)`,
					row.MessageID.Int64, destID); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(`UPDATE messages SET ui_hide = 0 WHERE id = ?`, row.MessageID.Int64); err != nil {
				return err
			}
		case DestroyArgs:
			if _, err := tx.Exec(`DELETE FROM messages WHERE id = ?`, row.MessageID.Int64); err != nil {
				return err
			}
		case FlagArgs:
			if _, err := tx.Exec(`UPDATE messages SET flags = ui_flags WHERE id = ?`, row.MessageID.Int64); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`UPDATE outbox SET status = ?, error = '' WHERE id = ?`, OpDone, row.ID)
		return err
	})
}

func (a *Account) dispatch(args OpArgs, row *outboxRow) error {
	uids := []mail.UID{}
	if row.ProtocolID.Valid && row.ProtocolID.String != "" {
		uids = append(uids, mail.UID(row.ProtocolID.String))
	}
	switch v := args.(type) {
	case MoveArgs:
		return a.Backend.Move(uids, v.Dest)
	case FlagArgs:
		return a.Backend.Flag(uids, v.Flag, v.Set)
	case DestroyArgs:
		return a.Backend.Destroy(uids)
	case SendArgs:
		return a.Backend.Send(v.Envelope, row.Payload)
	case AppendArgs:
		return a.Backend.Append(row.FolderName, row.Payload, v.Flag)
	case PushDraftArgs:
		newUID, err := a.Backend.PushDraft(row.FolderName, row.Payload, v.PrevServerUID)
		if err != nil {
			return err
		}
		return a.MarkDraftPushed(context.Background(), v.DraftID, newUID, row.FolderName)
	case ContactPutArgs:
		// ContactsWriter is nil when the account has no CardDAV config.
		if a.ContactsWriter == nil {
			return errors.New("contacts writer not wired")
		}
		newHref, newETag, err := a.ContactsWriter.PutAddressObject(context.Background(), v.Href, v.IfMatch, row.Payload)
		if err != nil {
			return err
		}
		_, dbErr := a.db.Exec(`UPDATE contacts SET href = ?, etag = ? WHERE href = ?`, newHref, newETag, v.Href)
		return dbErr
	case ContactDeleteArgs:
		if a.ContactsWriter == nil {
			return errors.New("contacts writer not wired")
		}
		return a.ContactsWriter.DeleteAddressObject(context.Background(), v.Href, v.IfMatch)
	}
	return fmt.Errorf("dispatch: unknown args %T", args)
}

func (a *Account) pendingCount() int32 {
	var n int32
	row := a.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE status IN (?, ?)`, OpPending, OpFailed)
	if err := row.Scan(&n); err != nil {
		a.log.Error("pendingCount scan", "err", err)
		return 0
	}
	return n
}

func (a *Account) publish(row *outboxRow, status OpStatus, err error) {
	if status == OpDone {
		a.burstDone.Add(1)
	}
	ev := CacheEvent{Account: a.name, OpID: row.ID, Kind: OpKind(row.Kind), Status: status}
	if err != nil {
		ev.Err = err.Error()
	}
	a.emit(ev)
}

// publishNote is publish + an advisory banner string.
func (a *Account) publishNote(row *outboxRow, status OpStatus, note string) {
	if status == OpDone {
		a.burstDone.Add(1)
	}
	a.emit(CacheEvent{
		Account: a.name,
		OpID:    row.ID,
		Kind:    OpKind(row.Kind),
		Status:  status,
		Note:    note,
	})
}

func (a *Account) emit(ev CacheEvent) {
	select {
	case a.events <- ev:
	default:
		a.droppedEvents.Add(1)
	}
}

func decodeArgs(kind string, payload string) (OpArgs, error) {
	switch OpKind(kind) {
	case KindMove:
		var v MoveArgs
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		return v, nil
	case KindFlag:
		var v FlagArgs
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		return v, nil
	case KindDestroy:
		return DestroyArgs{}, nil
	case KindSend:
		var v SendArgs
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		return v, nil
	case KindAppend:
		var v AppendArgs
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		return v, nil
	case KindPushDraft:
		var v PushDraftArgs
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		return v, nil
	case KindContactPut:
		var v ContactPutArgs
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		return v, nil
	case KindContactDelete:
		var v ContactDeleteArgs
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		return v, nil
	}
	return nil, fmt.Errorf("unknown op kind %q", kind)
}

func encodeErr(kind string, err error) string {
	type payload struct {
		Kind    string `json:"kind"`
		Message string `json:"message"`
	}
	b, _ := json.Marshal(payload{Kind: kind, Message: err.Error()})
	return string(b)
}
