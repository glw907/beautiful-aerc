package sync

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/glw907/poplar/internal/backend"
)

// fullResync rebuilds accountID's server-derived state for kind from
// a fresh baseline pull (an empty-token Changes call, paged to
// completion): the normal path ADR-0005 revision 2 treats a state
// reset or token expiry as, not an error. Each page upserts through
// its own bulk-lane transaction as it arrives, rather than buffering
// every hydrated Record from every page before writing anything: a
// 100k-message baseline stays within QA-5's memory bound this way,
// and no single transaction holds the bulk lane for the whole
// resync. Only the ids kept for the final stale-delete pass
// accumulate in memory.
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
		if err := runBulkChunks(ctx, w.writer, w.cfg.InteractiveQuiet, func(tx *sql.Tx) error {
			return upsertPage(tx, w.accountID, kind, cs.Created)
		}); err != nil {
			return err
		}
		token = cs.NewToken
		if !cs.HasMore {
			break
		}
	}

	return runBulkChunks(ctx, w.writer, w.cfg.InteractiveQuiet, func(tx *sql.Tx) error {
		if err := deleteStale(tx, w.accountID, kind, keep); err != nil {
			return err
		}
		return saveWatermark(tx, w.accountID, kind, watermark{ServerStateToken: token, LocalRev: 1})
	})
}

// upsertPage upserts one baseline resync page's records, keyed by
// server id so a surviving row keeps its poplar-minted internal id.
func upsertPage(tx *sql.Tx, accountID int64, kind backend.ObjectKind, records []backend.Record) error {
	for _, rec := range records {
		if err := upsertRecord(tx, accountID, kind, rec); err != nil {
			return err
		}
	}
	return nil
}

// deleteStale removes whatever accountID's kind rows are missing from
// keep, the full baseline listing's server ids collected across every
// resync page. For kind Message only origin = 'server' rows are ever
// a deletion candidate, so an origin = 'local' draft, its body, and
// any outbox row naming it survive a resync untouched.
func deleteStale(tx *sql.Tx, accountID int64, kind backend.ObjectKind, keep map[string]bool) error {
	stale, err := staleIDs(tx, accountID, kind, keep)
	if err != nil {
		return err
	}
	for _, id := range stale {
		if err := deleteByID(tx, kind, id); err != nil {
			return err
		}
	}
	return nil
}

// staleIDs returns the internal ids of accountID's kind rows whose
// server id is absent from keep: for Message, scoped to origin =
// 'server' rows, so a local-only draft is never considered.
func staleIDs(tx *sql.Tx, accountID int64, kind backend.ObjectKind, keep map[string]bool) ([]int64, error) {
	var rows *sql.Rows
	var err error
	switch kind {
	case backend.ObjectKindMessage:
		rows, err = tx.Query(`SELECT id, server_id FROM message WHERE account_id = ? AND origin = 'server'`, accountID)
	case backend.ObjectKindMailbox:
		rows, err = tx.Query(`SELECT id, server_id FROM mailbox WHERE account_id = ?`, accountID)
	default:
		return nil, fmt.Errorf("sync: resync: unsupported kind %v", kind)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var stale []int64
	for rows.Next() {
		var id int64
		var serverID sql.NullString
		if err := rows.Scan(&id, &serverID); err != nil {
			return nil, err
		}
		if !serverID.Valid || !keep[serverID.String] {
			stale = append(stale, id)
		}
	}
	return stale, rows.Err()
}

func deleteByID(tx *sql.Tx, kind backend.ObjectKind, id int64) error {
	switch kind {
	case backend.ObjectKindMessage:
		_, err := tx.Exec(`DELETE FROM message WHERE id = ?`, id)
		return err
	case backend.ObjectKindMailbox:
		_, err := tx.Exec(`DELETE FROM mailbox WHERE id = ?`, id)
		return err
	default:
		return fmt.Errorf("sync: resync: unsupported kind %v", kind)
	}
}
