package ui

import (
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/theme"
)

func TestFooterView(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("account: group separator", func(t *testing.T) {
		f := NewFooter(styles)
		f = f.SetContext(AccountContext)
		result := stripANSI(f.View(160))
		if !strings.Contains(result, "┊") {
			t.Error("missing group separator ┊")
		}
	})

	t.Run("account: compressed nav", func(t *testing.T) {
		f := NewFooter(styles)
		f = f.SetContext(AccountContext)
		// Full account footer is ~183 chars wide. Use 190 to ensure nav survives.
		result := stripANSI(f.View(190))
		if !strings.Contains(result, "j/k/J/K nav") {
			t.Error("missing j/k/J/K nav")
		}
		if !strings.Contains(result, "I/D/S/A folders") {
			t.Error("missing I/D/S/A folders")
		}
	})

	t.Run("account: future hints", func(t *testing.T) {
		f := NewFooter(styles)
		f = f.SetContext(AccountContext)
		result := stripANSI(f.View(200))
		for _, want := range []string{". read", "v select", "n/N results"} {
			if !strings.Contains(result, want) {
				t.Errorf("missing future hint %q", want)
			}
		}
	})

	t.Run("account: triage group", func(t *testing.T) {
		f := NewFooter(styles)
		f = f.SetContext(AccountContext)
		result := stripANSI(f.View(160))
		if !strings.Contains(result, "d del") {
			t.Error("missing d del")
		}
		if !strings.Contains(result, "a archive") {
			t.Error("missing a archive")
		}
	})

	t.Run("account: reply group", func(t *testing.T) {
		f := NewFooter(styles)
		f = f.SetContext(AccountContext)
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
		f = f.SetContext(AccountContext)
		result := stripANSI(f.View(160))
		if !strings.HasPrefix(result, " ") {
			t.Error("footer should start with 1-space padding")
		}
	})

	t.Run("responsive: nav drops first", func(t *testing.T) {
		f := NewFooter(styles)
		f = f.SetContext(AccountContext)
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

	t.Run("responsive: tools before triage/reply", func(t *testing.T) {
		f := NewFooter(styles)
		f = f.SetContext(AccountContext)
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

	t.Run("responsive: app sticks", func(t *testing.T) {
		f := NewFooter(styles)
		f = f.SetContext(AccountContext)
		result := stripANSI(f.View(40))
		for _, want := range []string{"? help", "q quit"} {
			if !strings.Contains(result, want) {
				t.Errorf("rank-0 hint %q should always be present", want)
			}
		}
	})

	t.Run("responsive: triage drops last before app", func(t *testing.T) {
		f := NewFooter(styles)
		f = f.SetContext(AccountContext)
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
		f = f.SetContext(ViewerContext)
		result := stripANSI(f.View(60))
		if !strings.Contains(result, "d del") {
			t.Error("viewer triage should survive at width 60")
		}
		if !strings.Contains(result, "Tab links") {
			t.Error("viewer affordances should survive at width 60")
		}
	})

	t.Run("ellipsis appears when hints drop", func(t *testing.T) {
		f := NewFooter(styles)
		f = f.SetContext(AccountContext)
		// 90 cells forces several drops (v select, n/N results, etc.).
		result := stripANSI(f.View(90))
		if !strings.Contains(result, "…") {
			t.Errorf("truncated footer missing … marker:\n%q", result)
		}
	})

	t.Run("ellipsis absent when full footer fits", func(t *testing.T) {
		f := NewFooter(styles)
		f = f.SetContext(AccountContext)
		result := stripANSI(f.View(220))
		if strings.Contains(result, "…") {
			t.Errorf("untruncated footer should not carry … marker:\n%q", result)
		}
	})
}

func TestFooterWindowCounter(t *testing.T) {
	styles := NewStyles(theme.Nord)

	t.Run("counter appears in account context at wide width", func(t *testing.T) {
		f := NewFooter(styles).SetCounter("500/2347")
		result := stripANSI(f.View(200))
		if !strings.Contains(result, "500/2347") {
			t.Error("expected counter 500/2347 in footer output")
		}
	})

	t.Run("empty counter does not appear", func(t *testing.T) {
		f := NewFooter(styles).SetCounter("")
		result := stripANSI(f.View(200))
		// The counter format is "N/M"; without a counter there should be
		// no digit/digit pattern beyond the existing static hints.
		if strings.Contains(result, "500/2347") {
			t.Error("counter 500/2347 should not appear when counter is empty")
		}
	})

	t.Run("viewer: no counter", func(t *testing.T) {
		f := NewFooter(styles).SetContext(ViewerContext).SetCounter("500/2347")
		result := stripANSI(f.View(200))
		if strings.Contains(result, "500/2347") {
			t.Error("counter should not appear in viewer context")
		}
	})

	t.Run("counter drops at narrow width like other rank-8 hints", func(t *testing.T) {
		f := NewFooter(styles).SetCounter("500/2347")
		// Width 130 drops rank-8+ hints (nav, v select, n/N results, counter).
		result := stripANSI(f.View(130))
		if strings.Contains(result, "500/2347") {
			t.Error("counter should be dropped at narrow width 130 (rank 8)")
		}
		// Core hints should still be present.
		if !strings.Contains(result, "d del") {
			t.Error("d del should still be present at width 130")
		}
		if !strings.Contains(result, "? help") {
			t.Error("? help should still be present at width 130")
		}
	})
}

func TestFooterThreadsGroup(t *testing.T) {
	styles := NewStyles(theme.Nord)
	f := NewFooter(styles)

	t.Run("renders Threads group at full width", func(t *testing.T) {
		out := stripANSI(f.View(200))
		if !strings.Contains(out, "␣ fold") {
			t.Error("expected ␣ fold hint")
		}
		if !strings.Contains(out, "F fold all") {
			t.Error("expected F fold all hint")
		}
	})
}

func TestComposeFooterAlwaysIncludesAttachHint(t *testing.T) {
	g := composeFooterGroups(false, false, false)
	if !containsHint(g, "Ctrl+O", "attach") {
		t.Errorf("compose footer missing Ctrl+O attach:\n%v", g)
	}
}

func TestComposeFooterIncludesSigHint(t *testing.T) {
	g := composeFooterGroups(true, false, false)
	if !containsHint(g, "Ctrl+G", "sig") {
		t.Errorf("compose footer missing Ctrl+G sig:\n%v", g)
	}
}

func TestComposeFooterOmitsSigWhenNoSigs(t *testing.T) {
	g := composeFooterGroups(false, false, false)
	if containsHint(g, "Ctrl+G", "sig") {
		t.Errorf("compose footer included Ctrl+G sig with no sigs:\n%v", g)
	}
}

func TestComposeFooterAddsFromHintsOnFocusFrom(t *testing.T) {
	g := composeFooterGroups(true, true, false)
	if !containsHint(g, "Space/←→", "identity") {
		t.Errorf("focusFrom footer missing identity hint:\n%v", g)
	}
}

func TestComposeFooterIncludesTidyHintWhenVisible(t *testing.T) {
	g := composeFooterGroups(false, false, true)
	if !containsHint(g, "Ctrl+T", "tidy") {
		t.Errorf("compose footer missing Ctrl+T tidy:\n%v", g)
	}
}

func TestComposeFooterOmitsTidyHintWhenInert(t *testing.T) {
	g := composeFooterGroups(false, false, false)
	if containsHint(g, "Ctrl+T", "tidy") {
		t.Errorf("compose footer included Ctrl+T tidy when inert:\n%v", g)
	}
}

func containsHint(groups [][]footerHint, key, desc string) bool {
	for _, g := range groups {
		for _, h := range g {
			if h.key == key && h.desc == desc {
				return true
			}
		}
	}
	return false
}
