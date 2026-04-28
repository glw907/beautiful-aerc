// SPDX-License-Identifier: MIT

package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/poplar/internal/theme"
)

func newTestLinkPicker(t *testing.T) LinkPicker {
	t.Helper()
	styles := NewStyles(theme.Nord)
	p := NewLinkPicker(styles, theme.Nord)
	p = p.SetSize(80, 24)
	return p
}

func TestLinkPickerOpenSetsCursor(t *testing.T) {
	p := newTestLinkPicker(t)
	links := []string{"https://a.com", "https://b.com", "https://c.com"}
	p = p.Open(links)
	if !p.IsOpen() {
		t.Fatal("picker should be open after Open()")
	}
	if p.Cursor() != 0 {
		t.Fatalf("cursor = %d, want 0", p.Cursor())
	}
}

func TestLinkPickerCursorBounds(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com", "https://b.com"})

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if p.Cursor() != 0 {
		t.Fatalf("k from row 0: cursor = %d, want 0", p.Cursor())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if p.Cursor() != 1 {
		t.Fatalf("j from row 0: cursor = %d, want 1", p.Cursor())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if p.Cursor() != 1 {
		t.Fatalf("j from last row: cursor = %d, want 1", p.Cursor())
	}
}

func TestLinkPickerEnterEmitsLaunchAndClose(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com", "https://b.com"})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})

	got := collectMsgs(cmd)
	if !containsLaunchURL(got, "https://b.com") {
		t.Fatalf("expected LaunchURLMsg{https://b.com}, got %v", got)
	}
	if !containsClosed(got) {
		t.Fatalf("expected LinkPickerClosedMsg, got %v", got)
	}
}

func TestLinkPickerNumericLaunchInRange(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com", "https://b.com", "https://c.com"})

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})

	got := collectMsgs(cmd)
	if !containsLaunchURL(got, "https://b.com") {
		t.Fatalf("expected LaunchURLMsg{https://b.com}, got %v", got)
	}
}

func TestLinkPickerNumericOutOfRangeInert(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com"})

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})

	if cmd != nil {
		t.Fatalf("out-of-range numeric should be inert, got cmd=%v", cmd)
	}
}

func TestLinkPickerEscCloses(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com"})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := collectMsgs(cmd)
	if !containsClosed(got) {
		t.Fatalf("expected LinkPickerClosedMsg from Esc, got %v", got)
	}
}

func TestLinkPickerTabCloses(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com"})
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := collectMsgs(cmd)
	if !containsClosed(got) {
		t.Fatalf("expected LinkPickerClosedMsg from Tab, got %v", got)
	}
}

func TestLinkPickerQSwallowed(t *testing.T) {
	p := newTestLinkPicker(t)
	p = p.Open([]string{"https://a.com"})
	p2, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Fatalf("q should be swallowed, got cmd=%v", cmd)
	}
	if !p2.IsOpen() {
		t.Fatal("q should not close picker")
	}
}

// collectMsgs runs cmd and returns the resulting messages. Handles
// tea.Batch by walking the batch tree.
func collectMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collectMsgs(c)...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

func containsLaunchURL(msgs []tea.Msg, url string) bool {
	for _, m := range msgs {
		if l, ok := m.(LaunchURLMsg); ok && l.URL == url {
			return true
		}
	}
	return false
}

func containsClosed(msgs []tea.Msg) bool {
	for _, m := range msgs {
		if _, ok := m.(LinkPickerClosedMsg); ok {
			return true
		}
	}
	return false
}
