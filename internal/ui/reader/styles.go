package reader

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// Styles holds the subset of UI styles the reader surface needs.
type Styles struct {
	ViewerBg     lipgloss.Style
	ViewerHeader lipgloss.Style
	Dim          lipgloss.Style
	Cursor       lipgloss.Style
}

// NewStyles builds reader Styles from a compiled theme.
func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		ViewerBg: lipgloss.NewStyle().
			Background(t.BgBase),
		ViewerHeader: lipgloss.NewStyle().
			Background(t.BgSubtle).
			Padding(1, 0, 1, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(t.FgDim).
			BorderBackground(t.BgBase),
		Dim:    lipgloss.NewStyle().Foreground(t.FgDim),
		Cursor: lipgloss.NewStyle().Foreground(t.AccentPrimary),
	}
}
