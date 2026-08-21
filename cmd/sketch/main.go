// Command sketch is a thin interactive wrapper over internal/ui's
// render seam (task 12): keys cycle a fixture, a capability profile,
// and a terminal rung, and each frame is exactly what ui.Render
// returns for that combination. It always renders through Render's
// ordinary composed path; the modal-confirm and help-overlay special
// cases gallery_test.go bypasses for (a stack-top screen rendered
// directly, a full-region screen skipping the pane split) are the
// gated gallery's concern, not this tool's. It sets no
// tea.Program color-profile option, so the terminal's detected
// capability governs what a truecolor fixture actually shows; the
// gallery's committed renders are what prove a profile's exact bytes.
// sketch verifies no pointer coordinate and no glyph's display width;
// it is a developer's eye, never a gate, and is excluded from
// `make install` and every release artifact.
package main

import (
	"fmt"
	"os"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
	"github.com/glw907/poplar/internal/ui/fixtures"
)

// sketchFixtures is every named screen state internal/ui/fixtures
// exports, in the same order the gallery sweeps them.
var sketchFixtures = []fixtures.Fixture{
	fixtures.Mail,
	fixtures.MailLoaded,
	fixtures.MailSyncing,
	fixtures.MailOffline,
	fixtures.MailBackingOff,
	fixtures.MailToast,
	fixtures.MailBanner,
	fixtures.ModalConfirm,
	fixtures.Help,
	fixtures.Calendar,
	fixtures.Contacts,
	fixtures.Config,
	fixtures.Floor,
	fixtures.Short,
}

// sketchProfile pairs a profile's label with the Theme it resolves
// to. ANSI-16 and NO_COLOR pin dark, matching the gallery's narrowing
// rationale (gallery_test.go): neither degrade profile distinguishes
// light from dark.
type sketchProfile struct {
	name  string
	theme theme.Theme
}

var sketchProfiles = []sketchProfile{
	{"truecolor-dark", theme.New(true, theme.ProfileTrueColor)},
	{"truecolor-light", theme.New(false, theme.ProfileTrueColor)},
	{"ansi16", theme.New(true, theme.ProfileANSI16)},
	{"nocolor", theme.New(true, theme.ProfileNoColor)},
}

// sketchRung is one terminal size sketch can show: the design
// language's rung boundary pairs (59/60, 79/80, 99/100, 139/140
// columns; 14/15, 19/20 rows), plus the gallery's standard, wide,
// floor, and short-height reference points.
type sketchRung struct{ width, height int }

var sketchRungs = []sketchRung{
	{59, 24},
	{60, 24},
	{79, 24},
	{80, 24},
	{99, 30},
	{100, 30},
	{139, 26},
	{140, 26},
	{100, 14},
	{100, 15},
	{100, 19},
	{100, 20},
	{150, 26},
	{40, 10},
	{100, 16},
}

func (r sketchRung) String() string { return fmt.Sprintf("%dx%d", r.width, r.height) }

// sketchKeys is sketch's keymap (elm-conventions rule 7: key.Binding
// declarations dispatched through key.Matches, never a raw switch on
// a key's string).
var sketchKeys = struct {
	NextFixture, PrevFixture key.Binding
	NextProfile, PrevProfile key.Binding
	NextRung, PrevRung       key.Binding
	Help, Quit               key.Binding
}{
	NextFixture: key.NewBinding(key.WithKeys("n")),
	PrevFixture: key.NewBinding(key.WithKeys("N")),
	NextProfile: key.NewBinding(key.WithKeys("p")),
	PrevProfile: key.NewBinding(key.WithKeys("P")),
	NextRung:    key.NewBinding(key.WithKeys("s")),
	PrevRung:    key.NewBinding(key.WithKeys("S")),
	Help:        key.NewBinding(key.WithKeys("?")),
	Quit:        key.NewBinding(key.WithKeys("q")),
}

func main() {
	if _, err := tea.NewProgram(newSketchModel()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// sketchModel cycles the three axes sketchKeys advertises and toggles
// its help text. frame is the composed string computeFrame last
// returned; View only ever renders frame, never calls Update or
// Render itself (elm-conventions rule 2). termWidth and termHeight
// are the real terminal's last-reported size, tracked only to warn
// when the chosen rung exceeds it (a rung wider or taller than the
// real terminal renders garbled with no other indication).
type sketchModel struct {
	fixture, profile, rung int
	help                   bool
	frame                  string
	termWidth, termHeight  int
}

func newSketchModel() sketchModel {
	var m sketchModel
	m.frame, _ = m.computeFrame()
	return m
}

// Init implements tea.Model.
func (m sketchModel) Init() tea.Cmd { return nil }

// Update implements tea.Model: a fixture, profile, or rung change
// recomputes frame immediately, batched with whatever Cmd the
// fixture's screen returned from its LayoutMsg Update (elm-
// conventions rule 2: no screen's Cmd is ever silently discarded,
// even though every pass-2 placeholder's LayoutMsg handling returns
// nil today).
func (m sketchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.termWidth, m.termHeight = sz.Width, sz.Height
		return m, nil
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(k, sketchKeys.Quit):
		return m, tea.Quit
	case key.Matches(k, sketchKeys.NextFixture):
		m.fixture = (m.fixture + 1) % len(sketchFixtures)
	case key.Matches(k, sketchKeys.PrevFixture):
		m.fixture = (m.fixture - 1 + len(sketchFixtures)) % len(sketchFixtures)
	case key.Matches(k, sketchKeys.NextProfile):
		m.profile = (m.profile + 1) % len(sketchProfiles)
	case key.Matches(k, sketchKeys.PrevProfile):
		m.profile = (m.profile - 1 + len(sketchProfiles)) % len(sketchProfiles)
	case key.Matches(k, sketchKeys.NextRung):
		m.rung = (m.rung + 1) % len(sketchRungs)
	case key.Matches(k, sketchKeys.PrevRung):
		m.rung = (m.rung - 1 + len(sketchRungs)) % len(sketchRungs)
	case key.Matches(k, sketchKeys.Help):
		m.help = !m.help
		return m, nil
	default:
		return m, nil
	}
	var cmd tea.Cmd
	m.frame, cmd = m.computeFrame()
	return m, cmd
}

// computeFrame builds m's current fixture at its current profile and
// rung, through ui.Render's ordinary composed path, and returns the
// rendered frame with sketch's status line appended, plus whatever
// Cmd the screen's LayoutMsg Update returned.
func (m sketchModel) computeFrame() (string, tea.Cmd) {
	f := sketchFixtures[m.fixture]
	p := sketchProfiles[m.profile]
	sz := sketchRungs[m.rung]

	lm := ui.ComputeLayout(sz.width, sz.height, f.Banner.Active)
	updated, cmd := f.Build(p.theme).Update(ui.LayoutMsg{Layout: lm})
	scr := updated.(ui.Screen) //nolint:errcheck // a Screen's Update always returns a Screen; the assertion's panic is the message

	frame := ui.Render(ui.RenderInput{Screen: scr, Layout: lm, Theme: p.theme, Status: f.Status, Banner: f.Banner})
	status := fmt.Sprintf("fixture %s   profile %s   rung %s   ? help", f.Name, p.name, sz)
	return frame.Content + "\n" + status, cmd
}

// sketchHelp is sketch's help text (task 12's acceptance criterion:
// it states plainly what sketch does not verify).
const sketchHelp = `n/N next/previous fixture   p/P next/previous profile   s/S next/previous rung   ? toggle this help   q quit
sketch does not verify pointer coordinates or a glyph's display width; the gallery (make gallery) is what gates on those.`

// View implements tea.Model.
func (m sketchModel) View() tea.View {
	out := m.frame
	if warn := m.narrowWarning(); warn != "" {
		out += "\n" + warn
	}
	if m.help {
		out += "\n" + sketchHelp
	}
	return tea.NewView(out)
}

// narrowWarning reports the line View appends when the chosen rung
// exceeds the real terminal's last-reported size, or "" once it fits
// (or before the first tea.WindowSizeMsg names a real size at all).
func (m sketchModel) narrowWarning() string {
	sz := sketchRungs[m.rung]
	if m.termWidth == 0 || (sz.width <= m.termWidth && sz.height <= m.termHeight) {
		return ""
	}
	return fmt.Sprintf("terminal is %dx%d, too small for the %s rung; this frame will render garbled", m.termWidth, m.termHeight, sz)
}
