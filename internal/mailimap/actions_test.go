// SPDX-License-Identifier: MIT

package mailimap

import (
	"context"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

func TestMoveUsesUIDMoveWhenAdvertised(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true, "MOVE": true}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.Move([]mail.UID{"1", "2"}, "Trash"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if len(cmd.moveCalls) != 1 {
		t.Fatalf("moveCalls = %d, want 1", len(cmd.moveCalls))
	}
	if len(cmd.copyCalls) != 0 {
		t.Errorf("copyCalls should be 0 with MOVE advertised, got %d", len(cmd.copyCalls))
	}
}

func TestMoveFallsBackToCopyExpungeWithoutMOVE(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true} // no MOVE
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.Move([]mail.UID{"1", "2"}, "Trash"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if len(cmd.copyCalls) != 1 {
		t.Errorf("copyCalls = %d, want 1", len(cmd.copyCalls))
	}
	if len(cmd.storeCalls) != 1 {
		t.Errorf("storeCalls = %d, want 1", len(cmd.storeCalls))
	}
	if len(cmd.expungeCalls) != 1 {
		t.Errorf("expungeCalls = %d, want 1", len(cmd.expungeCalls))
	}
}

func TestMoveEmptyIsNoOp(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true, "MOVE": true}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.Move(nil, "Trash"); err != nil {
		t.Errorf("Move(nil) = %v, want nil", err)
	}
	if len(cmd.moveCalls) != 0 || len(cmd.copyCalls) != 0 {
		t.Errorf("Move(nil) should not call move/copy")
	}
}

func TestDestroyEmptyIsNoOp(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.Destroy(nil); err != nil {
		t.Errorf("Destroy(nil) = %v, want nil", err)
	}
	if len(cmd.storeCalls) != 0 || len(cmd.expungeCalls) != 0 {
		t.Errorf("Destroy(nil) should not call store/expunge")
	}
}

func TestDestroyStoresDeletedThenExpunges(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.Destroy([]mail.UID{"7", "8"}); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if len(cmd.storeCalls) != 1 || len(cmd.expungeCalls) != 1 {
		t.Errorf("expected one store + one expunge, got %d / %d",
			len(cmd.storeCalls), len(cmd.expungeCalls))
	}
}

func TestFlagSetAddsFlag(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.Flag([]mail.UID{"1"}, mail.FlagSeen, true); err != nil {
		t.Fatalf("flag: %v", err)
	}
	if len(cmd.storeCalls) != 1 {
		t.Fatalf("storeCalls = %d, want 1", len(cmd.storeCalls))
	}
	item := cmd.storeCalls[0][1].(string)
	if item != "+FLAGS.SILENT" {
		t.Errorf("item = %q, want +FLAGS.SILENT", item)
	}
}

func TestFlagClearRemovesFlag(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.Flag([]mail.UID{"1"}, mail.FlagSeen, false); err != nil {
		t.Fatalf("flag: %v", err)
	}
	if len(cmd.storeCalls) != 1 {
		t.Fatalf("storeCalls = %d, want 1", len(cmd.storeCalls))
	}
	item := cmd.storeCalls[0][1].(string)
	if item != "-FLAGS.SILENT" {
		t.Errorf("item = %q, want -FLAGS.SILENT", item)
	}
}

func TestMarkReadCallsFlag(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.MarkRead([]mail.UID{"5"}); err != nil {
		t.Fatalf("markread: %v", err)
	}
	if len(cmd.storeCalls) != 1 {
		t.Errorf("storeCalls = %d, want 1", len(cmd.storeCalls))
	}
}

func TestMarkUnreadCallsFlag(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.MarkUnread([]mail.UID{"5"}); err != nil {
		t.Fatalf("markunread: %v", err)
	}
	if len(cmd.storeCalls) != 1 {
		t.Errorf("storeCalls = %d, want 1", len(cmd.storeCalls))
	}
}

func TestMarkAnsweredCallsFlag(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.MarkAnswered([]mail.UID{"5"}); err != nil {
		t.Fatalf("markanswered: %v", err)
	}
	if len(cmd.storeCalls) != 1 {
		t.Errorf("storeCalls = %d, want 1", len(cmd.storeCalls))
	}
}

func TestImapFlagsFor(t *testing.T) {
	tests := []struct {
		name  string
		flag  mail.Flag
		want  []string
	}{
		{"seen", mail.FlagSeen, []string{"\\Seen"}},
		{"answered", mail.FlagAnswered, []string{"\\Answered"}},
		{"flagged", mail.FlagFlagged, []string{"\\Flagged"}},
		{"deleted", mail.FlagDeleted, []string{"\\Deleted"}},
		{"draft", mail.FlagDraft, []string{"\\Draft"}},
		{"seen+flagged", mail.FlagSeen | mail.FlagFlagged, []string{"\\Seen", "\\Flagged"}},
		{"none", 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imapFlagsFor(tt.flag)
			if len(got) != len(tt.want) {
				t.Fatalf("imapFlagsFor(%d) = %v, want %v", tt.flag, got, tt.want)
			}
			for i, v := range tt.want {
				if got[i] != v {
					t.Errorf("[%d] = %q, want %q", i, got[i], v)
				}
			}
		})
	}
}

func TestDeleteResolvesTrashFolder(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true, "MOVE": true}
	cmd.folders = []listEntry{
		{Name: "INBOX"},
		{Name: "Trash", Attributes: []string{"\\Trash"}},
	}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.Delete([]mail.UID{"3"}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(cmd.moveCalls) != 1 {
		t.Fatalf("moveCalls = %d, want 1", len(cmd.moveCalls))
	}
	dest := cmd.moveCalls[0][1].(string)
	if dest != "Trash" {
		t.Errorf("dest = %q, want Trash", dest)
	}
}

func TestDeleteNoTrashFolderReturnsError(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	cmd.folders = []listEntry{
		{Name: "INBOX"},
	}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.Delete([]mail.UID{"3"}); err == nil {
		t.Errorf("Delete with no Trash folder should return error")
	}
}
