// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// SyncFolder runs one ChangeTracker.Changes pass for folder and
// applies the delta against the cache. On
// mail.ErrCannotCalculateChanges the folder is re-anchored: the
// stored sync_token is cleared, any pending/executing outbox row
// for a soon-to-be-deleted message is promoted to conflict with
// error.kind = "anchor-lost", and a fresh baseline pull happens
// on the next call.
func (a *Account) SyncFolder(ctx context.Context, folder string) error {
	if a.ChangeTracker == nil {
		return fmt.Errorf("cache: no change tracker")
	}
	since, err := a.readSyncToken(folder)
	if err != nil {
		return err
	}
	delta, next, err := a.ChangeTracker.Changes(ctx, folder, since)
	if errors.Is(err, mail.ErrCannotCalculateChanges) {
		return a.reAnchor(ctx, folder)
	}
	if err != nil {
		return fmt.Errorf("changes: %w", err)
	}
	if err := a.applyDelta(ctx, folder, delta); err != nil {
		return err
	}
	return a.writeSyncToken(folder, next)
}

// readSyncToken returns the stored opaque sync cursor for folder.
// Missing folder rows return a nil token and no error — the syncer
// then treats this as an initial-baseline call.
func (a *Account) readSyncToken(folder string) (mail.SyncToken, error) {
	var token []byte
	err := a.db.QueryRow(`SELECT sync_token FROM folders WHERE name = ?`, folder).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read sync token: %w", err)
	}
	return mail.SyncToken(token), nil
}

// writeSyncToken updates folders.sync_token + last_synced.
func (a *Account) writeSyncToken(folder string, token mail.SyncToken) error {
	_, err := a.db.Exec(`UPDATE folders SET sync_token = ?, last_synced = ? WHERE name = ?`,
		[]byte(token), time.Now().UnixNano(), folder)
	return err
}

// applyDelta materializes the ChangeSet against the cache. Added
// and Modified UIDs trigger a header fetch (cache misses) and
// upsert; Removed UIDs are deleted with CASCADE handling outbox.
func (a *Account) applyDelta(ctx context.Context, folder string, d mail.ChangeSet) error {
	if len(d.Removed) > 0 {
		placeholders, args := uidsPlaceholders(d.Removed)
		if _, err := a.db.ExecContext(ctx,
			`DELETE FROM messages WHERE protocol_id IN (`+placeholders+`)`, args...); err != nil {
			return fmt.Errorf("delete removed: %w", err)
		}
	}
	fetch := append([]mail.UID{}, d.Added...)
	fetch = append(fetch, d.Modified...)
	if len(fetch) == 0 {
		return nil
	}
	if a.Backend == nil {
		return fmt.Errorf("cache: no backend")
	}
	infos, err := a.Backend.FetchHeaders(fetch)
	if err != nil {
		return fmt.Errorf("backend fetch headers: %w", err)
	}
	return a.upsertMessages(ctx, folder, infos)
}

// reAnchor implements the spec §D.4 contract: promote any
// pending/executing outbox row whose backing message would be wiped
// to conflict with anchor-lost, then clear the folder so the next
// SyncFolder call rebuilds from scratch.
func (a *Account) reAnchor(ctx context.Context, folder string) error {
	folderID, err := a.folderID(folder)
	if err != nil {
		return err
	}
	return a.tx(ctx, func(tx *sql.Tx) error {
		// Promote outbox rows whose messages are about to be wiped.
		if _, err := tx.Exec(`
            UPDATE outbox
            SET status = 'conflict',
                error  = '{"kind":"anchor-lost","message":"folder re-anchored"}'
            WHERE status IN ('pending', 'executing')
              AND message IN (
                SELECT mm.message FROM message_mailboxes mm WHERE mm.folder = ?)`, folderID); err != nil {
			return fmt.Errorf("promote outbox: %w", err)
		}
		// Drop folder membership; CASCADE on the messages row would
		// be too aggressive (a message in many JMAP mailboxes still
		// lives elsewhere). We unlink only this folder.
		if _, err := tx.Exec(`DELETE FROM message_mailboxes WHERE folder = ?`, folderID); err != nil {
			return fmt.Errorf("clear membership: %w", err)
		}
		if _, err := tx.Exec(`UPDATE folders SET sync_token = NULL, last_synced = NULL WHERE id = ?`, folderID); err != nil {
			return fmt.Errorf("clear sync token: %w", err)
		}
		return nil
	})
}

// ListFolders. Existing rows are updated; new rows are inserted;
// rows that no longer appear in the backend are removed (CASCADE
// reaps their messages and outbox entries).
func (a *Account) SyncFolders(ctx context.Context) error {
	if a.Backend == nil {
		return fmt.Errorf("cache: no backend")
	}
	folders, err := a.Backend.ListFolders()
	if err != nil {
		return fmt.Errorf("backend list folders: %w", err)
	}
	classified := mail.Classify(folders)
	return a.tx(ctx, func(tx *sql.Tx) error {
		live := make(map[string]struct{}, len(classified))
		for _, cf := range classified {
			canonical := cf.DisplayName
			live[canonical] = struct{}{}
			if _, err := tx.Exec(`
                INSERT INTO folders (name, protocol_name, role, exists_total, unseen_total)
                VALUES (?, ?, ?, ?, ?)
                ON CONFLICT(name) DO UPDATE SET
                  protocol_name = excluded.protocol_name,
                  role          = excluded.role,
                  exists_total  = excluded.exists_total,
                  unseen_total  = excluded.unseen_total`,
				canonical, cf.Folder.Name, cf.Folder.Role, cf.Folder.Exists, cf.Folder.Unseen); err != nil {
				return fmt.Errorf("upsert folder %q: %w", canonical, err)
			}
		}
		rows, err := tx.Query(`SELECT name FROM folders`)
		if err != nil {
			return err
		}
		var stale []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				return err
			}
			if _, ok := live[name]; !ok {
				stale = append(stale, name)
			}
		}
		rows.Close()
		for _, name := range stale {
			if _, err := tx.Exec(`DELETE FROM folders WHERE name = ?`, name); err != nil {
				return fmt.Errorf("delete stale folder %q: %w", name, err)
			}
		}
		return nil
	})
}
