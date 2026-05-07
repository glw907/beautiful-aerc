package account

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

// Styles is the narrow projection of poplar's chrome styling that
// account.Model needs. ADR-0161: every subpackage owns its own
// Styles constructed from a theme.CompiledTheme.
type Styles struct {
	// Dim renders the centered "Loading messages…" placeholder while
	// the initial folder load is in flight.
	Dim lipgloss.Style

	// PanelDivider renders the single `│` between the sidebar column
	// and the right pane (msglist or viewer).
	PanelDivider lipgloss.Style
}

// NewStyles compiles account's styles from a CompiledTheme.
func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		Dim:          lipgloss.NewStyle().Foreground(t.FgDim),
		PanelDivider: lipgloss.NewStyle().Foreground(t.BgBorder).Background(t.BgBase),
	}
}
