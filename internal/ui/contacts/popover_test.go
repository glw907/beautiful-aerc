// SPDX-License-Identifier: MIT

package contacts

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glw907/poplar/internal/theme"
)

func newTestPopover(t *testing.T) Popover {
	t.Helper()
	s := NewStyles(theme.OneDark)
	p := NewPopover(s)
	p.SetSize(80, 24)
	return p
}

func TestPopover_MatchRender(t *testing.T) {
	p := newTestPopover(t)
	alice := Fixtures()[0] // Alice Chen, alice@example.com
	p.SetMatch("Alice Chen", "alice@example.com", alice, true)

	view := p.Box(80, 24)
	if !strings.Contains(view, "Alice Chen") {
		t.Errorf("matched render missing contact name")
	}
	if !strings.Contains(view, "alice@example.com") {
		t.Errorf("matched render missing email")
	}
}

func TestPopover_NoMatchRender(t *testing.T) {
	p := newTestPopover(t)
	p.SetMatch("Unknown Person", "unknown@example.com", Contact{}, false)

	view := p.Box(80, 24)
	if !strings.Contains(view, "No contact in address book.") {
		t.Errorf("no-match render missing absence message")
	}
	if !strings.Contains(view, "n add contact") {
		t.Errorf("no-match render missing 'n add contact' footer hint")
	}
}

func TestPopover_EscClose(t *testing.T) {
	p := newTestPopover(t)
	p.SetMatch("Alice Chen", "alice@example.com", Fixtures()[0], true)

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("Esc returned nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(ClosePopoverMsg); !ok {
		t.Errorf("Esc: got %T, want ClosePopoverMsg", msg)
	}
}

func TestPopover_IClose(t *testing.T) {
	p := newTestPopover(t)
	p.SetMatch("Alice Chen", "alice@example.com", Fixtures()[0], true)

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if cmd == nil {
		t.Fatal("i returned nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(ClosePopoverMsg); !ok {
		t.Errorf("i: got %T, want ClosePopoverMsg", msg)
	}
}

func TestPopover_NOpenForm_NoMatch(t *testing.T) {
	p := newTestPopover(t)
	displayName := "New Sender"
	email := "new@example.com"
	p.SetMatch(displayName, email, Contact{}, false)

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatal("n returned nil cmd")
	}
	msg := cmd()
	fm, ok := msg.(OpenFormMsg)
	if !ok {
		t.Fatalf("n: got %T, want OpenFormMsg", msg)
	}
	if !fm.FromPopover {
		t.Error("OpenFormMsg.FromPopover should be true")
	}
	if fm.Initial.Name != displayName {
		t.Errorf("Initial.Name = %q, want %q", fm.Initial.Name, displayName)
	}
	if len(fm.Initial.Emails) == 0 || fm.Initial.Emails[0].Address != email {
		t.Errorf("Initial.Emails[0].Address = %q, want %q",
			fm.Initial.Emails[0].Address, email)
	}
}

func TestPopover_NInertOnMatch(t *testing.T) {
	p := newTestPopover(t)
	p.SetMatch("Alice Chen", "alice@example.com", Fixtures()[0], true)

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd != nil {
		msg := cmd()
		if _, isForm := msg.(OpenFormMsg); isForm {
			t.Error("n should be inert when a match exists")
		}
	}
}
