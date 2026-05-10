package wizard

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/theme"
)

type themeSection struct {
	parent *Model
	form   *huh.Form
	pick   string
	last   string
}

func newThemeSection(parent *Model) *themeSection {
	s := &themeSection{parent: parent, pick: "one-dark", last: "one-dark"}
	opts := make([]huh.Option[string], 0, len(theme.Themes))
	for _, name := range theme.ThemeNames() {
		opts = append(opts, huh.NewOption(name, name))
	}
	s.form = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Pick a theme").
			Description("Preview updates as you scroll.").
			Height(8).
			Options(opts...).
			Value(&s.pick),
	)).WithTheme(HuhTheme(parent.Theme))
	return s
}

func (s *themeSection) Name() string  { return "theme" }
func (s *themeSection) Init() tea.Cmd { return s.form.Init() }

func (s *themeSection) Update(msg tea.Msg) (section, tea.Cmd) {
	updated, cmd := s.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		s.form = f
	}

	cmds := []tea.Cmd{cmd}
	if s.pick != s.last {
		s.last = s.pick
		if t, ok := theme.Themes[s.pick]; ok {
			pick := s.pick
			cmds = append(cmds, func() tea.Msg {
				return ThemeChangedMsg{Theme: t, Name: pick}
			})
		}
	}

	if s.form.State == huh.StateCompleted {
		cmds = append(cmds, func() tea.Msg { return AdvanceMsg{} })
	}
	return s, tea.Batch(cmds...)
}

func (s *themeSection) View() string {
	left := s.form.View()
	right := renderThemePreview(s.parent.Theme)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func renderThemePreview(t *theme.CompiledTheme) string {
	bg := lipgloss.NewStyle().Background(t.BgBase).Foreground(t.FgBase).Padding(1, 2)
	bright := lipgloss.NewStyle().Background(t.BgBase).Foreground(t.FgBright).Bold(true)
	dim := lipgloss.NewStyle().Background(t.BgBase).Foreground(t.FgDim)
	accent := lipgloss.NewStyle().Background(t.BgBase).Foreground(t.AccentPrimary)

	var b strings.Builder
	b.WriteString(bright.Render("Inbox"))
	b.WriteString("\n")
	b.WriteString(dim.Render("──────"))
	b.WriteString("\n")
	b.WriteString(accent.Render("• Geoff Wright   ") + dim.Render("11:47"))
	b.WriteString("\n")
	b.WriteString("  " + bright.Render("Re: poplar setup"))
	b.WriteString("\n")
	b.WriteString(dim.Render("  Hannah W.        9:14"))
	b.WriteString("\n")
	b.WriteString(dim.Render("  weekend plans"))
	b.WriteString("\n\n")
	b.WriteString(dim.Render("q quit  / search  c new"))
	return bg.Render(b.String())
}
