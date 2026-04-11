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

	t.Run("focus cycling", func(t *testing.T) {
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

	t.Run("window size", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		if tab.width != 120 || tab.height != 40 {
			t.Errorf("size = %dx%d, want 120x40", tab.width, tab.height)
		}
	})
}
