// Package sync stands in for poplar's internal/sync, one of the
// engines in ADR-0003's writer cast: its transaction work draws no
// diagnostic.
package sync

import (
	"context"
	"database/sql"

	"a/internal/store"
)

func ApplyPage(ctx context.Context, w *store.Writer, accountID int64, messages []store.MessageUpsert) error {
	return w.Apply(ctx, func(tx *sql.Tx) error {
		for _, m := range messages {
			if err := store.UpsertMessage(tx, accountID, m); err != nil {
				return err
			}
		}
		return nil
	})
}
