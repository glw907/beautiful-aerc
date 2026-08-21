// Command sketch is a thin interactive wrapper over internal/ui's
// render seam (task 12, survey amendment C's tier-3 companion to the
// gallery): keys cycle a fixture, a capability profile, and a
// terminal rung, and each frame is exactly what ui.Render returns for
// that combination, with one status line of sketch's own drawn below
// it. sketch renders every fixture through ui.Render's ordinary
// composed path; the modal-confirm and help-overlay special cases
// gallery_test.go's own harness bypasses for (a stack-top screen
// rendered directly, a full-region screen skipping the pane split)
// are the gated gallery's concern, not this tool's. It builds no
// tea.Program color-profile option of its own, so a real terminal's
// own detected capability governs what a truecolor fixture actually
// shows; the gallery's committed renders, not this tool, are what
// proves a profile's exact bytes. sketch verifies no pointer
// coordinate and no glyph's own display width; it is a developer's
// eye, never a gate, and is excluded from `make install` and every
// release artifact.
package main

import (
	"fmt"
	"os"

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
// to. ANSI-16 and NO_COLOR pin dark, matching the gallery's own
// narrowing rationale (gallery_test.go): neither degrade profile
// distinguishes light from dark.
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
// language's own rung boundary pairs (59/60, 79/80, 99/100, 139/140
// columns; 14/15, 19/20 rows), plus the gallery's own standard, wide,
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

func main() {
	if _, err := tea.NewProgram(newSketchModel()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// sketchModel cycles the three axes sketch's own keymap advertises
// (n/N fixture, p/P profile, s/S rung) and toggles its help text; it
// holds no other state.
type sketchModel struct {
	fixture, profile, rung int
	help                   bool
}

func newSketchModel() sketchModel { return sketchModel{} }

// Init implements tea.Model.
func (m sketchModel) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m sketchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "q":
		return m, tea.Quit
	case "n":
		m.fixture = (m.fixture + 1) % len(sketchFixtures)
	case "N":
		m.fixture = (m.fixture - 1 + len(sketchFixtures)) % len(sketchFixtures)
	case "p":
		m.profile = (m.profile + 1) % len(sketchProfiles)
	case "P":
		m.profile = (m.profile - 1 + len(sketchProfiles)) % len(sketchProfiles)
	case "s":
		m.rung = (m.rung + 1) % len(sketchRungs)
	case "S":
		m.rung = (m.rung - 1 + len(sketchRungs)) % len(sketchRungs)
	case "?":
		m.help = !m.help
	}
	return m, nil
}

// sketchHelp is sketch's own help text (task 12's acceptance
// criterion: it states plainly what sketch does not verify).
const sketchHelp = `n/N next/previous fixture   p/P next/previous profile   s/S next/previous rung   ? toggle this help   q quit
sketch does not verify pointer coordinates or a glyph's own display width; the gallery (make gallery) is what gates on those.`

// View implements tea.Model.
func (m sketchModel) View() tea.View {
	f := sketchFixtures[m.fixture]
	p := sketchProfiles[m.profile]
	sz := sketchRungs[m.rung]

	lm := ui.ComputeLayout(sz.width, sz.height, f.Banner.Active)
	updated, _ := f.Build(p.theme).Update(ui.LayoutMsg{Layout: lm})
	scr := updated.(ui.Screen) //nolint:errcheck // a Screen's own Update always returns a Screen; the assertion's panic is the message

	frame := ui.Render(ui.RenderInput{Screen: scr, Layout: lm, Theme: p.theme, Status: f.Status, Banner: f.Banner})

	out := frame.Content + "\n" + fmt.Sprintf("fixture %s   profile %s   rung %s   ? help", f.Name, p.name, sz)
	if m.help {
		out += "\n" + sketchHelp
	}
	return tea.NewView(out)
}
