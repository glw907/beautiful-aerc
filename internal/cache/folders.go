// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/glw907/poplar/internal/mail"
)

// SyncFolders refreshes the folders table from the backend's
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
                INSERT INTO folders (name, protocol_name, role)
                VALUES (?, ?, ?)
                ON CONFLICT(name) DO UPDATE SET
                  protocol_name = excluded.protocol_name,
                  role          = excluded.role`,
				canonical, cf.Folder.Name, cf.Folder.Role); err != nil {
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
