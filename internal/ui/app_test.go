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

	t.Run("initial state", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		if len(app.tabs) != 1 {
			t.Fatalf("expected 1 tab, got %d", len(app.tabs))
		}
		if app.activeTab != 0 {
			t.Errorf("activeTab = %d, want 0", app.activeTab)
		}
	})

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

	t.Run("tab delegates to account tab", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app.width = 80
		app.height = 24
		app, _ = app.Update(tea.KeyMsg{Type: tea.KeyTab})
		acct, ok := app.tabs[0].(AccountTab)
		if !ok {
			t.Fatal("tabs[0] is not AccountTab")
		}
		if acct.focused != MsgListPanel {
			t.Errorf("after Tab, focused = %d, want MsgListPanel", acct.focused)
		}
	})

	t.Run("view renders all sections", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app.width = 80
		app.height = 24
		view := app.View()
		if !strings.Contains(view, "Inbox") {
			t.Error("view missing Inbox in tab bar")
		}
		if !strings.Contains(view, "connected") {
			t.Error("view missing connection indicator")
		}
	})

	t.Run("vertical line connections", func(t *testing.T) {
		app := NewApp(theme.Nord, backend)
		app.width = 80
		app.height = 20
		// Propagate size so content renders
		app, _ = app.Update(tea.WindowSizeMsg{Width: 80, Height: 20})

		view := app.View()
		plain := stripANSI(view)
		lines := strings.Split(plain, "\n")

		if len(lines) < 6 {
			t.Fatalf("expected at least 6 lines, got %d:\n%s", len(lines), plain)
		}

		// Row 3 (index 2): must have ┬ for divider junction and ╮ for right frame corner
		row3 := strings.TrimRight(lines[2], " ")
		if !strings.Contains(row3, "┬") {
			t.Errorf("row 3 missing ┬ divider junction:\n%s", row3)
		}
		row3Runes := []rune(row3)
		lastRune := row3Runes[len(row3Runes)-1]
		if lastRune != '╮' {
			t.Errorf("row 3 last char = %c, want ╮:\n%s", lastRune, row3)
		}

		// Rows 1-2 should NOT have │ at the right edge (tab bubble floats free)
		for i := 0; i < 2; i++ {
			runes := []rune(lines[i])
			if len(runes) > 0 {
				last := runes[len(runes)-1]
				if last == '│' {
					t.Errorf("row %d has │ at right edge (should float free):\n%s", i+1, lines[i])
				}
			}
		}

		// Find the ┬ position on row 3 — this is where the panel divider is
		dividerCol := -1
		for i, r := range row3Runes {
			if r == '┬' {
				dividerCol = i
				break
			}
		}
		if dividerCol < 0 {
			t.Fatal("could not find ┬ position on row 3")
		}

		// Content lines (row 4 onward, before status bar): must have │ at dividerCol
		// and │ at the right edge (right border)
		for i := 3; i < len(lines)-2; i++ { // skip status bar and footer
			runes := []rune(lines[i])
			if len(runes) == 0 {
				continue
			}
			if dividerCol < len(runes) && runes[dividerCol] != '│' {
				t.Errorf("line %d: char at divider col %d = %c, want │:\n%s",
					i+1, dividerCol, runes[dividerCol], lines[i])
			}
			last := runes[len(runes)-1]
			if last != '│' {
				t.Errorf("line %d: last char = %c, want │ (right border):\n%s",
					i+1, last, lines[i])
			}
		}
	})
}
