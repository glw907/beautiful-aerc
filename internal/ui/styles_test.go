package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/theme"
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
		{"SidebarFolderUnread", s.SidebarFolderUnread},
		{"SidebarIconUnread", s.SidebarIconUnread},
		{"SidebarCount", s.SidebarCount},
		{"SidebarIndicator", s.SidebarIndicator},
		{"Dim", s.Dim},
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
