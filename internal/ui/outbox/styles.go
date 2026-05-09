package outbox

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// Styles holds the compiled lipgloss styles for the outbox view.
type Styles struct {
	Header lipgloss.Style
	Row    lipgloss.Style
	Cursor lipgloss.Style
	Empty  lipgloss.Style
}

// NewStyles builds Styles from a compiled theme.
func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		Header: lipgloss.NewStyle().Foreground(t.FgBright).Bold(true),
		Row:    lipgloss.NewStyle().Foreground(t.FgBase),
		Cursor: lipgloss.NewStyle().Foreground(t.AccentPrimary),
		Empty:  lipgloss.NewStyle().Foreground(t.FgDim).Align(lipgloss.Center),
	}
}
