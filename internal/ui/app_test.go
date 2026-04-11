package ui

import (
	"regexp"
	"strings"
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

	t.Run("view has top line with ╮", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app, _ = app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		view := app.View()
		plain := stripANSI(view)
		lines := strings.Split(plain, "\n")
		if len(lines) < 1 {
			t.Fatal("no lines rendered")
		}
		trimmed := strings.TrimRight(lines[0], " ")
		if !strings.HasSuffix(trimmed, "╮") {
			t.Errorf("first line should end with ╮: %q", trimmed)
		}
	})

	t.Run("view has status bar with ╯", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app, _ = app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		view := app.View()
		plain := stripANSI(view)
		found := false
		for _, line := range strings.Split(plain, "\n") {
			if strings.HasSuffix(strings.TrimRight(line, " "), "╯") {
				found = true
				break
			}
		}
		if !found {
			t.Error("no line ends with ╯ (status bar missing)")
		}
	})

	t.Run("no tab bar", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app, _ = app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		view := app.View()
		plain := stripANSI(view)
		if strings.Contains(plain, "╭") {
			t.Error("should not contain ╭ (tab bar removed)")
		}
	})

	t.Run("content height is height minus 3 chrome rows", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app.width = 80
		app.height = 24
		if app.contentHeight() != 21 {
			t.Errorf("contentHeight = %d, want 21", app.contentHeight())
		}
	})

	t.Run("sidebar renders in composite layout", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app, _ = app.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
		view := app.View()
		plain := stripANSI(view)
		lines := strings.Split(plain, "\n")

		for _, name := range []string{"Inbox", "Drafts", "Sent", "Archive", "Spam", "Trash"} {
			found := false
			for _, line := range lines {
				runes := []rune(line)
				if len(runes) >= 30 {
					sidebarPart := string(runes[:30])
					if strings.Contains(sidebarPart, name) {
						found = true
						break
					}
				}
			}
			if !found {
				t.Errorf("folder %q not found in sidebar region", name)
			}
		}
	})

	t.Run("status bar updates on sidebar navigation", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app, _ = app.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
		// Navigate to Spam (index 4: Inbox->Drafts->Sent->Archive->Spam)
		// J (uppercase) navigates folders, like aerc
		for range 4 {
			app, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
		}
		view := app.View()
		plain := stripANSI(view)
		// Spam has 12 unseen
		if !strings.Contains(plain, "12 unread") {
			t.Error("status bar should show 12 unread after navigating to Spam")
		}
	})
}
