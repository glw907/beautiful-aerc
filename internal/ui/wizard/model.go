// Package wizard hosts the bubbletea + huh first-run setup surface.
// The UI-free domain (Model, Strategy, Apply, Probe) lives at
// github.com/glw907/poplar/internal/wizard. Each section here is a
// self-contained sub-model that drives one configurable slice of the
// final config (account, theme, eventually contacts/signatures/tidy).
//
// huh handles the static form pages; the probe screen is a custom
// tea.Model because its live spinner + transcript don't fit huh's
// Note field shape (ADR-0191).
package wizard

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/glw907/poplar/internal/theme"
	wizdomain "github.com/glw907/poplar/internal/wizard"
)

// Model is the wizard parent. It owns the active section, the theme
// reference (rebuilt on ThemeChangedMsg), and the working domain
// state that sections mutate as they collect input.
//
// It satisfies tea.Model directly so cmd/poplar can hand it to
// tea.NewProgram without a wrapper.
type Model struct {
	State  wizdomain.Model
	Theme  *theme.CompiledTheme
	Styles Styles
	Width  int
	Height int

	sections []section
	active   int
	finished bool
}

// section is one wizard step's UI. Sections are ordinary string-View
// children. the parent assembles them into the tea.View it returns
// from Model.View().
type section interface {
	Name() string
	Init() tea.Cmd
	Update(tea.Msg) (section, tea.Cmd)
	View() string
}

// NewModel constructs the wizard with the default section registry
// (account → theme → confirm). Sections that aren't yet wired
// (contacts, signatures, tidy) are absent. adding them later means
// appending to defaultSections.
func NewModel(t *theme.CompiledTheme) Model {
	m := Model{
		Theme:  t,
		Styles: NewStyles(t),
	}
	m.sections = defaultSections(&m)
	return m
}

// WithSections filters the registry by name. Used by `poplar config
// init --interactive --section=account,theme` to run a subset.
// Unknown names are silently dropped. caller validates beforehand.
func (m Model) WithSections(names []string) Model {
	if len(names) == 0 {
		return m
	}
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	filtered := m.sections[:0]
	for _, s := range m.sections {
		if want[s.Name()] {
			filtered = append(filtered, s)
		}
	}
	m.sections = filtered
	m.active = 0
	return m
}

func (m Model) Init() tea.Cmd {
	if len(m.sections) == 0 {
		return nil
	}
	return m.sections[m.active].Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
	case ThemeChangedMsg:
		m.Theme = msg.Theme
		m.Styles = NewStyles(msg.Theme)
		m.State.Theme = msg.Name
	case AdvanceMsg:
		if m.active+1 < len(m.sections) {
			m.active++
			return m, m.sections[m.active].Init()
		}
		m.finished = true
		return m, tea.Quit
	case FinishMsg:
		m.finished = true
		return m, tea.Quit
	case CancelMsg:
		return m, tea.Quit
	case tea.KeyPressMsg:
		if msg.Mod&tea.ModCtrl != 0 && msg.Code == 'c' {
			return m, tea.Quit
		}
	}

	if len(m.sections) == 0 {
		return m, nil
	}
	updated, cmd := m.sections[m.active].Update(msg)
	m.sections[m.active] = updated
	return m, cmd
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.WindowTitle = "poplar setup"

	if len(m.sections) == 0 {
		v.SetContent(m.Styles.Help.Render("(no sections configured)"))
		return v
	}

	header := renderLogo(m.Styles)
	body := m.sections[m.active].View()
	counter := m.Styles.StepCounter.Render(stepCounter(m.active+1, len(m.sections)))

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		body,
		"",
		counter,
	)
	v.SetContent(content)
	return v
}

// Finished reports whether the wizard exited normally (used by the
// caller to decide between re-loading config and reporting an error).
func (m Model) Finished() bool { return m.finished }

func stepCounter(cur, total int) string {
	dots := make([]rune, total)
	for i := range dots {
		if i+1 == cur {
			dots[i] = '●'
		} else {
			dots[i] = '○'
		}
	}
	return string(dots)
}
