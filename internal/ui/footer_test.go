package ui

import (
	"strings"
	"testing"

	"github.com/glw907/beautiful-aerc/internal/theme"
)

func TestFooterView(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("message list context has group separator", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(MsgListContext)
		result := stripANSI(f.View(120))
		if !strings.Contains(result, "┊") {
			t.Error("missing group separator ┊")
		}
	})

	t.Run("message list has triage group", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(MsgListContext)
		result := stripANSI(f.View(120))
		if !strings.Contains(result, "d del") {
			t.Error("missing d del")
		}
		if !strings.Contains(result, "a archive") {
			t.Error("missing a archive")
		}
	})

	t.Run("message list has reply group", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(MsgListContext)
		result := stripANSI(f.View(120))
		if !strings.Contains(result, "r reply") {
			t.Error("missing r reply")
		}
		if !strings.Contains(result, "c compose") {
			t.Error("missing c compose")
		}
	})

	t.Run("sidebar context has folder jumps", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(SidebarContext)
		result := stripANSI(f.View(120))
		if !strings.Contains(result, "I inbox") {
			t.Error("missing I inbox folder jump")
		}
		if !strings.Contains(result, "D drafts") {
			t.Error("missing D drafts folder jump")
		}
	})

	t.Run("starts with 1-space padding", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(MsgListContext)
		result := stripANSI(f.View(120))
		if !strings.HasPrefix(result, " ") {
			t.Error("footer should start with 1-space padding")
		}
	})

	t.Run("sidebar does not show delete", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(SidebarContext)
		result := stripANSI(f.View(120))
		if strings.Contains(result, "d del") {
			t.Error("sidebar should not show delete hint")
		}
	})
}
