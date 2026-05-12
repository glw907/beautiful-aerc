package ui

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/theme"
)

func TestNewStyles(t *testing.T) {
	s := NewStyles(theme.Nord)

	tests := []struct {
		name     string
		style    lipgloss.Style
		wantFG   bool
		wantBG   bool
		wantBold bool
	}{
		{name: "TabActiveBorder", style: s.TabActiveBorder, wantFG: true},
		{name: "TabActiveText", style: s.TabActiveText, wantFG: true, wantBG: true},
		{name: "TabInactiveText", style: s.TabInactiveText, wantFG: true},
		{name: "TabConnectLine", style: s.TabConnectLine, wantFG: true},
		{name: "FrameBorder", style: s.FrameBorder, wantFG: true, wantBG: true},
		{name: "PanelDivider", style: s.PanelDivider, wantFG: true, wantBG: true},
		{name: "StatusBar", style: s.StatusBar, wantFG: true, wantBG: true},
		{name: "StatusConnected", style: s.StatusConnected, wantFG: true, wantBG: true},
		{name: "StatusReconnect", style: s.StatusReconnect, wantFG: true, wantBG: true},
		{name: "StatusOffline", style: s.StatusOffline, wantFG: true, wantBG: true},
		{name: "FooterKey", style: s.FooterKey, wantFG: true, wantBold: true},
		{name: "FooterHint", style: s.FooterHint, wantFG: true},
		{name: "Selection", style: s.Selection, wantBG: true},
		{name: "SidebarFolder", style: s.SidebarFolder, wantFG: true},
		{name: "SidebarUnread", style: s.SidebarUnread, wantFG: true, wantBold: true},
		{name: "SidebarIndicator", style: s.SidebarIndicator, wantFG: true},
		{name: "Dim", style: s.Dim, wantFG: true},
		{name: "HelpTitle", style: s.HelpTitle, wantFG: true, wantBold: true},
		{name: "HelpGroupHeader", style: s.HelpGroupHeader, wantFG: true, wantBold: true},
		{name: "HelpKey", style: s.HelpKey, wantFG: true, wantBold: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantFG && tt.style.GetForeground() == nil {
				t.Errorf("%s.GetForeground() = nil, want non-nil", tt.name)
			}
			if tt.wantBG && tt.style.GetBackground() == nil {
				t.Errorf("%s.GetBackground() = nil, want non-nil", tt.name)
			}
			if tt.wantBold && !tt.style.GetBold() {
				t.Errorf("%s.GetBold() = false, want true", tt.name)
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
