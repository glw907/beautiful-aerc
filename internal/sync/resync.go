package sync

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/store"
)

// fullResync rebuilds accountID's server-derived state for kind from
// a fresh baseline pull (an empty-token Changes call, paged to
// completion): the normal path ADR-0005 revision 2 treats a state
// reset or token expiry as, not an error. Each page writes through
// the bulk lane as it arrives, rather than buffering every hydrated
// Record from every page before writing anything, so a 100k-message
// baseline stays within QA-5's memory bound. Only the ids kept for
// the final stale-delete pass accumulate in memory.
//
// The resync is idempotent: an upsert is keyed by server id, a delete
// names a row the new baseline no longer lists, and the watermark
// lands only after both passes have. A chunk that fails partway
// therefore leaves the store holding part of the baseline under the
// watermark it already had, and the next cycle's state reset runs the
// resync again.
func (w *Worker) fullResync(ctx context.Context, kind backend.ObjectKind) error {
	keep := make(map[string]bool)
	var token string
	for {
		cs, err := w.backend.Mail().Changes(ctx, kind, token, 0)
		if err != nil {
			return err
		}
		for _, rec := range cs.Created {
			keep[rec.ID] = true
		}
		err = applyChunked(ctx, w, cs.Created, func(tx *sql.Tx, rec backend.Record) error {
			return upsertRecord(tx, w.accountID, kind, rec)
		})
		if err != nil {
			return err
		}
		token = cs.NewToken
		if !cs.HasMore {
			break
		}
	}

	if err := w.deleteStale(ctx, kind, keep); err != nil {
		return err
	}
	return runBulkChunks(ctx, w.writer, w.cfg.InteractiveQuiet, func(tx *sql.Tx) error {
		return saveWatermark(tx, w.accountID, kind, mailCollection, watermark{ServerStateToken: token, LocalRev: 1})
	})
}

// deleteStale removes whatever accountID's kind rows are missing from
// keep, the full baseline listing's server ids collected across every
// resync page. For kind Message only origin = 'server' rows are ever
// a deletion candidate, so an origin = 'local' draft, its body, and
// any outbox row naming it survive a resync untouched.
//
// Both halves of the pass are bounded: the scan pages by internal id
// across transactions of its own, and each page's deletes follow it.
// Every delete names a row behind the cursor, so removing rows
// mid-scan cannot make a later page skip one.
func (w *Worker) deleteStale(ctx context.Context, kind backend.ObjectKind, keep map[string]bool) error {
	var after int64
	for {
		var stale []int64
		var cursor int64
		err := runBulkChunks(ctx, w.writer, w.cfg.InteractiveQuiet, func(tx *sql.Tx) error {
			var err error
			stale, cursor, err = staleIDs(tx, w.accountID, kind, keep, after)
			return err
		})
		if err != nil {
			return err
		}
		if cursor == 0 {
			return nil
		}
		after = cursor

		err = applyChunked(ctx, w, stale, func(tx *sql.Tx, id int64) error {
			return deleteByID(tx, kind, id)
		})
		if err != nil {
			return err
		}
	}
}

// staleIDs returns the internal ids of accountID's kind rows whose
// server id is absent from keep, among the one page of rows following
// internal id after, and the highest id that page examined. A zero
// cursor reports that the scan reached the end. For Message the scan
// is scoped to origin = 'server' rows, so a local-only draft is never
// considered. The query itself is
// store.StaleMessageIDs/StaleMailboxIDs's concern; this only picks
// which one kind names.
func staleIDs(tx *sql.Tx, accountID int64, kind backend.ObjectKind, keep map[string]bool, after int64) ([]int64, int64, error) {
	switch kind {
	case backend.ObjectKindMessage:
		return store.StaleMessageIDs(tx, accountID, keep, after, staleScanPage)
	case backend.ObjectKindMailbox:
		return store.StaleMailboxIDs(tx, accountID, keep, after, staleScanPage)
	default:
		return nil, 0, fmt.Errorf("sync: resync: unsupported kind %v", kind)
	}
}

func deleteByID(tx *sql.Tx, kind backend.ObjectKind, id int64) error {
	switch kind {
	case backend.ObjectKindMessage:
		return store.DeleteMessageByID(tx, id)
	case backend.ObjectKindMailbox:
		return store.DeleteMailboxByID(tx, id)
	default:
		return fmt.Errorf("sync: resync: unsupported kind %v", kind)
	}
}
