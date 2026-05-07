package uicore

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// NewSpinner returns a configured bubbles/spinner.Model with poplar's
// shared style: Dot variant, FgDim foreground. Centralized so all
// load and send-progress placeholders share one look.
func NewSpinner(t *theme.CompiledTheme) spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(t.FgDim)
	return sp
}
