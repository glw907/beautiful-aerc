// Package sync is poplar's delta orchestration engine (ADR-0005): per
// account and object kind it keeps two persisted watermarks, the
// opaque server state token and a local revision, applies
// changes-since batches through the store writer's bulk lane, and
// treats a server state reset as a normal full-resync path rather
// than an error. Worker also consumes a backend's push transport,
// coalescing bursts into a fixed window and reconnecting through
// jittered backoff on drop.
//
// Sync never touches the terminal and never writes the store
// directly: every mutation runs through store.Writer.Apply, the bulk
// lane's entry point from outside internal/store.
package sync

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
)

// Config governs one Worker: the fixed push-coalescing window, the
// server's requested EventSource ping cadence (a stall is silence
// past twice this), jittered backoff bounds on stream loss, and the
// bulk lane's yield window (ADR-0003 revision 2).
type Config struct {
	CoalesceWindow   time.Duration
	PingInterval     time.Duration
	BackoffMin       time.Duration
	BackoffMax       time.Duration
	InteractiveQuiet time.Duration
	// PollInterval is the fixed cadence RunPush falls back to for a
	// backend whose Push() is nil (backend.PushTransportNone): with no
	// event stream to coalesce, a plain ticker takes push's place.
	PollInterval time.Duration
}

// DefaultConfig returns poplar's production sync timing. CoalesceWindow
// is ADR-0005 revision 2's fixed value; the others have no document
// specifying them, so Worker picks values proportionate to it: a
// ping cadence generous enough that a live server's own keepalive
// never trips the stall detector, a backoff range bounded by SY-2's
// 30s p95 recovery criterion, and a poll cadence (twice the ping
// interval) generous for what is meant to be a degraded fallback, not
// the norm.
func DefaultConfig() Config {
	return Config{
		CoalesceWindow:   200 * time.Millisecond,
		PingInterval:     30 * time.Second,
		BackoffMin:       500 * time.Millisecond,
		BackoffMax:       30 * time.Second,
		InteractiveQuiet: time.Second,
		PollInterval:     60 * time.Second,
	}
}

// Worker syncs one account's mail against a backend: changes-since
// batches through writer's bulk lane, and (via RunPush) the backend's
// push transport.
type Worker struct {
	accountID int64
	backend   backend.Backend
	writer    *store.Writer
	cfg       Config
	echo      *echoTracker
}

// NewWorker returns a Worker for accountID, syncing be against writer
// under cfg.
func NewWorker(accountID int64, be backend.Backend, writer *store.Writer, cfg Config) *Worker {
	return &Worker{accountID: accountID, backend: be, writer: writer, cfg: cfg, echo: newEchoTracker()}
}

// NoteDispatchedState records that a dispatch already produced token
// as kind's new server state, for the exact records named by ids
// (ADR-0005 revision 2's self-echo suppression): the next
// push-triggered sync cycle that resolves to this same token skips
// re-applying only those records, so an outbox's own optimistic write
// never round-trips into a redundant re-apply while a third-party
// change batched into the same page still lands. The outbox
// dispatcher calls this after a successful ApplyBatch; ApplyBatch's
// BatchResult carries no post-dispatch state token yet, so wiring the
// real production caller is task 10's, once BatchResult (or its
// caller) can supply one.
func (w *Worker) NoteDispatchedState(kind backend.ObjectKind, token string, ids []string) {
	w.echo.note(kind, token, ids)
}

// kindName is sync_state.object_kind's value for kind.
func kindName(kind backend.ObjectKind) string {
	switch kind {
	case backend.ObjectKindMessage:
		return "message"
	case backend.ObjectKindMailbox:
		return "mailbox"
	case backend.ObjectKindEvent:
		return "event"
	case backend.ObjectKindContact:
		return "contact"
	default:
		return "unknown"
	}
}

// watermark is one account and object kind's persisted sync
// position: the opaque server state token and a local revision
// counting sync cycles applied (ADR-0005's two-watermark shape).
type watermark struct {
	ServerStateToken string
	LocalRev         int64
}

// loadWatermark returns accountID's persisted watermark for kind, or
// the zero watermark (an empty token, asking Changes for a full
// initial sync) if none is recorded yet.
func loadWatermark(ctx context.Context, w *store.Writer, accountID int64, kind backend.ObjectKind) (watermark, error) {
	var wm watermark
	var token sql.NullString
	err := w.Apply(ctx, func(tx *sql.Tx) error {
		return tx.QueryRow(
			`SELECT server_state_token, local_rev FROM sync_state WHERE account_id = ? AND object_kind = ?`,
			accountID, kindName(kind),
		).Scan(&token, &wm.LocalRev)
	})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return watermark{}, nil
	case err != nil:
		return watermark{}, err
	}
	wm.ServerStateToken = token.String
	return wm, nil
}

// saveWatermark writes accountID's watermark for kind inside tx,
// replacing whatever sync_state row it already holds.
func saveWatermark(tx *sql.Tx, accountID int64, kind backend.ObjectKind, wm watermark) error {
	_, err := tx.Exec(
		`INSERT INTO sync_state (account_id, object_kind, server_state_token, local_rev) VALUES (?, ?, ?, ?)
		 ON CONFLICT(account_id, object_kind) DO UPDATE SET server_state_token = excluded.server_state_token, local_rev = excluded.local_rev`,
		accountID, kindName(kind), wm.ServerStateToken, wm.LocalRev,
	)
	return err
}

// SyncKind runs one changes-since cycle for kind: paging Changes
// until HasMore is false, applying each page through the writer's
// bulk lane, and falling back to a full resync when the backend
// reports a state reset (an expired or unrecognized token).
func (w *Worker) SyncKind(ctx context.Context, kind backend.ObjectKind) error {
	wm, err := loadWatermark(ctx, w.writer, w.accountID, kind)
	if err != nil {
		return err
	}

	for {
		cs, err := w.backend.Mail().Changes(ctx, kind, wm.ServerStateToken, 0)
		if err != nil {
			if errors.Is(err, backend.ErrStateReset) {
				return w.fullResync(ctx, kind)
			}
			return err
		}

		skip := w.echo.consume(kind, cs.NewToken)
		next := watermark{ServerStateToken: cs.NewToken, LocalRev: wm.LocalRev + 1}
		err = runBulkChunks(ctx, w.writer, w.cfg.InteractiveQuiet, func(tx *sql.Tx) error {
			if err := applyChangeSet(tx, w.accountID, kind, cs, skip); err != nil {
				return err
			}
			return saveWatermark(tx, w.accountID, kind, next)
		})
		if err != nil {
			return err
		}
		wm = next
		if !cs.HasMore {
			return nil
		}
	}
}
