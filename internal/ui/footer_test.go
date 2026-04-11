package ui

import (
	"strings"
	"testing"

	"github.com/glw907/beautiful-aerc/internal/theme"
)

func TestFooterView(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("message list context", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(MsgListContext)
		result := f.View(120)
		if !strings.Contains(result, "del") {
			t.Error("missing delete hint")
		}
		if !strings.Contains(result, "compose") {
			t.Error("missing compose hint")
		}
	})

	t.Run("sidebar context", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(SidebarContext)
		result := f.View(120)
		if !strings.Contains(result, "open") {
			t.Error("missing open hint")
		}
		if !strings.Contains(result, "compose") {
			t.Error("missing compose hint")
		}
		if strings.Contains(result, "del") {
			t.Error("sidebar should not show delete hint")
		}
	})
}
