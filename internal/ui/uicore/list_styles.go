package uicore

import (
	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/theme"
)

// NewListStyles projects a poplar compiled theme onto bubbles/v2/list
// styles. Three picker surfaces (movepicker, reader.LinkPicker,
// reader.AttachPicker) share this projection so list chrome stays
// consistent with the rest of the UI.
func NewListStyles(t *theme.CompiledTheme) list.Styles {
	s := list.DefaultStyles(true)

	s.Title = lipgloss.NewStyle().
		Foreground(t.FgBright).
		Bold(true).
		Padding(0, 1)

	s.TitleBar = lipgloss.NewStyle().Padding(0, 0, 1, 0)

	s.StatusBar = lipgloss.NewStyle().Foreground(t.FgDim)
	s.StatusEmpty = lipgloss.NewStyle().Foreground(t.FgDim)
	s.StatusBarActiveFilter = lipgloss.NewStyle().Foreground(t.AccentPrimary)
	s.StatusBarFilterCount = lipgloss.NewStyle().Foreground(t.FgDim)

	s.NoItems = lipgloss.NewStyle().Foreground(t.FgDim).Italic(true)

	s.PaginationStyle = lipgloss.NewStyle().Foreground(t.FgDim)
	s.HelpStyle = lipgloss.NewStyle().Foreground(t.FgDim)

	s.Filter.Focused.Prompt = lipgloss.NewStyle().Foreground(t.AccentPrimary)
	s.Filter.Focused.Text = lipgloss.NewStyle().Foreground(t.FgBright)
	s.Filter.Focused.Placeholder = lipgloss.NewStyle().Foreground(t.FgDim)
	s.Filter.Blurred.Prompt = lipgloss.NewStyle().Foreground(t.FgDim)
	s.Filter.Blurred.Text = lipgloss.NewStyle().Foreground(t.FgDim)

	s.DefaultFilterCharacterMatch = lipgloss.NewStyle().
		Foreground(t.AccentPrimary).
		Underline(true)

	return s
}
