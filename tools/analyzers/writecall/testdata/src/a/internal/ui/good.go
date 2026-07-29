package ui

import (
	"context"

	"a/internal/store"
)

func read(ctx context.Context, path string, mailboxID int64) ([]store.MessageSummary, error) {
	pool, err := store.NewReadPool(path, 4)
	if err != nil {
		return nil, err
	}
	return pool.ListMailboxForward(ctx, mailboxID)
}

func keywords(bits store.Flags, overflow []string) []string {
	return store.DecodeFlags(bits, overflow)
}
