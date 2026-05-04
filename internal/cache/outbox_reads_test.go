// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

func TestOutboxDepth_Empty(t *testing.T) {
	a := openTestAccount(t)
	d, err := a.OutboxDepth(context.Background())
	if err != nil {
		t.Fatalf("OutboxDepth: %v", err)
	}
	if d != (OutboxDepth{}) {
		t.Errorf("empty: %+v, want zero", d)
	}
}

func TestOutboxDepth_Mixed(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{
		{UID: "1", Subject: "x", SentAt: time.Now()},
		{UID: "2", Subject: "y", SentAt: time.Now()},
		{UID: "3", Subject: "z", SentAt: time.Now()},
		{UID: "4", Subject: "w", SentAt: time.Now()},
	})
	a.QueueOp(ctx, "Inbox", "1", FlagArgs{Flag: mail.FlagSeen, Set: true})
	a.QueueOp(ctx, "Inbox", "2", FlagArgs{Flag: mail.FlagSeen, Set: true})
	id3, _ := a.QueueOp(ctx, "Inbox", "3", MoveArgs{Dest: "Archive"})
	id4, _ := a.QueueOp(ctx, "Inbox", "4", DestroyArgs{})
	a.db.Exec(`UPDATE outbox SET status = ? WHERE id = ?`, OpFailed, id3)
	a.db.Exec(`UPDATE outbox SET status = ? WHERE id = ?`, OpConflict, id4)

	d, err := a.OutboxDepth(ctx)
	if err != nil {
		t.Fatalf("OutboxDepth: %v", err)
	}
	want := OutboxDepth{Pending: 2, Failed: 1, Conflict: 1}
	if d != want {
		t.Errorf("got %+v, want %+v", d, want)
	}
}

func TestOutboxSummary_Empty(t *testing.T) {
	a := openTestAccount(t)
	gs, err := a.OutboxSummary(context.Background())
	if err != nil {
		t.Fatalf("OutboxSummary: %v", err)
	}
	if len(gs) != 0 {
		t.Errorf("empty: got %d groups, want 0", len(gs))
	}
}

func TestOutboxSummary_GroupsByKindFolderStatus(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{
		{UID: "1", SentAt: time.Now()}, {UID: "2", SentAt: time.Now()},
		{UID: "3", SentAt: time.Now()}, {UID: "4", SentAt: time.Now()},
	})
	a.QueueOp(ctx, "Inbox", "1", MoveArgs{Dest: "Archive"})
	a.QueueOp(ctx, "Inbox", "2", MoveArgs{Dest: "Archive"})
	a.QueueOp(ctx, "Inbox", "3", FlagArgs{Flag: mail.FlagSeen, Set: true})
	id4, _ := a.QueueOp(ctx, "Inbox", "4", FlagArgs{Flag: mail.FlagSeen, Set: true})
	a.db.Exec(`UPDATE outbox SET status = ? WHERE id = ?`, OpExecuting, id4)

	gs, err := a.OutboxSummary(ctx)
	if err != nil {
		t.Fatalf("OutboxSummary: %v", err)
	}

	if len(gs) != 3 {
		t.Fatalf("got %d groups, want 3: %+v", len(gs), gs)
	}
	if gs[0].Status != OpExecuting || gs[0].Kind != KindFlag || gs[0].Count != 1 {
		t.Errorf("group[0] = %+v", gs[0])
	}
	if gs[1].Status != OpPending || gs[1].Kind != KindFlag || gs[1].Count != 1 {
		t.Errorf("group[1] = %+v", gs[1])
	}
	if gs[2].Status != OpPending || gs[2].Kind != KindMove || gs[2].Count != 2 {
		t.Errorf("group[2] = %+v", gs[2])
	}
}

func TestOutboxSummary_FailedCarriesNextAt(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{
		{UID: "1", SentAt: time.Now()}, {UID: "2", SentAt: time.Now()},
	})
	id1, _ := a.QueueOp(ctx, "Inbox", "1", FlagArgs{Flag: mail.FlagSeen, Set: true})
	id2, _ := a.QueueOp(ctx, "Inbox", "2", FlagArgs{Flag: mail.FlagSeen, Set: true})
	t1 := time.Now().Add(20 * time.Second).UnixNano()
	t2 := time.Now().Add(5 * time.Second).UnixNano()
	a.db.Exec(`UPDATE outbox SET status = ?, next_eligible_at = ? WHERE id = ?`, OpFailed, t1, id1)
	a.db.Exec(`UPDATE outbox SET status = ?, next_eligible_at = ? WHERE id = ?`, OpFailed, t2, id2)

	gs, err := a.OutboxSummary(ctx)
	if err != nil {
		t.Fatalf("OutboxSummary: %v", err)
	}
	if len(gs) != 1 {
		t.Fatalf("got %d groups, want 1", len(gs))
	}
	if !gs[0].NextAt.Valid || gs[0].NextAt.Int64 != t2 {
		t.Errorf("NextAt = %+v, want valid %d (MIN)", gs[0].NextAt, t2)
	}
}

func TestOutboxConflicts_Empty(t *testing.T) {
	a := openTestAccount(t)
	rs, err := a.OutboxConflicts(context.Background())
	if err != nil {
		t.Fatalf("OutboxConflicts: %v", err)
	}
	if len(rs) != 0 {
		t.Errorf("empty: got %d, want 0", len(rs))
	}
}

func TestOutboxConflicts_DecodesError(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{
		{UID: "c1", Subject: "x", SentAt: time.Now()},
	})
	opID, _ := a.QueueOp(ctx, "Inbox", "c1", FlagArgs{Flag: mail.FlagSeen, Set: true})
	a.db.Exec(`UPDATE outbox SET status = ?, attempts = 5,
		error = '{"kind":"auth-failure","message":"invalid creds"}' WHERE id = ?`,
		OpConflict, opID)

	rs, err := a.OutboxConflicts(ctx)
	if err != nil {
		t.Fatalf("OutboxConflicts: %v", err)
	}
	if len(rs) != 1 {
		t.Fatalf("got %d conflicts, want 1", len(rs))
	}
	r := rs[0]
	if r.ID != opID || r.Kind != KindFlag || r.Folder != "Inbox" || r.ProtocolID != "c1" {
		t.Errorf("row mismatch: %+v", r)
	}
	if r.ErrorKind != "auth-failure" || r.ErrorMessage != "invalid creds" {
		t.Errorf("error decode: kind=%q msg=%q", r.ErrorKind, r.ErrorMessage)
	}
	if r.Attempts != 5 {
		t.Errorf("attempts = %d, want 5", r.Attempts)
	}
	if r.EnqueuedAt.IsZero() {
		t.Errorf("EnqueuedAt zero")
	}
}

func TestOutboxConflicts_OrderByEnqueuedAtASC(t *testing.T) {
	a := openTestAccount(t)
	ctx := context.Background()
	a.upsertMessages(ctx, "Inbox", []mail.MessageInfo{
		{UID: "o1", SentAt: time.Now()}, {UID: "o2", SentAt: time.Now()},
	})
	id1, _ := a.QueueOp(ctx, "Inbox", "o1", FlagArgs{Flag: mail.FlagSeen, Set: true})
	id2, _ := a.QueueOp(ctx, "Inbox", "o2", FlagArgs{Flag: mail.FlagSeen, Set: true})
	a.db.Exec(`UPDATE outbox SET enqueued_at = ?, status = ? WHERE id = ?`,
		time.Now().Add(-time.Hour).UnixNano(), OpConflict, id2)
	a.db.Exec(`UPDATE outbox SET status = ? WHERE id = ?`, OpConflict, id1)

	rs, err := a.OutboxConflicts(ctx)
	if err != nil || len(rs) != 2 {
		t.Fatalf("OutboxConflicts: rs=%v err=%v", rs, err)
	}
	if rs[0].ID != id2 || rs[1].ID != id1 {
		t.Errorf("order: got [%d,%d], want [%d,%d]", rs[0].ID, rs[1].ID, id2, id1)
	}
}
