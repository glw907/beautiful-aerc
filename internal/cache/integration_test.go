// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

func TestIntegration_TriageRoundTrip(t *testing.T) {
	be := &fakeBackend{
		folders: []mail.Folder{{Name: "INBOX", Role: "inbox"}},
	}
	a, err := Open("integration", be, &fakeChangeTracker{}, t.TempDir(), Config{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer a.Close()
	ctx := context.Background()

	if err := a.SyncFolders(ctx); err != nil {
		t.Fatalf("SyncFolders: %v", err)
	}
	headers := []mail.MessageInfo{
		{UID: "100", Subject: "first", From: "x", SentAt: time.Now(), Flags: 0},
		{UID: "101", Subject: "second", From: "y", SentAt: time.Now().Add(-time.Minute), Flags: 0},
	}
	if err := a.upsertMessages(ctx, "Inbox", headers); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// User triages: star message 100.
	if _, err := a.QueueOp(ctx, "Inbox", "100", FlagArgs{Flag: mail.FlagFlagged, Set: true}); err != nil {
		t.Fatalf("QueueOp: %v", err)
	}

	// Optimistic state immediately visible.
	got, _, err := a.QueryFolder("Inbox", 0, 10)
	if err != nil {
		t.Fatalf("QueryFolder before drain: %v", err)
	}
	var foundOptimistic bool
	for _, m := range got {
		if m.UID == "100" && m.Flags&mail.FlagFlagged != 0 {
			foundOptimistic = true
		}
	}
	if !foundOptimistic {
		t.Fatal("optimistic ui_flags not reflected in QueryFolder pre-drain")
	}

	// Start the drainer; wait for the done event.
	if err := a.StartDrainer(ctx); err != nil {
		t.Fatalf("StartDrainer: %v", err)
	}
	select {
	case ev := <-a.Events():
		if ev.Status != "done" {
			t.Fatalf("event status = %q, want done; err=%q", ev.Status, ev.Err)
		}
		if ev.Kind != "flag" {
			t.Errorf("event kind = %q, want flag", ev.Kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for drainer event")
	}

	if len(be.flags) != 1 || be.flags[0] != "100" {
		t.Errorf("backend.Flag calls = %v, want [100]", be.flags)
	}

	// Confirm flags converged.
	var serverFlags, uiFlags uint32
	a.db.QueryRow(`SELECT flags, ui_flags FROM messages WHERE protocol_id = '100'`).Scan(&serverFlags, &uiFlags)
	if mail.Flag(serverFlags)&mail.FlagFlagged == 0 || mail.Flag(uiFlags)&mail.FlagFlagged == 0 {
		t.Errorf("flags = %#x, ui_flags = %#x; expected both to carry FlagFlagged after success", serverFlags, uiFlags)
	}

	// Outbox row reaped to done.
	var status string
	a.db.QueryRow(`SELECT status FROM outbox WHERE id = 1`).Scan(&status)
	if status != "done" {
		t.Errorf("outbox status = %q, want done", status)
	}
}

// TestIntegration_CrashRecovery exercises the spec §D.2 contract.
func TestIntegration_CrashRecovery(t *testing.T) {
	dir := t.TempDir()
	be := &fakeBackend{folders: []mail.Folder{{Name: "INBOX", Role: "inbox"}}}

	a, err := Open("crash", be, &fakeChangeTracker{}, dir, Config{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ctx := context.Background()
	if err := a.SyncFolders(ctx); err != nil {
		t.Fatalf("SyncFolders: %v", err)
	}
	a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{{UID: "1"}, {UID: "2"}})
	flagOp, _ := a.QueueOp(ctx, "Inbox", "1", FlagArgs{Flag: mail.FlagSeen, Set: true})
	moveOp, _ := a.QueueOp(ctx, "Inbox", "2", MoveArgs{Dest: "Trash"})
	// Insert a synthetic send op (Pass 9 will queue these for real).
	folderID, _ := a.folderID("Inbox")
	res, err := a.db.Exec(`INSERT INTO outbox (folder, kind, args, enqueued_at, status) VALUES (?, 'send', '{}', ?, 'pending')`,
		folderID, time.Now().UnixNano())
	if err != nil {
		t.Fatalf("seed send: %v", err)
	}
	sendOp, _ := res.LastInsertId()

	// Simulate a crash mid-execute on every queued op.
	if _, err := a.db.Exec(`UPDATE outbox SET status = 'executing'`); err != nil {
		t.Fatalf("simulate crash: %v", err)
	}
	a.Close()

	// Re-open: recoverExecuting fires inside StartDrainer.
	a2, err := Open("crash", be, &fakeChangeTracker{}, dir, Config{})
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	defer a2.Close()
	if err := a2.recoverExecuting(); err != nil {
		t.Fatalf("recoverExecuting: %v", err)
	}

	check := func(opID int64, wantStatus string) {
		t.Helper()
		var status string
		a2.db.QueryRow(`SELECT status FROM outbox WHERE id = ?`, opID).Scan(&status)
		if status != wantStatus {
			t.Errorf("op %d status = %q, want %q", opID, status, wantStatus)
		}
	}
	check(flagOp, "pending")
	check(moveOp, "pending")
	check(sendOp, "conflict")
}
