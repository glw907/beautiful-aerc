// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

func sampleFolders() []FolderEntry {
	return []FolderEntry{
		{Display: "Inbox", Provider: "INBOX", Group: GroupPrimary},
		{Display: "Drafts", Provider: "Drafts", Group: GroupPrimary},
		{Display: "Sent", Provider: "Sent", Group: GroupPrimary},
		{Display: "Archive", Provider: "Archive", Group: GroupDisposal},
		{Display: "Trash", Provider: "Trash", Group: GroupDisposal},
		{Display: "Receipts/2026", Provider: "Receipts/2026", Group: GroupCustom},
		{Display: "Receipts/2025", Provider: "Receipts/2025", Group: GroupCustom},
	}
}

func newTestPicker() MovePicker {
	t := theme.Themes[theme.DefaultThemeName]
	return NewMovePicker(NewStyles(t), t)
}

func TestMovePicker_OpenSetsState(t *testing.T) {
	p := newTestPicker()
	p = p.Open([]mail.UID{"1", "2"}, "INBOX", sampleFolders())
	if !p.IsOpen() {
		t.Fatal("picker should be open after Open")
	}
	// src "INBOX" is excluded; expect one fewer than full list
	if got, want := len(p.all), len(sampleFolders())-1; got != want {
		t.Errorf("all len = %d, want %d", got, want)
	}
	if len(p.matches) != len(p.all) {
		t.Errorf("matches len = %d, want %d (no filter)", len(p.matches), len(p.all))
	}
	if p.cursor != 0 {
		t.Errorf("cursor = %d, want 0", p.cursor)
	}
}

func TestMovePicker_FilterNarrows(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if p.filter != "rec" {
		t.Errorf("filter = %q, want %q", p.filter, "rec")
	}
	if len(p.matches) != 2 {
		t.Errorf("matches = %d, want 2 (Receipts/2026, Receipts/2025)", len(p.matches))
	}
	for _, idx := range p.matches {
		if !strings.Contains(strings.ToLower(p.all[idx].Display), "rec") {
			t.Errorf("match %q does not contain 'rec'", p.all[idx].Display)
		}
	}
}

func TestMovePicker_FilterCaseInsensitive(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'I'}})
	if len(p.matches) == 0 {
		t.Fatal("expected matches for 'I' (Inbox, Receipts), got 0")
	}
}

func TestMovePicker_BackspaceWidens(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.filter != "r" {
		t.Errorf("filter = %q, want %q", p.filter, "r")
	}
}

func TestMovePicker_BackspaceEmptyNoOp(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if p.filter != "" {
		t.Errorf("filter = %q, want empty", p.filter)
	}
}

func TestMovePicker_CursorClampsOnFilter(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	for i := 0; i < 5; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if p.cursor != 5 {
		t.Fatalf("cursor = %d, want 5 (precondition)", p.cursor)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if p.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after filter change", p.cursor)
	}
}

func TestMovePicker_NavigationBounds(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.cursor != 0 {
		t.Errorf("up at top: cursor = %d, want 0", p.cursor)
	}
	for i := 0; i < 100; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if p.cursor != len(p.matches)-1 {
		t.Errorf("down past bottom: cursor = %d, want %d", p.cursor, len(p.matches)-1)
	}
}

func TestMovePicker_EnterEmitsPickedMsg(t *testing.T) {
	// INBOX excluded; p.all = [Drafts, Sent, Archive, Trash, Receipts/2026, Receipts/2025]
	// cursor=0 is Drafts; no Down needed.
	p := newTestPicker().Open([]mail.UID{"42"}, "INBOX", sampleFolders())
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter returned nil cmd")
	}
	msgs := drainBatch(cmd)
	var picked *MovePickerPickedMsg
	var sawClosed bool
	for _, m := range msgs {
		switch v := m.(type) {
		case MovePickerPickedMsg:
			picked = &v
		case MovePickerClosedMsg:
			sawClosed = true
		}
	}
	if picked == nil {
		t.Fatal("did not see MovePickerPickedMsg")
	}
	if !sawClosed {
		t.Error("did not see MovePickerClosedMsg")
	}
	if picked.Dest != "Drafts" {
		t.Errorf("Dest = %q, want %q", picked.Dest, "Drafts")
	}
	if picked.Src != "INBOX" {
		t.Errorf("Src = %q, want %q", picked.Src, "INBOX")
	}
	if len(picked.UIDs) != 1 || picked.UIDs[0] != "42" {
		t.Errorf("UIDs = %v, want [42]", picked.UIDs)
	}
}

func TestMovePicker_EnterInertOnEmpty(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	for _, r := range "zzzzz" {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(p.matches) != 0 {
		t.Fatalf("matches = %d, want 0 (precondition)", len(p.matches))
	}
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Errorf("Enter on empty matches returned non-nil cmd")
	}
}

func TestMovePicker_EscClosesNoOp(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc returned nil cmd")
	}
	msgs := drainBatch(cmd)
	var sawClosed bool
	for _, m := range msgs {
		if _, ok := m.(MovePickerClosedMsg); ok {
			sawClosed = true
		}
		if _, ok := m.(MovePickerPickedMsg); ok {
			t.Error("Esc emitted PickedMsg")
		}
	}
	if !sawClosed {
		t.Error("Esc did not emit ClosedMsg")
	}
}

func TestMovePicker_QSwallowed(t *testing.T) {
	p := newTestPicker().Open([]mail.UID{"1"}, "INBOX", sampleFolders())
	beforeFilter := p.filter
	p2, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Errorf("q produced cmd, want nil (swallowed)")
	}
	if p2.filter != beforeFilter {
		t.Errorf("q modified filter to %q, want unchanged %q", p2.filter, beforeFilter)
	}
}

func drainBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, drainBatch(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}
