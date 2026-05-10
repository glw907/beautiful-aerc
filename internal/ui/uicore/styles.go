package uicore

import (
	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/theme"
)

func spinnerStyle(t *theme.CompiledTheme) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.FgDim)
}
