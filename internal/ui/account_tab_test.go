package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/glw907/beautiful-aerc/internal/mail"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

func TestAccountTab(t *testing.T) {
	styles := NewStyles(theme.Nord)
	backend := mail.NewMockBackend()

	t.Run("title returns folder name", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		if tab.Title() != "Inbox" {
			t.Errorf("Title() = %q, want Inbox", tab.Title())
		}
	})

	t.Run("not closeable", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		if tab.Closeable() {
			t.Error("AccountTab should not be closeable")
		}
	})

	t.Run("tab key toggles focus", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		if tab.focused != SidebarPanel {
			t.Errorf("initial focus = %d, want SidebarPanel", tab.focused)
		}

		tab, _ = tab.Update(tea.KeyMsg{Type: tea.KeyTab})
		if tab.focused != MsgListPanel {
			t.Errorf("after Tab, focus = %d, want MsgListPanel", tab.focused)
		}

		tab, _ = tab.Update(tea.KeyMsg{Type: tea.KeyTab})
		if tab.focused != SidebarPanel {
			t.Errorf("after second Tab, focus = %d, want SidebarPanel", tab.focused)
		}
	})

	t.Run("view renders two panels with divider", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab.width = 80
		tab.height = 20
		result := tab.View()
		if !strings.Contains(result, "│") {
			t.Error("missing panel divider")
		}
	})

	t.Run("resize propagates", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		if tab.width != 120 {
			t.Errorf("width = %d, want 120", tab.width)
		}
		if tab.height != 40 {
			t.Errorf("height = %d, want 40", tab.height)
		}
	})
}
