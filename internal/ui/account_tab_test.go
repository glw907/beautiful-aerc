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

	t.Run("J/K navigates sidebar", func(t *testing.T) {
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

	t.Run("j/k navigates the message list", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		if tab.msglist.Selected() != 1 {
			t.Errorf("after j, msglist.Selected() = %d, want 1", tab.msglist.Selected())
		}
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		if tab.msglist.Selected() != 0 {
			t.Errorf("after k, msglist.Selected() = %d, want 0", tab.msglist.Selected())
		}
	})

	t.Run("G jumps message list to bottom", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
		want := tab.msglist.Count() - 1
		if tab.msglist.Selected() != want {
			t.Errorf("after G, msglist.Selected() = %d, want %d",
				tab.msglist.Selected(), want)
		}
	})

	t.Run("g jumps message list to top", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
		if tab.msglist.Selected() != 0 {
			t.Errorf("after g, msglist.Selected() = %d, want 0", tab.msglist.Selected())
		}
	})

	t.Run("J refreshes the message list for the new folder", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
		// Move cursor down on the message list, then change folder.
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		tab, _ = tab.updateTab(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
		if tab.msglist.Selected() != 0 {
			t.Errorf("after folder change, msglist cursor should reset to 0, got %d",
				tab.msglist.Selected())
		}
	})

	t.Run("placeholder is gone", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
		view := stripANSI(tab.View())
		if strings.Contains(view, "Message List") {
			t.Error("expected message list placeholder to be replaced with real component")
		}
	})

	t.Run("real message subjects appear", func(t *testing.T) {
		tab := NewAccountTab(styles, backend)
		tab, _ = tab.updateTab(tea.WindowSizeMsg{Width: 120, Height: 30})
		view := stripANSI(tab.View())
		if !strings.Contains(view, "Alice Johnson") {
			t.Error("expected first mock sender to be visible in account tab view")
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
