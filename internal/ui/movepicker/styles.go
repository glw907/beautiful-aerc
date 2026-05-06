// SPDX-License-Identifier: MIT

package movepicker

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// NewStyles builds movepicker Styles from a compiled theme.
func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		Dim:    lipgloss.NewStyle().Foreground(t.FgDim),
		Cursor: lipgloss.NewStyle().Foreground(t.AccentPrimary),
	}
}
