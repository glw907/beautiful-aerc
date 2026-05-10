package wizard

import (
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/theme"
)

// Styles is the wizard's projection of the compiled theme. Built once
// at New and treated as read-only. Recomputed when the theme section
// emits ThemeChangedMsg.
type Styles struct {
	Wordmark         lipgloss.Style
	Rule             lipgloss.Style
	Tagline          lipgloss.Style
	StepCounter      lipgloss.Style
	Frame            lipgloss.Style
	ProbeStepOK      lipgloss.Style
	ProbeStepFail    lipgloss.Style
	ProbeStepPending lipgloss.Style
	Help             lipgloss.Style
	Body             lipgloss.Style
}

func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		Wordmark:         lipgloss.NewStyle().Bold(true).Foreground(t.AccentPrimary),
		Rule:             lipgloss.NewStyle().Foreground(t.BgBorder),
		Tagline:          lipgloss.NewStyle().Foreground(t.FgDim).Italic(true),
		StepCounter:      lipgloss.NewStyle().Foreground(t.FgDim),
		Frame:            lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.BgBorder).Padding(1, 2),
		ProbeStepOK:      lipgloss.NewStyle().Foreground(t.ColorSuccess),
		ProbeStepFail:    lipgloss.NewStyle().Foreground(t.ColorError),
		ProbeStepPending: lipgloss.NewStyle().Foreground(t.FgDim),
		Help:             lipgloss.NewStyle().Foreground(t.FgDim),
		Body:             lipgloss.NewStyle().Foreground(t.FgBase),
	}
}
