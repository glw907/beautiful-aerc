package ui

import (
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/theme"
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

	t.Run("responsive: nav drops first when space is tight", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(AccountContext)
		result := stripANSI(f.View(130))
		if strings.Contains(result, "j/k/J/K") {
			t.Error("nav hint j/k/J/K should be dropped at narrow width")
		}
		if strings.Contains(result, "I/D/S/A") {
			t.Error("nav hint I/D/S/A should be dropped at narrow width")
		}
		if !strings.Contains(result, "d del") {
			t.Error("triage should still be present at width 130")
		}
		if !strings.Contains(result, "? help") {
			t.Error("? help should still be present at width 130")
		}
	})

	t.Run("responsive: tools drop before triage and reply", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(AccountContext)
		result := stripANSI(f.View(90))
		if strings.Contains(result, "v select") {
			t.Error("v select should be dropped at width 90")
		}
		if strings.Contains(result, "n/N results") {
			t.Error("n/N results should be dropped at width 90")
		}
		if !strings.Contains(result, "d del") {
			t.Error("d del should still be present at width 90")
		}
		if !strings.Contains(result, "r/R reply") {
			t.Error("r/R reply should still be present at width 90")
		}
	})

	t.Run("responsive: app group never drops", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(AccountContext)
		result := stripANSI(f.View(40))
		for _, want := range []string{"? help", "q quit"} {
			if !strings.Contains(result, want) {
				t.Errorf("rank-0 hint %q should always be present", want)
			}
		}
	})

	t.Run("responsive: triage drops last before app", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(AccountContext)
		// At width 58 the minimum email loop survives: primary triage
		// (d/a), compose, and the always-kept app group. Reply (r/R)
		// has dropped but compose has not.
		result := stripANSI(f.View(58))
		if !strings.Contains(result, "d del") {
			t.Error("d del should still be present at width 58")
		}
		if !strings.Contains(result, "c compose") {
			t.Error("c compose should still be present at width 58")
		}
		if strings.Contains(result, "r/R reply") {
			t.Error("r/R reply should be dropped at width 58")
		}
		if !strings.Contains(result, "? help") {
			t.Error("? help should still be present at width 58")
		}
	})

	t.Run("viewer context drops reply before triage", func(t *testing.T) {
		f := NewFooter(styles)
		f.SetContext(ViewerContext)
		result := stripANSI(f.View(60))
		if !strings.Contains(result, "d del") {
			t.Error("viewer triage should survive at width 60")
		}
		if !strings.Contains(result, "Tab links") {
			t.Error("viewer affordances should survive at width 60")
		}
	})
}
