package movepicker

import (
	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/theme"
)

func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		Dim:    lipgloss.NewStyle().Foreground(t.FgDim),
		Cursor: lipgloss.NewStyle().Foreground(t.AccentPrimary),
		Match:  lipgloss.NewStyle().Underline(true),
	}
}
