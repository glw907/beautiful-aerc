package reader

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/glw907/poplar/internal/theme"
)

type Styles struct {
	ViewerBg        lipgloss.Style
	ViewerHeader    lipgloss.Style
	Dim             lipgloss.Style
	Cursor          lipgloss.Style
	InviteIcon      lipgloss.Style
	InviteSummary   lipgloss.Style
	InviteField     lipgloss.Style
	InviteCancelled lipgloss.Style
}

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
		Dim:             lipgloss.NewStyle().Foreground(t.FgDim),
		Cursor:          lipgloss.NewStyle().Foreground(t.AccentPrimary),
		InviteIcon:      lipgloss.NewStyle().Foreground(t.AccentPrimary).Background(t.BgBase),
		InviteSummary:   lipgloss.NewStyle().Foreground(t.FgBright).Bold(true).Background(t.BgBase),
		InviteField:     lipgloss.NewStyle().Foreground(t.FgBase).Background(t.BgBase),
		InviteCancelled: lipgloss.NewStyle().Foreground(t.ColorWarning).Bold(true).Background(t.BgBase),
	}
}
