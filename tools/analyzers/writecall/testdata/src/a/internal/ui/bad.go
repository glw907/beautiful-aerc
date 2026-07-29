package ui

import (
	"context"
	"database/sql"

	"a/internal/store"
)

var held *store.Writer // want `internal/ui reaches store\.Writer, part of internal/store's write surface; mutate through an intent \(ADR-0003\)`

func construct(ctx context.Context, db *sql.DB, path string) {
	_, _ = store.NewWriter(db, store.DefaultWriterConfig()) // want `reaches store\.NewWriter, part of internal/store's write surface`
	_, _ = store.Open(path, store.DefaultWriterConfig())    // want `reaches store\.Open, part of internal/store's write surface`
	_, _ = store.OpenWriteConn(path)                        // want `reaches store\.OpenWriteConn, part of internal/store's write surface`
	_ = store.Migrate(db)                                   // want `reaches store\.Migrate, part of internal/store's write surface`
	_ = store.CheckIntegrity(ctx, db)                       // want `reaches store\.CheckIntegrity, part of internal/store's write surface`
	_ = store.Recover(ctx, path)                            // want `reaches store\.Recover, part of internal/store's write surface`
}

func lanes(ctx context.Context, w *store.Writer) { // want `reaches store\.Writer, part of internal/store's write surface`
	_ = store.RebuildIndex(ctx, w)      // want `reaches store\.RebuildIndex, part of internal/store's write surface`
	_ = w.Apply(ctx, upsert)            // want `reaches store\.Apply, part of internal/store's write surface`
	_ = w.ApplyInteractive(ctx, remove) // want `reaches store\.ApplyInteractive, part of internal/store's write surface`
	_ = w.Close()                       // want `reaches store\.Close, part of internal/store's write surface`
}

func upsert(tx *sql.Tx) error {
	if err := store.UpsertMessage(tx, 1, store.MessageUpsert{ServerID: "m1"}); err != nil { // want `reaches store\.UpsertMessage, part of internal/store's write surface`
		return err
	}
	if err := store.UpsertMailbox(tx, 1, store.MailboxUpsert{ServerID: "b1"}); err != nil { // want `reaches store\.UpsertMailbox, part of internal/store's write surface`
		return err
	}
	if err := store.SyncMessageMailboxes(tx, 1, 2, []string{"b1"}); err != nil { // want `reaches store\.SyncMessageMailboxes, part of internal/store's write surface`
		return err
	}
	return store.RepairMailboxAssociations(tx, 1, 2, "b1") // want `reaches store\.RepairMailboxAssociations, part of internal/store's write surface`
}

func remove(tx *sql.Tx) error {
	messages, err := store.StaleMessageIDs(tx, 1, map[string]bool{"m1": true}) // want `reaches store\.StaleMessageIDs, part of internal/store's write surface`
	if err != nil {
		return err
	}
	for _, id := range messages {
		if err := store.DeleteMessageByID(tx, id); err != nil { // want `reaches store\.DeleteMessageByID, part of internal/store's write surface`
			return err
		}
	}
	mailboxes, err := store.StaleMailboxIDs(tx, 1, map[string]bool{"b1": true}) // want `reaches store\.StaleMailboxIDs, part of internal/store's write surface`
	if err != nil {
		return err
	}
	for _, id := range mailboxes {
		if err := store.DeleteMailboxByID(tx, id); err != nil { // want `reaches store\.DeleteMailboxByID, part of internal/store's write surface`
			return err
		}
	}
	if err := store.DeleteMessage(tx, 1, "m1"); err != nil { // want `reaches store\.DeleteMessage, part of internal/store's write surface`
		return err
	}
	return store.DeleteMailbox(tx, 1, "b1") // want `reaches store\.DeleteMailbox, part of internal/store's write surface`
}
