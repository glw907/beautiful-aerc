package compose

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// Styles is the compose surface's projection of internal/ui.Styles.
type Styles struct {
	ErrorBanner lipgloss.Style
}

func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		ErrorBanner: lipgloss.NewStyle().Foreground(t.ColorError),
	}
}
