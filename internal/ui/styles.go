// Package ui implements poplar's bubbletea terminal UI.
package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/beautiful-aerc/internal/theme"
)

// Styles holds composed lipgloss styles derived from a CompiledTheme.
// Created once at startup and passed read-only to all child components.
type Styles struct {
	// Tab bar
	TabActiveBorder lipgloss.Style
	TabActiveText   lipgloss.Style
	TabInactiveText lipgloss.Style
	TabConnectLine  lipgloss.Style

	// Content frame
	FrameBorder  lipgloss.Style
	PanelDivider lipgloss.Style

	// Status bar
	StatusBar       lipgloss.Style
	StatusConnected lipgloss.Style
	StatusReconnect lipgloss.Style
	StatusOffline   lipgloss.Style

	// Footer
	FooterKey  lipgloss.Style
	FooterHint lipgloss.Style
	FooterSep  lipgloss.Style

	// Selection (used by focus cycling)
	Selection lipgloss.Style

	// Sidebar
	SidebarAccount  lipgloss.Style
	SidebarSelected lipgloss.Style

	// Placeholder text
	Dim lipgloss.Style

	// Top line frame edge
	TopLine   lipgloss.Style
	ToastText lipgloss.Style
}

// NewStyles creates a Styles from a CompiledTheme.
func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		TabActiveBorder: lipgloss.NewStyle().
			Foreground(t.BgBorder),
		TabActiveText: lipgloss.NewStyle().
			Foreground(t.AccentSecondary).
			Background(t.BgBase),
		TabInactiveText: lipgloss.NewStyle().
			Foreground(t.FgDim),
		TabConnectLine: lipgloss.NewStyle().
			Foreground(t.BgBorder),

		FrameBorder: lipgloss.NewStyle().
			Foreground(t.BgBorder),
		PanelDivider: lipgloss.NewStyle().
			Foreground(t.BgBorder),

		StatusBar: lipgloss.NewStyle().
			Foreground(t.FgBright).
			Background(t.BgBorder),
		StatusConnected: lipgloss.NewStyle().
			Foreground(t.ColorSuccess).
			Background(t.BgBorder),
		StatusReconnect: lipgloss.NewStyle().
			Foreground(t.ColorWarning).
			Background(t.BgBorder),
		StatusOffline: lipgloss.NewStyle().
			Foreground(t.FgDim).
			Background(t.BgBorder),

		FooterKey: lipgloss.NewStyle().
			Foreground(t.FgBright).Bold(true),
		FooterHint: lipgloss.NewStyle().
			Foreground(t.FgDim),
		FooterSep: lipgloss.NewStyle().
			Foreground(t.FgDim),

		Selection: lipgloss.NewStyle().
			Background(t.BgSelection),

		SidebarAccount: lipgloss.NewStyle().
			Foreground(t.AccentSecondary).Bold(true),
		SidebarSelected: lipgloss.NewStyle().
			Background(t.BgSelection),

		Dim: lipgloss.NewStyle().
			Foreground(t.FgDim),

		TopLine: lipgloss.NewStyle().
			Foreground(t.BgBorder),
		ToastText: lipgloss.NewStyle().
			Foreground(t.ColorSuccess),
	}
}
