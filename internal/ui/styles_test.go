package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

func TestNewStyles(t *testing.T) {
	s := NewStyles(theme.Nord)

	tests := []struct {
		name  string
		style lipgloss.Style
	}{
		{"TabActiveBorder", s.TabActiveBorder},
		{"TabActiveText", s.TabActiveText},
		{"TabInactiveText", s.TabInactiveText},
		{"TabConnectLine", s.TabConnectLine},
		{"FrameBorder", s.FrameBorder},
		{"PanelDivider", s.PanelDivider},
		{"StatusBar", s.StatusBar},
		{"StatusConnected", s.StatusConnected},
		{"StatusReconnect", s.StatusReconnect},
		{"StatusOffline", s.StatusOffline},
		{"FooterKey", s.FooterKey},
		{"FooterHint", s.FooterHint},
		{"Selection", s.Selection},
		{"SidebarFolder", s.SidebarFolder},
		{"SidebarUnread", s.SidebarUnread},
		{"SidebarIndicator", s.SidebarIndicator},
		{"Dim", s.Dim},
		{"HelpTitle", s.HelpTitle},
		{"HelpGroupHeader", s.HelpGroupHeader},
		{"HelpKey", s.HelpKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.style.Render("test")
			if out == "" {
				t.Errorf("style %s rendered empty string", tt.name)
			}
		})
	}
}

func TestSearchStyles(t *testing.T) {
	styles := NewStyles(theme.Nord)

	checks := map[string]lipgloss.Style{
		"SearchIcon":         styles.SearchIcon,
		"SearchHint":         styles.SearchHint,
		"SearchPrompt":       styles.SearchPrompt,
		"SearchModeBadge":    styles.SearchModeBadge,
		"SearchResultCount":  styles.SearchResultCount,
		"SearchNoResults":    styles.SearchNoResults,
		"MsgListPlaceholder": styles.MsgListPlaceholder,
	}
	for name, s := range checks {
		if s.GetForeground() == nil {
			t.Errorf("%s has no foreground color", name)
		}
	}
}

func TestNewSpinner(t *testing.T) {
	th := theme.Themes[theme.DefaultThemeName]
	sp := NewSpinner(th)
	if got := len(sp.Spinner.Frames); got != len(spinner.Dot.Frames) {
		t.Errorf("frames: got %d, want %d (spinner.Dot)", got, len(spinner.Dot.Frames))
	}
	if sp.Style.GetForeground() == nil {
		t.Error("NewSpinner returned a model with no foreground color")
	}
	if !strings.Contains(sp.Style.Render("x"), "x") {
		t.Errorf("Style.Render dropped its content")
	}
}
