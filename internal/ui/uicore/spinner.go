package uicore

import (
	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/theme"
)

// NewSpinner returns a bubbles/spinner.Model styled to the shared poplar
// look (Dot variant, FgDim foreground) so every load and send-progress
// placeholder matches.
func NewSpinner(t *theme.CompiledTheme) spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(t.FgDim)
	return sp
}
