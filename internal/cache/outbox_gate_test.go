package cache

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// queueLinkedSendAppend inserts a Send + Append pair sharing draftID,
// returning the two op ids. Both pending.
func queueLinkedSendAppend(t *testing.T, a *Account, draftID string) (sendID, appendID int64) {
	t.Helper()
	if err := a.CreateDraft(context.Background(), draftID, []byte("draft-payload")); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	env := mail.Envelope{From: "geoff@907.life", Rcpts: []string{"a@example.com"}}
	mime := []byte("hello\r\n")
	var err error
	sendID, err = a.QueueSend(context.Background(), "Inbox", env, mime, 0, draftID)
	if err != nil {
		t.Fatalf("QueueSend: %v", err)
	}
	appendID, err = a.QueueAppend(context.Background(), "Inbox", mail.FlagSeen, mime, 0, draftID)
	if err != nil {
		t.Fatalf("QueueAppend: %v", err)
	}
	return sendID, appendID
}

func TestNextOutboxRow_AppendGatedByPendingSend(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	sendID, appendID := queueLinkedSendAppend(t, a, "draft-pending")

	row, err := a.nextOutboxRow(time.Now())
	if err != nil {
		t.Fatalf("nextOutboxRow: %v", err)
	}
	if row.ID != sendID {
		t.Fatalf("nextOutboxRow.ID = %d, want Send id %d (got append id %d)", row.ID, sendID, appendID)
	}
	if row.Kind != string(KindSend) {
		t.Errorf("kind = %q, want send", row.Kind)
	}
}

func TestNextOutboxRow_AppendGatedByFailedSend(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	sendID, _ := queueLinkedSendAppend(t, a, "draft-failed")

	// Force the Send to OpFailed with an expired backoff window: it
	// is eligible to retry. Append must remain gated.
	past := time.Now().Add(-time.Second).UnixNano()
	mustExec(t, a.db,
		`UPDATE outbox SET status = ?, next_eligible_at = ? WHERE id = ?`,
		OpFailed, past, sendID)

	row, err := a.nextOutboxRow(time.Now())
	if err != nil {
		t.Fatalf("nextOutboxRow: %v", err)
	}
	if row.ID != sendID {
		t.Errorf("nextOutboxRow.ID = %d, want Send id %d", row.ID, sendID)
	}
}

func TestNextOutboxRow_AppendReleasedAfterSendDone(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	sendID, appendID := queueLinkedSendAppend(t, a, "draft-done")

	mustExec(t, a.db, `UPDATE outbox SET status = ? WHERE id = ?`, OpDone, sendID)

	row, err := a.nextOutboxRow(time.Now())
	if err != nil {
		t.Fatalf("nextOutboxRow: %v", err)
	}
	if row.ID != appendID {
		t.Errorf("nextOutboxRow.ID = %d, want Append id %d", row.ID, appendID)
	}
}

func TestNextOutboxRow_AppendStrandedWhenSendAbsent(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	sendID, _ := queueLinkedSendAppend(t, a, "draft-orphan")

	// Send vanishes (simulates Discard without cascade).
	mustExec(t, a.db, `DELETE FROM outbox WHERE id = ?`, sendID)

	_, err := a.nextOutboxRow(time.Now())
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("orphan Append must remain ineligible; got err=%v", err)
	}
}

func TestNextOutboxRow_AppendWithoutDraftIDNotGated(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	// Lone Append, no draft link.
	if _, err := a.QueueAppend(context.Background(), "Inbox", mail.FlagSeen, []byte("body"), 0, ""); err != nil {
		t.Fatalf("QueueAppend: %v", err)
	}

	row, err := a.nextOutboxRow(time.Now())
	if err != nil {
		t.Fatalf("nextOutboxRow: %v", err)
	}
	if row.Kind != string(KindAppend) {
		t.Errorf("kind = %q, want append", row.Kind)
	}
}

func TestDiscardOp_Send_CascadesToAppend(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()

	sendID, appendID := queueLinkedSendAppend(t, a, "draft-cascade")

	// Push Send into conflict so DiscardOp accepts it.
	mustExec(t, a.db,
		`UPDATE outbox SET status = ?, error = '{"kind":"max-attempts-exceeded"}' WHERE id = ?`,
		OpConflict, sendID)

	if err := a.DiscardOp(context.Background(), sendID); err != nil {
		t.Fatalf("DiscardOp: %v", err)
	}

	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE id IN (?, ?)`, sendID, appendID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("DiscardOp(Send) left %d rows, want 0 (cascade should remove sibling Append)", count)
	}
}

// TestQueuedSendFailure_AppendHeldOff drives the full drainer pickup
// loop: when Send fails transiently and Append shares the same
// draft_id, the drainer must never dispatch the Append.
func TestQueuedSendFailure_AppendHeldOff(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	fb := a.Backend.(*fakeBackend)

	// Send returns a generic transient error.
	fb.sendErr = errors.New("transient send failure")

	_, _ = queueLinkedSendAppend(t, a, "draft-soakbug")

	a.drainOnce(context.Background(), defaultDrainerConfig())

	if len(fb.sends) != 1 {
		t.Errorf("backend Send calls = %d, want 1", len(fb.sends))
	}
	if len(fb.appends) != 0 {
		t.Errorf("backend Append calls = %d, want 0 — Append must stay gated while Send is non-Done", len(fb.appends))
	}
}
