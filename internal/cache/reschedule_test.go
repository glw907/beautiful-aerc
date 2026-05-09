package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

func TestRescheduleOp_UpdatesPendingRow(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	original := time.Now().Add(1 * time.Hour).UnixNano()
	id, err := a.QueueSend(ctx, "Inbox", testEnvelope(), []byte("MIME"), original, "")
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	want := time.Now().Add(2 * time.Hour).UnixNano()
	if err := a.RescheduleOp(ctx, id, want); err != nil {
		t.Fatalf("reschedule: %v", err)
	}
	got := readScheduledFor(t, a, id)
	if got != want {
		t.Errorf("scheduled_for: got %d, want %d", got, want)
	}
}

func TestRescheduleOp_RowAboutToDispatch(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	past := time.Now().Add(-1 * time.Second).UnixNano()
	id, err := a.QueueSend(ctx, "Inbox", testEnvelope(), []byte("MIME"), past, "")
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	err = a.RescheduleOp(ctx, id, time.Now().Add(1*time.Hour).UnixNano())
	if !errors.Is(err, ErrNotPending) {
		t.Errorf("got %v, want ErrNotPending", err)
	}
}

func TestRescheduleOp_AdvancedRow(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	id, err := a.QueueSend(ctx, "Inbox", testEnvelope(), []byte("MIME"),
		time.Now().Add(1*time.Hour).UnixNano(), "")
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	markStatus(t, a, id, OpDone)
	err = a.RescheduleOp(ctx, id, time.Now().Add(2*time.Hour).UnixNano())
	if !errors.Is(err, ErrNotPending) {
		t.Errorf("got %v, want ErrNotPending", err)
	}
}

func testEnvelope() mail.Envelope {
	return mail.Envelope{From: "geoff@907.life", Rcpts: []string{"x@example.com"}}
}

func readScheduledFor(t *testing.T, a *Account, id int64) int64 {
	t.Helper()
	var v int64
	if err := a.db.QueryRow(`SELECT scheduled_for FROM outbox WHERE id = ?`, id).Scan(&v); err != nil {
		t.Fatalf("readScheduledFor: %v", err)
	}
	return v
}

func markStatus(t *testing.T, a *Account, id int64, status OpStatus) {
	t.Helper()
	mustExec(t, a.db, `UPDATE outbox SET status = ? WHERE id = ?`, status, id)
}
