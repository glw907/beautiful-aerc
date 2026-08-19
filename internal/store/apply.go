package store

import (
	"context"
	"database/sql"
)

// Apply runs fn as one transaction on the writer's bulk lane: the
// entry point ADR-0003 reserves for a background engine outside
// internal/store (internal/sync's changes-since batches and
// resyncs) to reach the writer, alongside submitBulk's in-package
// callers. fn shares submitBulk's all-or-nothing commit.
func (w *Writer) Apply(ctx context.Context, fn func(*sql.Tx) error) error {
	return w.submitBulk(ctx, fn)
}

// ApplyInteractive runs fn as one transaction on the writer's
// interactive lane: the entry point a future outbox dispatcher's
// claim-then-dispatch transaction reaches the store through, and the
// one internal/sync's tests use to simulate recent interactive
// activity when exercising the bulk lane's subordination policy.
func (w *Writer) ApplyInteractive(ctx context.Context, fn func(*sql.Tx) error) error {
	return w.submit(ctx, fn)
}

// applyTagged is Apply with a caller-chosen op in place of the
// generic store.write. It stays unexported: RebuildIndex is package
// store's only caller, tagging its own failure so a log line can
// tell it apart from an ordinary write failure without a second
// uerr.New call over the same outcome (ADR-0013 revision 2).
func (w *Writer) applyTagged(ctx context.Context, op string, fn func(*sql.Tx) error) error {
	return w.submitBulkTagged(ctx, op, fn)
}
