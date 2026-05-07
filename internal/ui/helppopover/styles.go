package helppopover

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// NewStyles builds a Styles from a compiled theme.
func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		HelpTitle: lipgloss.NewStyle().
			Foreground(t.AccentPrimary).Bold(true),
		HelpGroupHeader: lipgloss.NewStyle().
			Foreground(t.FgBright).Bold(true),
		HelpKey: lipgloss.NewStyle().
			Foreground(t.FgBright).Bold(true),
		HelpBoxBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), false, true, true, true).
			BorderForeground(t.BgBorder).
			Padding(1, 2),
		FrameBorder: lipgloss.NewStyle().
			Foreground(t.BgBorder).Background(t.BgBase),
		Dim: lipgloss.NewStyle().
			Foreground(t.FgDim),
	}
}
