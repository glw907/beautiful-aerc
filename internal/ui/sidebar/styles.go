package sidebar

import (
	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/theme"
)

// Styles is the sidebar's projection of internal/ui.Styles.
type Styles struct {
	SidebarBg        lipgloss.Style
	SidebarAccount   lipgloss.Style
	SidebarSelected  lipgloss.Style
	SidebarFolder    lipgloss.Style
	SidebarUnread    lipgloss.Style
	SidebarIndicator lipgloss.Style
	SidebarTreeRule  lipgloss.Style

	SearchIcon        lipgloss.Style
	SearchHint        lipgloss.Style
	SearchPrompt      lipgloss.Style
	SearchModeBadge   lipgloss.Style
	SearchResultCount lipgloss.Style
	SearchNoResults   lipgloss.Style
}

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
		SidebarTreeRule: lipgloss.NewStyle().
			Foreground(t.FgDim),

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
