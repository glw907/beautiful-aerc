// SPDX-License-Identifier: MIT

package sidebar

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// Styles holds the subset of UI styles the sidebar package needs.
// Populated from ui.Styles at construction time.
type Styles struct {
	SidebarBg        lipgloss.Style
	SidebarAccount   lipgloss.Style
	SidebarSelected  lipgloss.Style
	SidebarFolder    lipgloss.Style
	SidebarUnread    lipgloss.Style
	SidebarIndicator lipgloss.Style

	SearchIcon        lipgloss.Style
	SearchHint        lipgloss.Style
	SearchPrompt      lipgloss.Style
	SearchModeBadge   lipgloss.Style
	SearchResultCount lipgloss.Style
	SearchNoResults   lipgloss.Style
}

// NewStyles builds a sidebar.Styles from a compiled theme.
func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		SidebarBg: lipgloss.NewStyle().
			Background(t.BgElevated),
		SidebarAccount: lipgloss.NewStyle().
			Foreground(t.AccentSecondary).Bold(true).
			Background(t.BgElevated),
		SidebarSelected: lipgloss.NewStyle().
			Background(t.BgSelection),
		SidebarFolder: lipgloss.NewStyle().
			Foreground(t.FgBase),
		SidebarUnread: lipgloss.NewStyle().
			Foreground(t.FgBright).Bold(true),
		SidebarIndicator: lipgloss.NewStyle().
			Foreground(t.AccentSecondary),

		SearchIcon: lipgloss.NewStyle().
			Foreground(t.FgDim),
		SearchHint: lipgloss.NewStyle().
			Foreground(t.FgDim),
		SearchPrompt: lipgloss.NewStyle().
			Foreground(t.FgBase),
		SearchModeBadge: lipgloss.NewStyle().
			Foreground(t.FgDim),
		SearchResultCount: lipgloss.NewStyle().
			Foreground(t.AccentTertiary),
		SearchNoResults: lipgloss.NewStyle().
			Foreground(t.ColorWarning),
	}
}
