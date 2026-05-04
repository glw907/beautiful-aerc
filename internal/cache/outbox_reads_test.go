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
