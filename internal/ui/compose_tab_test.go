// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

func newTestCompose(t *testing.T) *ComposeTab {
	t.Helper()
	styles := NewStyles(theme.Nord)
	c := NewComposeTab(styles, theme.Nord, "geoff@907.life", SimpleIcons)
	c.SetSize(80, 24)
	return c
}

func keyMsgFromString(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func sendKey(c *ComposeTab, k string) *ComposeTab {
	next, _ := c.Update(keyMsgFromString(k))
	return next
}

func TestComposeTab_View_HonorsAssignedWidth(t *testing.T) {
	c := newTestCompose(t)
	c.SetSize(60, 20)
	for i, line := range strings.Split(c.View(), "\n") {
		if w := lipgloss.Width(line); w != 60 {
			t.Fatalf("line %d width = %d, want 60: %q", i, w, line)
		}
	}
}

func TestComposeTab_View_HasHeaderRows(t *testing.T) {
	c := newTestCompose(t)
	v := c.View()
	for _, want := range []string{"From:", "To:", "Cc:", "Bcc:", "Subject:"} {
		if !strings.Contains(v, want) {
			t.Fatalf("View missing %q\n%s", want, v)
		}
	}
}

func TestComposeTab_TabCyclesFields(t *testing.T) {
	c := newTestCompose(t)
	want := []int{composeFocusCc, composeFocusBcc, composeFocusSubject, composeFocusBody, composeFocusTo}
	for i, w := range want {
		c = sendKey(c, "tab")
		if c.focus != w {
			t.Fatalf("step %d: focus = %d, want %d", i, c.focus, w)
		}
	}
}

func TestComposeTab_ShiftTabCyclesBackward(t *testing.T) {
	c := newTestCompose(t)
	c = sendKey(c, "shift+tab")
	if c.focus != composeFocusBody {
		t.Fatalf("Shift+Tab from To should wrap to Body, got %d", c.focus)
	}
}

func TestComposeTab_EscFromBodyReturnsToSubject(t *testing.T) {
	c := newTestCompose(t)
	c.focus = composeFocusBody
	c.editor.Focus()
	c.to.Blur()
	c = sendKey(c, "esc")
	if c.focus != composeFocusSubject {
		t.Fatalf("Esc from Body should focus Subject, got %d", c.focus)
	}
}

func TestComposeTab_EscFromHeaderReturnsToBody(t *testing.T) {
	c := newTestCompose(t)
	c.focus = composeFocusTo
	c = sendKey(c, "esc")
	if c.focus != composeFocusBody {
		t.Fatalf("Esc from header should focus Body, got %d", c.focus)
	}
}
