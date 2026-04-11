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

		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyTab})
		if tab.focused != MsgListPanel {
			t.Errorf("after Tab, focus = %d, want MsgListPanel", tab.focused)
		}

		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyTab})
		if tab.focused != SidebarPanel {
			t.Errorf("after second Tab, focus = %d, want SidebarPanel", tab.focused)
		}
	})

	t.Run("title returns folder name", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		if tab.Title() != "Inbox" {
			t.Errorf("Title() = %q, want Inbox", tab.Title())
		}
	})

	t.Run("view renders two panels with divider", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 80, Height: 20})
		result := tab.View()
		if !strings.Contains(result, "│") {
			t.Error("missing panel divider")
		}
	})

	t.Run("view shows account name", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 80, Height: 20})
		view := stripANSI(tab.View())
		if !strings.Contains(view, "geoff@907.life") {
			t.Error("sidebar should show account name")
		}
	})

	t.Run("view renders folder names", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 80, Height: 20})
		view := tab.View()
		plain := stripANSI(view)
		for _, name := range []string{"Inbox", "Drafts", "Sent", "Archive", "Spam", "Trash"} {
			if !strings.Contains(plain, name) {
				t.Errorf("missing folder %q in sidebar", name)
			}
		}
	})

	t.Run("J/K navigates sidebar when focused", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab.width = 80
		tab.height = 20
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
		if tab.sidebar.SelectedFolder() != "Drafts" {
			t.Errorf("after J, selected = %q, want Drafts", tab.sidebar.SelectedFolder())
		}
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
		if tab.sidebar.SelectedFolder() != "Inbox" {
			t.Errorf("after K, selected = %q, want Inbox", tab.sidebar.SelectedFolder())
		}
	})

	t.Run("title tracks selected folder", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab.width = 80
		tab.height = 20
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
		if tab.Title() != "Drafts" {
			t.Errorf("Title() = %q, want Drafts", tab.Title())
		}
	})

	t.Run("sidebar unfocused when msglist focused", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab.width = 80
		tab.height = 20
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyTab})
		if tab.sidebar.focused {
			t.Error("sidebar should be unfocused after Tab")
		}
	})

	t.Run("G jumps to bottom folder", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab.width = 80
		tab.height = 20
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
		if tab.sidebar.SelectedFolder() != "Lists/rust" {
			t.Errorf("after G, selected = %q, want Lists/rust", tab.sidebar.SelectedFolder())
		}
	})

	t.Run("window size", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 40})
		if tab.width != 120 || tab.height != 40 {
			t.Errorf("size = %dx%d, want 120x40", tab.width, tab.height)
		}
	})
}
