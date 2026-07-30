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

// Config governs one Worker: the fixed push-coalescing window,
// jittered backoff bounds for reopening a push transport that stopped,
// and the bulk lane's yield window (ADR-0003 revision 2). The push
// stream's own liveness cadence is not here. The transport negotiates
// that one with the server and keeps it (backend.Push).
type Config struct {
	CoalesceWindow   time.Duration
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
// backoff range bounded by SY-2's 30s p95 recovery criterion, and a
// poll cadence generous for what is meant to be a degraded fallback,
// not the norm.
func DefaultConfig() Config {
	return Config{
		CoalesceWindow:   200 * time.Millisecond,
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

// watermark is one account, object kind, and collection's persisted
// sync position: the opaque server state token and a local revision
// counting sync cycles applied (ADR-0005's two-watermark shape).
type watermark struct {
	ServerStateToken string
	LocalRev         int64
}

// mailCollection is the collection loadWatermark and saveWatermark
// use for mail, which ADR-0005 does not segment by collection: one
// sync_state row per account and object kind, under this fixed
// sentinel rather than a real collection id. Calendar and contacts
// (pass 5) pass their calendar or address book id instead.
const mailCollection = ""

// loadWatermark returns accountID's persisted watermark for kind and
// collection, or the zero watermark (an empty token, asking Changes
// for a full initial sync) if none is recorded yet.
func loadWatermark(ctx context.Context, w *store.Writer, accountID int64, kind backend.ObjectKind, collection string) (watermark, error) {
	var wm watermark
	err := w.Apply(ctx, func(tx *sql.Tx) error {
		var token sql.NullString
		err := tx.QueryRow(
			`SELECT server_state_token, local_rev FROM sync_state WHERE account_id = ? AND object_kind = ? AND collection_id = ?`,
			accountID, kindName(kind), collection,
		).Scan(&token, &wm.LocalRev)
		// An absent row is an account's first sync. Reporting it out of
		// here would roll the transaction back and wrap it as a store
		// failure, logged at error level with nothing behind it for the
		// user to see (ADR-0013's one line per outcome).
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		wm.ServerStateToken = token.String
		return err
	})
	if err != nil {
		return watermark{}, err
	}
	return wm, nil
}

// saveWatermark writes accountID's watermark for kind and collection
// inside tx, replacing whatever sync_state row it already holds.
func saveWatermark(tx *sql.Tx, accountID int64, kind backend.ObjectKind, collection string, wm watermark) error {
	_, err := tx.Exec(
		`INSERT INTO sync_state (account_id, object_kind, collection_id, server_state_token, local_rev) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(account_id, object_kind, collection_id) DO UPDATE SET server_state_token = excluded.server_state_token, local_rev = excluded.local_rev`,
		accountID, kindName(kind), collection, wm.ServerStateToken, wm.LocalRev,
	)
	return err
}

// SyncKind runs one changes-since cycle for kind: paging Changes
// until HasMore is false, applying each page through the writer's
// bulk lane, and falling back to a full resync when the backend
// reports a state reset (an expired or unrecognized token).
func (w *Worker) SyncKind(ctx context.Context, kind backend.ObjectKind) error {
	wm, err := loadWatermark(ctx, w.writer, w.accountID, kind, mailCollection)
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
			return saveWatermark(tx, w.accountID, kind, mailCollection, next)
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
