// SPDX-License-Identifier: MIT

package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

func TestRenderErrorBannerNil(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	styles := NewStyles(th)
	if got := renderErrorBanner(ErrorMsg{}, 80, styles); got != "" {
		t.Errorf("nil err: got %q, want empty string", got)
	}
}

func TestRenderErrorBannerBasic(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	styles := NewStyles(th)
	msg := ErrorMsg{Op: "mark read", Err: errors.New("timeout")}
	got := renderErrorBanner(msg, 80, styles)
	if !strings.Contains(got, "⚠") {
		t.Errorf("missing warning glyph: %q", got)
	}
	if !strings.Contains(got, "mark read") {
		t.Errorf("missing op: %q", got)
	}
	if !strings.Contains(got, "timeout") {
		t.Errorf("missing err message: %q", got)
	}
}

func TestRenderErrorBannerWithoutOp(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	styles := NewStyles(th)
	msg := ErrorMsg{Err: errors.New("connection refused")}
	got := renderErrorBanner(msg, 80, styles)
	if !strings.Contains(got, "connection refused") {
		t.Errorf("missing err message: %q", got)
	}
	if strings.Contains(got, ": connection refused") {
		t.Errorf("unexpected colon prefix when Op is empty: %q", got)
	}
}

func TestRenderErrorBannerTruncates(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	styles := NewStyles(th)
	long := strings.Repeat("x", 200)
	msg := ErrorMsg{Op: "fetch body", Err: errors.New(long)}
	got := renderErrorBanner(msg, 40, styles)
	if w := lipgloss.Width(got); w > 40 {
		t.Errorf("width = %d, want ≤ 40", w)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("missing truncation ellipsis: %q", got)
	}
}

func TestRenderErrorBannerMultibyte(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	styles := NewStyles(th)
	msg := ErrorMsg{Op: "open", Err: errors.New("日本語日本語日本語日本語日本語")}
	got := renderErrorBanner(msg, 20, styles)
	if w := lipgloss.Width(got); w > 20 {
		t.Errorf("width = %d, want ≤ 20", w)
	}
	for _, r := range got {
		if r == '�' {
			t.Errorf("found replacement rune in output: %q", got)
		}
	}
}
