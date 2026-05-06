// SPDX-License-Identifier: MIT

package mailimap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

// newWithFake returns a Backend wired to a fake client for tests.
// Construction bypasses the network dial so unit tests don't need
// a live server.
func newWithFake(cfg config.AccountConfig, cmd, idle imapClient) *Backend {
	b := New(cfg)
	b.cmd = cmd
	b.idle = idle
	return b
}

func TestConnectFailsWithoutUIDPLUS(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true} // no UIDPLUS
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	err := b.finishConnect(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "UIDPLUS") {
		t.Errorf("error = %v, want UIDPLUS mention", err)
	}
}

func TestConnectStoresCapabilities(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true, "MOVE": true, "IDLE": true, "SPECIAL-USE": true}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if !b.caps.UIDPLUS || !b.caps.MOVE || !b.caps.IDLE || !b.caps.SpecialUse {
		t.Errorf("caps = %+v", b.caps)
	}
}

func TestDisconnectLogsOutBoth(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	cmd.logoutErr = errors.New("cmd-err")
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	// Disconnect should attempt both even if cmd fails.
	if err := b.Disconnect(); err == nil {
		t.Errorf("expected error from cmd Logout, got nil")
	}
}

func TestFinishConnect_GmailQuirks_RequiresXGM(t *testing.T) {
	cfg := config.AccountConfig{Name: "g", Backend: "imap", GmailQuirks: true}
	b := New(cfg)
	fc := newFakeClient()
	fc.caps = map[string]bool{"UIDPLUS": true} // X-GM-EXT-1 absent
	b.cmd = fc
	b.idle = newFakeClient()

	err := b.finishConnect(context.Background())
	if err == nil {
		t.Fatal("finishConnect with GmailQuirks and no X-GM-EXT-1: want error, got nil")
	}
	if !strings.Contains(err.Error(), "X-GM-EXT-1") {
		t.Errorf("error %q does not mention X-GM-EXT-1", err)
	}
}

func TestFinishConnect_GmailQuirks_AcceptsXGM(t *testing.T) {
	cfg := config.AccountConfig{Name: "g", Backend: "imap", GmailQuirks: true}
	b := New(cfg)
	fc := newFakeClient()
	fc.caps = map[string]bool{"UIDPLUS": true, "X-GM-EXT-1": true}
	b.cmd = fc
	b.idle = newFakeClient()

	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("finishConnect: %v", err)
	}
	// Tear down the idle goroutine started by finishConnect.
	_ = b.Disconnect()
}

func TestPushDraft_IMAP_FirstPush(t *testing.T) {
	cmd := newFakeClient()
	cmd.nextUID = 42
	idle := newFakeClient()
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)

	uid, err := b.PushDraft("Drafts", []byte("draft mime"), mail.UID(""))
	if err != nil {
		t.Fatalf("PushDraft: %v", err)
	}
	if uid != mail.UID("42") {
		t.Errorf("uid = %q, want 42", uid)
	}
	if len(cmd.appendCalls) != 1 {
		t.Fatalf("appendCalls = %d, want 1", len(cmd.appendCalls))
	}
	ac := cmd.appendCalls[0]
	if ac.folder != "Drafts" || string(ac.mime) != "draft mime" || len(ac.flags) != 1 || ac.flags[0] != "\\Draft" {
		t.Errorf("append call = %+v", ac)
	}
	if len(cmd.storeCalls) != 0 || len(cmd.expungeCalls) != 0 {
		t.Errorf("first push expunged prior image: store=%d expunge=%d",
			len(cmd.storeCalls), len(cmd.expungeCalls))
	}
}

func TestPushDraft_IMAP_ReplacesPrev(t *testing.T) {
	cmd := newFakeClient()
	cmd.nextUID = 99
	idle := newFakeClient()
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)

	uid, err := b.PushDraft("Drafts", []byte("v2"), mail.UID("42"))
	if err != nil {
		t.Fatalf("PushDraft: %v", err)
	}
	if uid != mail.UID("99") {
		t.Errorf("uid = %q, want 99", uid)
	}
	if cmd.selected != "Drafts" {
		t.Errorf("selected = %q, want Drafts", cmd.selected)
	}
	if len(cmd.storeCalls) != 1 {
		t.Fatalf("storeCalls = %d, want 1", len(cmd.storeCalls))
	}
	if len(cmd.expungeCalls) != 1 || len(cmd.expungeCalls[0]) != 1 || cmd.expungeCalls[0][0] != mail.UID("42") {
		t.Errorf("expunge calls = %v, want [[42]]", cmd.expungeCalls)
	}
}
