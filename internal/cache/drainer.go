// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/glw907/poplar/internal/backoff"
	"github.com/glw907/poplar/internal/mail"
)

// drainerConfig governs backoff and retry caps. Defaults match
// spec §G; the constructor exposes a hook so tests can compress
// timings.
type drainerConfig struct {
	BackoffMin  time.Duration
	BackoffMax  time.Duration
	MaxAttempts int
	Idle        time.Duration // poll cadence when no signal arrives
}

func defaultDrainerConfig() drainerConfig {
	return drainerConfig{
		BackoffMin:  1 * time.Second,
		BackoffMax:  60 * time.Second,
		MaxAttempts: 10,
		// drainSignal handles immediate wake on every QueueOp; the
		// idle ticker exists only so failed-row backoff windows are
		// re-checked without an external signal. 5s is a comfortable
		// floor: well under typical backoff windows, well above what
		// would burn CPU.
		Idle: 5 * time.Second,
	}
}

// StartDrainer launches the per-account outbox drainer. The
// goroutine exits when Close is called. Crash-recovered
// `executing` rows are reset (idempotent kinds → pending; send
// → conflict with crashed-mid-execute) before the loop starts.
func (a *Account) StartDrainer(ctx context.Context) error {
	if err := a.recoverExecuting(); err != nil {
		return fmt.Errorf("recover executing: %w", err)
	}
	cfg := defaultDrainerConfig()
	a.wg.Add(1)
	go a.drainLoop(ctx, cfg)
	return nil
}

// recoverExecuting handles the crash-recovery contract from spec
// §D.2. Idempotent kinds reset to pending; non-idempotent (send,
// append) become conflict with crashed-mid-execute.
func (a *Account) recoverExecuting() error {
	if _, err := a.db.Exec(
		`UPDATE outbox SET status = ? WHERE status = ? AND kind IN (?,?,?)`,
		OpPending, OpExecuting, KindMove, KindFlag, KindDestroy); err != nil {
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

// drainLoop is the single-account drainer goroutine. It blocks on
// drainSignal or Idle ticks, picks one eligible row at a time, and
// routes through executeOp.
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

// drainOnce attempts to clear all eligible rows in a single sweep.
// It runs until nextOutboxRow returns sql.ErrNoRows.
func (a *Account) drainOnce(ctx context.Context, cfg drainerConfig) {
	for {
		if ctx.Err() != nil {
			return
		}
		row, err := a.nextOutboxRow(time.Now())
		if errors.Is(err, sql.ErrNoRows) {
			return
		}
		if err != nil {
			fmt.Fprintln(stderrLog(), "cache: drainer pickup error:", err)
			return
		}
		a.executeOne(ctx, row, cfg)
	}
}

// executeOne runs one op against the backend and writes its
// terminal state. It emits a CacheEvent on every terminal
// transition (done, conflict, failed→retry not included).
func (a *Account) executeOne(ctx context.Context, row *outboxRow, cfg drainerConfig) {
	if err := a.markExecuting(row.ID); err != nil {
		fmt.Fprintln(stderrLog(), "cache: mark executing:", err)
		return
	}
	args, err := decodeArgs(row.Kind, row.ArgsJSON)
	if err != nil {
		_ = a.finishOp(row.ID, OpConflict, encodeErr("args-decode", err), 0)
		a.publish(row, OpConflict, err)
		return
	}
	if a.Backend == nil {
		_ = a.finishOp(row.ID, OpFailed,
			encodeErr("no-backend", errors.New("backend not wired")),
			time.Now().Add(cfg.BackoffMax).UnixNano())
		return
	}
	uids := []mail.UID{}
	if row.ProtocolID.Valid && row.ProtocolID.String != "" {
		uids = append(uids, mail.UID(row.ProtocolID.String))
	}
	dispatchErr := a.dispatch(args, uids)
	switch {
	case dispatchErr == nil:
		_ = a.finalizeSuccess(ctx, row, args)
		a.publish(row, OpDone, nil)
	case errors.Is(dispatchErr, mail.ErrAuth):
		_ = a.finishOp(row.ID, OpConflict, encodeErr("auth-failure", dispatchErr), 0)
		a.publish(row, OpConflict, dispatchErr)
	case errors.Is(dispatchErr, mail.ErrNotFound):
		// Idempotent success per spec §D.4 — message already gone.
		_ = a.finalizeSuccess(ctx, row, args)
		a.publish(row, OpDone, nil)
	default:
		if cfg.MaxAttempts > 0 && row.Attempts+1 >= cfg.MaxAttempts {
			_ = a.finishOp(row.ID, OpConflict, encodeErr("max-attempts-exceeded", dispatchErr), 0)
			a.publish(row, OpConflict, dispatchErr)
			return
		}
		nextAt := time.Now().Add(backoffFor(cfg)(row.Attempts + 1)).UnixNano()
		_ = a.finishOp(row.ID, OpFailed, encodeErr("transient", dispatchErr), nextAt)
	}
}

// finalizeSuccess writes the post-success cache mutation: drop
// junction row for move sources, delete the message row for
// destroy, sync flags for flag.
func (a *Account) finalizeSuccess(ctx context.Context, row *outboxRow, args OpArgs) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		if !row.MessageID.Valid {
			return mark(tx, row, OpDone)
		}
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
		return mark(tx, row, OpDone)
	})
}

func mark(tx *sql.Tx, row *outboxRow, status OpStatus) error {
	_, err := tx.Exec(`UPDATE outbox SET status = ?, error = '' WHERE id = ?`, status, row.ID)
	return err
}

// dispatch routes one decoded op to the backend.
func (a *Account) dispatch(args OpArgs, uids []mail.UID) error {
	switch v := args.(type) {
	case MoveArgs:
		return a.Backend.Move(uids, v.Dest)
	case FlagArgs:
		return a.Backend.Flag(uids, v.Flag, v.Set)
	case DestroyArgs:
		return a.Backend.Destroy(uids)
	}
	return fmt.Errorf("dispatch: unknown args %T", args)
}

func (a *Account) publish(row *outboxRow, status OpStatus, err error) {
	ev := CacheEvent{Account: a.name, OpID: row.ID, Kind: OpKind(row.Kind), Status: status}
	if err != nil {
		ev.Err = err.Error()
	}
	select {
	case a.events <- ev:
	default:
		// Buffer full — record drop so the UI can detect staleness
		// and reconcile via a full cache re-read (DroppedEvents).
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

// backoffFor returns a function that maps attempt count → wait
// duration via the shared internal/backoff helper.
func backoffFor(cfg drainerConfig) func(int) time.Duration {
	return func(attempts int) time.Duration {
		return backoff.Exponential(attempts, cfg.BackoffMin, cfg.BackoffMax)
	}
}

// stderrLog returns the writer for diagnostic logs. Tests reassign
// it to capture output. Default writes to os.Stderr.
var stderrLog = func() interface{ Write(p []byte) (int, error) } { return os.Stderr }
