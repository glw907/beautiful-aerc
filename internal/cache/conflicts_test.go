package cache

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

// seedConflictRow is shared scaffolding: upsert a message, queue an
// op (which applies the optimistic flip), then force the outbox row
// into conflict state and return its id.
func seedConflictRow(t *testing.T, a *Account, uid mail.UID, args OpArgs) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	if err := a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{
		{UID: uid, Subject: "x", SentAt: time.Now()},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	opID, err := a.QueueOp(ctx, "Inbox", uid, args)
	if err != nil {
		t.Fatalf("QueueOp: %v", err)
	}
	if _, err := a.db.Exec(`UPDATE outbox SET status = ? WHERE id = ?`,
		OpConflict, opID); err != nil {
		t.Fatalf("force conflict: %v", err)
	}
	var msgID int64
	if err := a.db.QueryRow(`SELECT id FROM messages WHERE protocol_id = ?`,
		string(uid)).Scan(&msgID); err != nil {
		t.Fatalf("scan msgID: %v", err)
	}
	return opID, msgID
}

func TestRevertOptimistic_FlagSet(t *testing.T) {
	a := openTestAccount(t)
	_, msgID := seedConflictRow(t, a, "f1", FlagArgs{Flag: mail.FlagFlagged, Set: true})

	// Verify forward flip happened.
	var ui uint32
	a.db.QueryRow(`SELECT ui_flags FROM messages WHERE id = ?`, msgID).Scan(&ui)
	if mail.Flag(ui)&mail.FlagFlagged == 0 {
		t.Fatalf("forward flip missing")
	}

	tx, _ := a.db.Begin()
	if err := revertOptimisticTx(tx, msgID, FlagArgs{Flag: mail.FlagFlagged, Set: true}); err != nil {
		t.Fatalf("revert: %v", err)
	}
	tx.Commit()

	a.db.QueryRow(`SELECT ui_flags FROM messages WHERE id = ?`, msgID).Scan(&ui)
	if mail.Flag(ui)&mail.FlagFlagged != 0 {
		t.Errorf("ui_flags still has FlagFlagged after revert: %d", ui)
	}
}

func TestRevertOptimistic_FlagClear(t *testing.T) {
	a := openTestAccount(t)
	// Pre-set the flag, then queue a Set:false op (which clears).
	ctx := context.Background()
	a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{
		{UID: "f2", Subject: "x", Flags: mail.FlagFlagged, SentAt: time.Now()},
	})
	a.db.Exec(`UPDATE messages SET ui_flags = ? WHERE protocol_id = ?`,
		uint32(mail.FlagFlagged), "f2")
	opID, err := a.QueueOp(ctx, "Inbox", "f2", FlagArgs{Flag: mail.FlagFlagged, Set: false})
	if err != nil {
		t.Fatalf("QueueOp: %v", err)
	}
	a.db.Exec(`UPDATE outbox SET status = ? WHERE id = ?`, OpConflict, opID)

	var msgID int64
	a.db.QueryRow(`SELECT id FROM messages WHERE protocol_id = 'f2'`).Scan(&msgID)

	tx, _ := a.db.Begin()
	if err := revertOptimisticTx(tx, msgID, FlagArgs{Flag: mail.FlagFlagged, Set: false}); err != nil {
		t.Fatalf("revert: %v", err)
	}
	tx.Commit()

	var ui uint32
	a.db.QueryRow(`SELECT ui_flags FROM messages WHERE id = ?`, msgID).Scan(&ui)
	if mail.Flag(ui)&mail.FlagFlagged == 0 {
		t.Errorf("ui_flags lost FlagFlagged after revert of clear: %d", ui)
	}
}

func TestRevertOptimistic_Move(t *testing.T) {
	a := openTestAccount(t)
	_, msgID := seedConflictRow(t, a, "m1", MoveArgs{Dest: "Archive"})
	var hide int
	a.db.QueryRow(`SELECT ui_hide FROM messages WHERE id = ?`, msgID).Scan(&hide)
	if hide != 1 {
		t.Fatalf("forward ui_hide flip missing")
	}
	tx, _ := a.db.Begin()
	if err := revertOptimisticTx(tx, msgID, MoveArgs{Dest: "Archive"}); err != nil {
		t.Fatalf("revert: %v", err)
	}
	tx.Commit()
	a.db.QueryRow(`SELECT ui_hide FROM messages WHERE id = ?`, msgID).Scan(&hide)
	if hide != 0 {
		t.Errorf("ui_hide = %d after revert, want 0", hide)
	}
}

func TestRevertOptimistic_Destroy(t *testing.T) {
	a := openTestAccount(t)
	_, msgID := seedConflictRow(t, a, "d1", DestroyArgs{})
	tx, _ := a.db.Begin()
	if err := revertOptimisticTx(tx, msgID, DestroyArgs{}); err != nil {
		t.Fatalf("revert: %v", err)
	}
	tx.Commit()
	var hide int
	a.db.QueryRow(`SELECT ui_hide FROM messages WHERE id = ?`, msgID).Scan(&hide)
	if hide != 0 {
		t.Errorf("ui_hide = %d after revert, want 0", hide)
	}
}

func TestRevertOptimistic_SendNoOp(t *testing.T) {
	a := openTestAccount(t)
	tx, _ := a.db.Begin()
	defer tx.Rollback()
	// Send and Append have no message-row state. Revert is a no-op.
	if err := revertOptimisticTx(tx, 1, SendArgs{}); err != nil {
		t.Errorf("revertOptimisticTx(SendArgs): unexpected error: %v", err)
	}
	if err := revertOptimisticTx(tx, 1, AppendArgs{}); err != nil {
		t.Errorf("revertOptimisticTx(AppendArgs): unexpected error: %v", err)
	}
}

func TestRetryOp_Reset(t *testing.T) {
	a := openTestAccount(t)
	opID, _ := seedConflictRow(t, a, "r1", FlagArgs{Flag: mail.FlagSeen, Set: true})
	a.db.Exec(`UPDATE outbox SET attempts = 7, next_eligible_at = ?, error = 'x' WHERE id = ?`,
		time.Now().Add(time.Hour).UnixNano(), opID)

	if err := a.RetryOp(context.Background(), opID); err != nil {
		t.Fatalf("RetryOp: %v", err)
	}

	var status string
	var attempts int
	var nextAt sql.NullInt64
	var errStr string
	if err := a.db.QueryRow(`SELECT status, attempts, next_eligible_at, error FROM outbox WHERE id = ?`,
		opID).Scan(&status, &attempts, &nextAt, &errStr); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if OpStatus(status) != OpPending {
		t.Errorf("status = %q, want pending", status)
	}
	if attempts != 0 {
		t.Errorf("attempts = %d, want 0", attempts)
	}
	if nextAt.Valid {
		t.Errorf("next_eligible_at not cleared: %v", nextAt)
	}
	if errStr != "" {
		t.Errorf("error not cleared: %q", errStr)
	}
}

func TestRetryOp_NonConflict(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{{UID: "n1", Subject: "x"}})
	opID, _ := a.QueueOp(ctx, "Inbox", "n1", FlagArgs{Flag: mail.FlagSeen, Set: true})
	// op is in OpPending, not OpConflict.

	err := a.RetryOp(ctx, opID)
	if !errors.Is(err, ErrNotConflict) {
		t.Errorf("RetryOp on pending row: err = %v, want ErrNotConflict", err)
	}
}

func TestRetryOp_Signal(t *testing.T) {
	a := openTestAccount(t)
	opID, _ := seedConflictRow(t, a, "s1", FlagArgs{Flag: mail.FlagSeen, Set: true})

	// Drain the signal channel first so we know any signal we observe came from RetryOp.
	select {
	case <-a.drainSignal:
	default:
	}

	if err := a.RetryOp(context.Background(), opID); err != nil {
		t.Fatalf("RetryOp: %v", err)
	}
	select {
	case <-a.drainSignal:
		// good
	case <-time.After(100 * time.Millisecond):
		t.Errorf("RetryOp did not signal drainer")
	}
}

func TestDiscardOp_Revert(t *testing.T) {
	a := openTestAccount(t)
	opID, msgID := seedConflictRow(t, a, "x1", FlagArgs{Flag: mail.FlagFlagged, Set: true})

	if err := a.DiscardOp(context.Background(), opID); err != nil {
		t.Fatalf("DiscardOp: %v", err)
	}

	// Outbox row gone.
	var n int
	a.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE id = ?`, opID).Scan(&n)
	if n != 0 {
		t.Errorf("outbox row not deleted")
	}
	// Optimistic flip reverted.
	var ui uint32
	a.db.QueryRow(`SELECT ui_flags FROM messages WHERE id = ?`, msgID).Scan(&ui)
	if mail.Flag(ui)&mail.FlagFlagged != 0 {
		t.Errorf("ui_flags still flagged after discard")
	}
}

func TestDiscardOp_Move(t *testing.T) {
	a := openTestAccount(t)
	opID, msgID := seedConflictRow(t, a, "x2", MoveArgs{Dest: "Archive"})

	if err := a.DiscardOp(context.Background(), opID); err != nil {
		t.Fatalf("DiscardOp: %v", err)
	}
	var hide int
	a.db.QueryRow(`SELECT ui_hide FROM messages WHERE id = ?`, msgID).Scan(&hide)
	if hide != 0 {
		t.Errorf("ui_hide = %d after discard, want 0", hide)
	}
}

func TestDiscardOp_NonConflict(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{{UID: "x3", Subject: "x"}})
	opID, _ := a.QueueOp(ctx, "Inbox", "x3", FlagArgs{Flag: mail.FlagSeen, Set: true})

	err := a.DiscardOp(ctx, opID)
	if !errors.Is(err, ErrNotConflict) {
		t.Errorf("DiscardOp on pending row: err = %v, want ErrNotConflict", err)
	}
}

func TestDiscardConflictedSend(t *testing.T) {
	a := openTestAccount(t)
	defer a.Close()
	fb := a.Backend.(*fakeBackend)

	fb.sendErr = mail.ErrAuth
	opID, err := a.QueueSend(context.Background(), "Inbox",
		mail.Envelope{From: "geoff@907.life", Rcpts: []string{"a@example.com"}},
		[]byte("body\r\n"))
	if err != nil {
		t.Fatalf("QueueSend: %v", err)
	}

	a.drainOnce(context.Background(), defaultDrainerConfig())

	// Sanity: row is in conflict.
	var status string
	if err := a.db.QueryRow(`SELECT status FROM outbox WHERE id = ?`, opID).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if OpStatus(status) != OpConflict {
		t.Fatalf("status = %q, want conflict", status)
	}

	if err := a.DiscardOp(context.Background(), opID); err != nil {
		t.Fatalf("DiscardOp: %v", err)
	}

	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE id = ?`, opID).Scan(&n); err != nil {
		t.Fatalf("count row: %v", err)
	}
	if n != 0 {
		t.Errorf("row count = %d, want 0", n)
	}
}
