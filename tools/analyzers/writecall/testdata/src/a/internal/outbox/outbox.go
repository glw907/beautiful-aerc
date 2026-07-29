// Package outbox stands in for poplar's internal/outbox, the
// durable intent queue: the sanctioned way a mutation reaches the
// writer, so its transaction work draws no diagnostic.
package outbox

import (
	"context"
	"database/sql"

	"a/internal/store"
)

func EnqueueCreateMailbox(ctx context.Context, w *store.Writer, accountID int64, m store.MailboxUpsert) error {
	return w.ApplyInteractive(ctx, func(tx *sql.Tx) error {
		return store.UpsertMailbox(tx, accountID, m)
	})
}
