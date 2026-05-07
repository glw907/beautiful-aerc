package compose

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// Styles holds the subset of UI styles the compose surface needs.
type Styles struct {
	ErrorBanner lipgloss.Style
}

// NewStyles builds compose Styles from a compiled theme.
func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		ErrorBanner: lipgloss.NewStyle().Foreground(t.ColorError),
	}
}
