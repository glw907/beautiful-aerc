package cache

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestCancelOpsAllPending(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()

	id1 := mustInsertPendingSend(t, a, "")
	id2 := mustInsertPendingSend(t, a, "")

	if err := a.CancelOps(ctx, []int64{id1, id2}); err != nil {
		t.Fatalf("CancelOps: %v", err)
	}

	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE id IN (?, ?)`, id1, id2).Scan(&n); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if n != 0 {
		t.Errorf("rows remaining: %d, want 0", n)
	}
}

func TestCancelOpsRejectsExecuting(t *testing.T) {
	a := openTestAccount(t)

	id1 := mustInsertPendingSend(t, a, "")
	id2 := mustInsertPendingSend(t, a, "")
	mustExec(t, a.db, `UPDATE outbox SET status = ? WHERE id = ?`, OpExecuting, id2)

	err := a.CancelOps(context.Background(), []int64{id1, id2})
	if !errors.Is(err, ErrNotPending) {
		t.Fatalf("err = %v, want ErrNotPending", err)
	}

	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE id IN (?, ?)`, id1, id2).Scan(&n); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if n != 2 {
		t.Errorf("rows remaining: %d, want 2 (atomic reject)", n)
	}
}

func TestCancelOpsLeavesDraftRow(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()

	if err := a.CreateDraft(ctx, "draft-keep", []byte("payload")); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	id := mustInsertPendingSend(t, a, "draft-keep")

	if err := a.CancelOps(ctx, []int64{id}); err != nil {
		t.Fatalf("CancelOps: %v", err)
	}

	if _, err := a.LoadDraft(ctx, "draft-keep"); err != nil {
		t.Errorf("draft was deleted; want kept (err=%v)", err)
	}
}

func TestCancelOpsEmpty(t *testing.T) {
	a := openTestAccount(t)
	if err := a.CancelOps(context.Background(), nil); err != nil {
		t.Fatalf("nil: %v", err)
	}
	if err := a.CancelOps(context.Background(), []int64{}); err != nil {
		t.Fatalf("empty: %v", err)
	}
}

func mustInsertPendingSend(t *testing.T, a *Account, draftID string) int64 {
	t.Helper()
	res, err := a.db.Exec(
		`INSERT INTO outbox(folder, message, kind, args, payload, enqueued_at, status, attempts, draft_id)
		 VALUES (NULL, NULL, ?, ?, x'', 0, ?, 0, ?)`,
		KindSend, `{}`, OpPending, sql.NullString{String: draftID, Valid: draftID != ""})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("lastInsertId: %v", err)
	}
	return id
}
