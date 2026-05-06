// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
	"testing"

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
