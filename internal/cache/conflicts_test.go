// SPDX-License-Identifier: MIT

package cache

import (
	"context"
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

func TestRevertOptimistic_SendUnsupported(t *testing.T) {
	a := openTestAccount(t)
	tx, _ := a.db.Begin()
	defer tx.Rollback()
	if err := revertOptimisticTx(tx, 1, SendArgs{}); err == nil {
		t.Errorf("expected error for SendArgs revert")
	}
}
