package uicore

import (
	"charm.land/bubbles/v2/spinner"
	"github.com/glw907/poplar/internal/theme"
)

// NewSpinner returns a bubbles/spinner.Model styled to the shared poplar
// look (Dot variant, FgDim foreground) so every load and send-progress
// placeholder matches.
func NewSpinner(t *theme.CompiledTheme) spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle(t)
	return sp
}
