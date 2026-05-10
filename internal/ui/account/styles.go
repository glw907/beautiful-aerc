package account

import (
	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/theme"
)

// Styles is account's narrow projection of poplar's chrome styling
// (ADR-0161).
type Styles struct {
	// Dim renders the centered "Loading messages…" placeholder during the
	// initial folder load.
	Dim lipgloss.Style

	// PanelDivider renders the single `│` between sidebar and right pane.
	PanelDivider lipgloss.Style
}

func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		Dim:          lipgloss.NewStyle().Foreground(t.FgDim),
		PanelDivider: lipgloss.NewStyle().Foreground(t.BgBorder).Background(t.BgBase),
	}
}
