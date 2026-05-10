package wizard

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/theme"
)

// HuhTheme builds a huh.Theme that inherits from poplar's compiled
// theme so the wizard's huh forms match the user's chosen palette.
// The huh package consumes Themes via the Theme interface; we wrap a
// huh.ThemeFunc closing over the compiled theme.
func HuhTheme(t *theme.CompiledTheme) huh.Theme {
	return huh.ThemeFunc(func(bool) *huh.Styles {
		s := huh.ThemeBase(true)

		focusedTitle := lipgloss.NewStyle().Foreground(t.AccentPrimary).Bold(true)
		focusedDesc := lipgloss.NewStyle().Foreground(t.FgDim)
		focusedBase := lipgloss.NewStyle().PaddingLeft(1).BorderStyle(lipgloss.ThickBorder()).BorderLeft(true).BorderForeground(t.AccentPrimary)

		s.Focused.Title = focusedTitle
		s.Focused.Description = focusedDesc
		s.Focused.Base = focusedBase
		s.Focused.SelectSelector = lipgloss.NewStyle().Foreground(t.AccentPrimary).SetString("> ")
		s.Focused.Option = lipgloss.NewStyle().Foreground(t.FgBase)
		s.Focused.SelectedOption = lipgloss.NewStyle().Foreground(t.AccentPrimary).Bold(true)
		s.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(t.AccentPrimary).SetString("[•] ")
		s.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(t.FgDim).SetString("[ ] ")
		s.Focused.NoteTitle = focusedTitle
		s.Focused.ErrorMessage = lipgloss.NewStyle().Foreground(t.ColorError)
		s.Focused.ErrorIndicator = lipgloss.NewStyle().Foreground(t.ColorError).SetString(" *")

		s.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(t.AccentPrimary)
		s.Focused.TextInput.Prompt = lipgloss.NewStyle().Foreground(t.FgDim).SetString("> ")
		s.Focused.TextInput.Text = lipgloss.NewStyle().Foreground(t.FgBase)
		s.Focused.TextInput.Placeholder = lipgloss.NewStyle().Foreground(t.FgDim)

		s.Focused.FocusedButton = lipgloss.NewStyle().Foreground(t.BgBase).Background(t.AccentPrimary).Padding(0, 2).MarginRight(1)
		s.Focused.BlurredButton = lipgloss.NewStyle().Foreground(t.FgDim).Padding(0, 2).MarginRight(1)

		s.Blurred = s.Focused
		s.Blurred.Title = lipgloss.NewStyle().Foreground(t.FgDim)
		s.Blurred.Description = lipgloss.NewStyle().Foreground(t.FgDim)
		s.Blurred.Base = lipgloss.NewStyle().PaddingLeft(1).BorderStyle(lipgloss.HiddenBorder()).BorderLeft(true)

		s.Help.ShortKey = lipgloss.NewStyle().Foreground(t.FgDim)
		s.Help.ShortDesc = lipgloss.NewStyle().Foreground(t.FgDim)
		s.Help.ShortSeparator = lipgloss.NewStyle().Foreground(t.FgDim)
		s.Help.FullKey = s.Help.ShortKey
		s.Help.FullDesc = s.Help.ShortDesc
		s.Help.FullSeparator = s.Help.ShortSeparator

		return s
	})
}
