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

	t.Run("account context has compressed nav group", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(AccountContext)
		result := stripANSI(f.View(160))
		if !strings.Contains(result, "j/k/J/K nav") {
			t.Error("missing j/k/J/K nav")
		}
		if !strings.Contains(result, "I/D/S/A folders") {
			t.Error("missing I/D/S/A folders")
		}
	})

	t.Run("account context has planned future hints", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(AccountContext)
		result := stripANSI(f.View(200))
		for _, want := range []string{". read", "v select", "n/N results"} {
			if !strings.Contains(result, want) {
				t.Errorf("missing future hint %q", want)
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
		if !strings.Contains(result, "r/R reply") {
			t.Error("missing r/R reply")
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
