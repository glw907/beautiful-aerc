// SPDX-License-Identifier: MIT

package mailimap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/config"
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
