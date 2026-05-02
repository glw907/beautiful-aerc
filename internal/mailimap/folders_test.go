// SPDX-License-Identifier: MIT

package mailimap

import (
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

func TestListFoldersWithSpecialUse(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true, "SPECIAL-USE": true}
	cmd.folders = []listEntry{
		{Name: "INBOX"},
		{Name: "Sent", Attributes: []string{"\\Sent"}},
		{Name: "Trash", Attributes: []string{"\\Trash"}},
		{Name: "Custom"},
	}
	cmd.folderSummary = map[string]mail.Folder{
		"INBOX":  {Name: "INBOX", Exists: 12, Unseen: 3},
		"Sent":   {Name: "Sent", Exists: 1},
		"Trash":  {Name: "Trash"},
		"Custom": {Name: "Custom"},
	}
	idle := newFakeClient()
	idle.caps = cmd.caps

	// Set up state that ListFolders needs without starting the idle
	// goroutine.
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	b.caps = capSet{UIDPLUS: true, SpecialUse: true}

	got, err := b.ListFolders()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	wantRoles := map[string]string{"INBOX": "", "Sent": "sent", "Trash": "trash", "Custom": ""}
	for _, f := range got {
		if got, want := f.Role, wantRoles[f.Name]; got != want {
			t.Errorf("folder %q role = %q, want %q", f.Name, got, want)
		}
	}
}

func TestOpenFolderTracksCurrent(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	idle := newFakeClient()
	idle.caps = cmd.caps

	// Set up state that OpenFolder needs without starting the idle
	// goroutine — finishConnect would spawn it and create a race
	// between the goroutine's b.current write and this test's read.
	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	b.caps = capSet{UIDPLUS: true}
	b.switchCh = make(chan string, 1)

	if err := b.OpenFolder("INBOX"); err != nil {
		t.Fatalf("open: %v", err)
	}
	if cmd.selected != "INBOX" {
		t.Errorf("selected = %q, want INBOX", cmd.selected)
	}

	b.mu.Lock()
	cur := b.current
	b.mu.Unlock()
	if cur != "INBOX" {
		t.Errorf("b.current = %q, want INBOX", cur)
	}
}
