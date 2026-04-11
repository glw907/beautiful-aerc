package ui

import (
	"regexp"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/beautiful-aerc/internal/mail"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

// stripANSI removes ANSI escape sequences to get plain text for positional checks.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func TestApp(t *testing.T) {
	backend := mail.NewMockBackend()

	t.Run("quit on q", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app.width = 80
		app.height = 24
		_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		if cmd == nil {
			t.Fatal("expected quit command")
		}
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("expected QuitMsg, got %T", msg)
		}
	})

	t.Run("quit on ctrl+c", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app.width = 80
		app.height = 24
		_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if cmd == nil {
			t.Fatal("expected quit command")
		}
	})

	t.Run("window size stored", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		if app.width != 120 || app.height != 40 {
			t.Errorf("size = %dx%d, want 120x40", app.width, app.height)
		}
	})

	t.Run("tab key delegates to account tab", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app.width = 80
		app.height = 24
		app, _ = app.Update(tea.KeyMsg{Type: tea.KeyTab})
		if app.acct.focused != MsgListPanel {
			t.Errorf("after Tab, focused = %d, want MsgListPanel", app.acct.focused)
		}
	})
}
