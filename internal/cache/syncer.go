package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// SyncFolder runs one ChangeTracker.Changes pass for folder. On
// mail.ErrCannotCalculateChanges the folder is re-anchored, and the
// next call pulls a fresh baseline.
func (a *Account) SyncFolder(ctx context.Context, folder string) error {
	if !a.Connected() {
		return ErrNotConnected
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

// readSyncToken returns the stored cursor for folder. A missing row
// returns a nil token, which the syncer treats as a baseline call.
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

func (a *Account) writeSyncToken(folder string, token mail.SyncToken) error {
	_, err := a.db.Exec(`UPDATE folders SET sync_token = ?, last_synced = ? WHERE name = ?`,
		[]byte(token), time.Now().UnixNano(), folder)
	return err
}

// applyDelta materializes the ChangeSet against the cache. Removed
// UIDs delete via CASCADE. Added and Modified upsert from a header fetch.
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
	infos, err := a.Backend.FetchHeaders(fetch)
	if err != nil {
		return fmt.Errorf("backend fetch headers: %w", err)
	}
	return a.upsertMessages(ctx, folder, infos)
}

// reAnchor promotes any pending/executing outbox row whose backing
// message is about to be wiped to conflict-anchor-lost, then clears
// the folder so the next SyncFolder rebuilds from scratch.
func (a *Account) reAnchor(ctx context.Context, folder string) error {
	folderID, err := a.folderID(folder)
	if err != nil {
		return err
	}
	return a.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
            UPDATE outbox
            SET status = 'conflict',
                error  = '{"kind":"anchor-lost","message":"folder re-anchored"}'
            WHERE status IN ('pending', 'executing')
              AND message IN (
                SELECT mm.message FROM message_mailboxes mm WHERE mm.folder = ?)`, folderID); err != nil {
			return fmt.Errorf("promote outbox: %w", err)
		}
		// Unlink only this folder. A JMAP message in many mailboxes still lives elsewhere.
		if _, err := tx.Exec(`DELETE FROM message_mailboxes WHERE folder = ?`, folderID); err != nil {
			return fmt.Errorf("clear membership: %w", err)
		}
		if _, err := tx.Exec(`UPDATE folders SET sync_token = NULL, last_synced = NULL WHERE id = ?`, folderID); err != nil {
			return fmt.Errorf("clear sync token: %w", err)
		}
		return nil
	})
}

// SyncFolders refreshes the folders table from the backend listing.
// Rows that no longer appear server-side are deleted, and CASCADE
// reaps their messages and outbox entries.
func (a *Account) SyncFolders(ctx context.Context) error {
	if !a.Connected() {
		return ErrNotConnected
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
