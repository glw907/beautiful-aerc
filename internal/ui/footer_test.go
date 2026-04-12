package ui

import (
	"strings"
	"testing"

	"github.com/glw907/beautiful-aerc/internal/theme"
)

func TestFooterView(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("account context has group separator", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(AccountContext)
		result := stripANSI(f.View(160))
		if !strings.Contains(result, "┊") {
			t.Error("missing group separator ┊")
		}
	})

	t.Run("account context has nav group", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(AccountContext)
		result := stripANSI(f.View(160))
		if !strings.Contains(result, "j/k messages") {
			t.Error("missing j/k messages")
		}
		if !strings.Contains(result, "J/K folders") {
			t.Error("missing J/K folders")
		}
	})

	t.Run("account context has folder jumps in nav group", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(AccountContext)
		result := stripANSI(f.View(160))
		for _, want := range []string{"I inbox", "D drafts", "S sent", "A archive"} {
			if !strings.Contains(result, want) {
				t.Errorf("missing folder jump hint %q", want)
			}
		}
	})

	t.Run("account context has triage group", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(AccountContext)
		result := stripANSI(f.View(160))
		if !strings.Contains(result, "d del") {
			t.Error("missing d del")
		}
		if !strings.Contains(result, "a archive") {
			t.Error("missing a archive")
		}
	})

	t.Run("account context has reply group", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(AccountContext)
		result := stripANSI(f.View(160))
		if !strings.Contains(result, "r reply") {
			t.Error("missing r reply")
		}
		if !strings.Contains(result, "c compose") {
			t.Error("missing c compose")
		}
	})

	t.Run("starts with 1-space padding", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(AccountContext)
		result := stripANSI(f.View(160))
		if !strings.HasPrefix(result, " ") {
			t.Error("footer should start with 1-space padding")
		}
	})
}
