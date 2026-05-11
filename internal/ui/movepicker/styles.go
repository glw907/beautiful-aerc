package movepicker

import (
	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/uicore"
)

// Styles is the move picker's projection of internal/ui.Styles.
type Styles struct {
	Dim    lipgloss.Style
	Cursor lipgloss.Style
	Match  lipgloss.Style
	List   list.Styles
}

func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		Dim:    lipgloss.NewStyle().Foreground(t.FgDim),
		Cursor: lipgloss.NewStyle().Foreground(t.AccentPrimary),
		Match:  lipgloss.NewStyle().Underline(true),
		List:   uicore.NewListStyles(t),
	}
}
