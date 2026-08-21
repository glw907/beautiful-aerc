package ui_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
	"github.com/glw907/poplar/internal/ui/fixtures"
)

// galleryDir is where the gallery's committed renders live (design
// decision 10, amendment B): one file per fixture × profile × size
// point, named <fixture>-<w>x<h>-<profile>.txt, plus one ground-map
// sidecar per fixture × size point, named <fixture>-<w>x<h>.ground.txt
// (task 12): the ground map is profile-invariant (ProfileTrueColor is
// the only profile a background paints at all), so it carries no
// profile suffix and is not duplicated per profile.
const galleryDir = "testdata/gallery"

type gallerySize struct{ width, height int }

func (sz gallerySize) String() string { return fmt.Sprintf("%dx%d", sz.width, sz.height) }

type galleryProfile struct {
	name  string
	theme theme.Theme
}

// galleryProfiles is the four profiles amendment B names: truecolor
// dark and light, plus the two degrade profiles, each pinned dark
// (ANSI-16 and NO_COLOR have no light/dark distinction of their own
// to sweep, since decision 11's degrade grounds carry no color
// either way).
var galleryProfiles = []galleryProfile{
	{"truecolor-dark", theme.New(true, theme.ProfileTrueColor)},
	{"truecolor-light", theme.New(false, theme.ProfileTrueColor)},
	{"ansi16", theme.New(true, theme.ProfileANSI16)},
	{"nocolor", theme.New(true, theme.ProfileNoColor)},
}

// galleryCase pairs one fixture with the size points it renders
// distinctly at, and (rarely) a narrowed profile list: a nil
// narrowProfiles means every galleryProfiles entry, this type's own
// profiles method's default. stackTop renders the fixture's own
// View() directly, bypassing ui.Render entirely: the same branch
// App.View takes for a StateModal screen pushed onto its stack,
// Confirm's own case, since a modal owns the whole terminal itself
// rather than landing in a named LayoutMode pane. fullRegion instead
// runs the fixture through ui.Render with RenderInput.FullRegion set:
// the help overlay's own case, a non-modal stacked screen composed
// with the surrounding chrome but no sidebar or split of its own
// (task 9).
type galleryCase struct {
	fixture        fixtures.Fixture
	sizes          []gallerySize
	narrowProfiles []galleryProfile
	stackTop       bool
	fullRegion     bool
}

// profiles returns c's own narrowed profile list, or every
// galleryProfiles entry when c did not narrow it.
func (c galleryCase) profiles() []galleryProfile {
	if c.narrowProfiles != nil {
		return c.narrowProfiles
	}
	return galleryProfiles
}

// galleryCases is the whole sweep: the four placeholders sweep a
// spartan and a standard rung (the acceptance criterion's own 80×24
// and 100×30); Mail also sweeps the wide rung (150×26), the only
// size that exercises PaneSplit's compositing path (degrade divider
// vs. blank gutter), so that path lands in the gallery itself rather
// than a second, separate golden mechanism. Floor and Short each pin
// the one size that names their own layout state. MailLoaded is a
// second mail state, realistic pinned counts rather than Mail's
// empty ones, so the gallery still commits at least one rendered
// "36,102"-style grouped count (decision 12); a profile variant on
// an already-swept fixture doesn't need the full four-profile matrix
// again, so it sweeps only truecolor dark and light.
// syncStateProfiles narrows the SY-5 sync-state fixtures to truecolor
// dark and NO_COLOR (task 6's acceptance criterion: "golden per state
// (truecolor + NO_COLOR profiles)"), since the content pane beneath
// them never varies and ANSI-16 adds no distinct sync-segment
// rendering path of its own.
var syncStateProfiles = []galleryProfile{galleryProfiles[0], galleryProfiles[3]}

// chromeStateProfiles narrows task 8's toast and banner fixtures to
// truecolor dark and NO_COLOR, the same pairing syncStateProfiles
// uses: the content pane beneath them never varies, and ANSI-16 adds
// no distinct rendering path. ModalConfirm and Help do not use this
// narrowing (review round 1, finding 3): unlike toast and banner,
// the box and border glyphs they draw change per profile.
var chromeStateProfiles = syncStateProfiles

// mailBoundarySizes are the three remaining named boundary pairs
// (task 3's LayoutMode table; 99/100 already lands via the Help case
// below), each at a comfortable height in the class the pair sits
// inside: 59/60 is the width-floor/spartan boundary, where Mail drops
// all chrome below it; 139/140 is the standard/wide boundary, where
// PaneSplit and the reader card appear (150x26 exercises the same
// path at a size past it); 14/15 is the height-floor/short boundary;
// 19/20 is the short/full boundary, where a banner row becomes
// possible. Mail changes distinctly at each, the same reasoning that
// gave it 150x26.
var mailBoundarySizes = []gallerySize{
	{59, 24},
	{60, 24},
	{139, 26},
	{140, 26},
	{100, 14},
	{100, 15},
	{100, 19},
	{100, 20},
}

var galleryCases = []galleryCase{
	{fixture: fixtures.Mail, sizes: append([]gallerySize{{80, 24}, {100, 30}, {150, 26}}, mailBoundarySizes...)},
	{fixture: fixtures.MailLoaded, sizes: []gallerySize{{100, 30}}, narrowProfiles: galleryProfiles[:2]},
	// The second size point, 100×16, is HeightShort: F6's own gallery
	// proof that the sync segment drops its known total there
	// (wireframe F3).
	{fixture: fixtures.MailSyncing, sizes: []gallerySize{{100, 30}, {100, 16}}, narrowProfiles: syncStateProfiles},
	{fixture: fixtures.MailOffline, sizes: []gallerySize{{100, 30}}, narrowProfiles: syncStateProfiles},
	{fixture: fixtures.MailBackingOff, sizes: []gallerySize{{100, 30}}, narrowProfiles: syncStateProfiles},
	{fixture: fixtures.MailToast, sizes: []gallerySize{{100, 30}}, narrowProfiles: chromeStateProfiles},
	{fixture: fixtures.MailBanner, sizes: []gallerySize{{100, 30}}, narrowProfiles: chromeStateProfiles},
	// ModalConfirm and Help both sweep every profile (review round 1,
	// finding 3): the chromeStateProfiles narrowing loses the degrade
	// lane for a screen where a box and border glyphs, not a content
	// pane beneath it, are what a profile actually changes.
	{fixture: fixtures.ModalConfirm, sizes: []gallerySize{{80, 24}, {100, 30}}, stackTop: true},
	// The help overlay sweeps F5 (80x24, one column), F6 (100x30, two
	// columns), and the 99/100 boundary pair at a shared height, the
	// one layout boundary the overlay itself consumes.
	{fixture: fixtures.Help, sizes: []gallerySize{{80, 24}, {99, 30}, {100, 30}}, fullRegion: true},
	{fixture: fixtures.Calendar, sizes: []gallerySize{{80, 24}, {100, 30}, {150, 26}}},
	{fixture: fixtures.Contacts, sizes: []gallerySize{{80, 24}, {100, 30}, {150, 26}}},
	{fixture: fixtures.Config, sizes: []gallerySize{{80, 24}, {100, 30}, {150, 26}}},
	{fixture: fixtures.Floor, sizes: []gallerySize{{40, 10}}},
	{fixture: fixtures.Short, sizes: []gallerySize{{100, 16}}},
}

// update is the gallery's own regeneration flag: internal/ui's test
// binary owns it directly (no other file in this package persists a
// golden through a shared import), so `go test ./internal/ui/
// -run '^TestGallery$' -update` is the whole regeneration contract.
var update = flag.Bool("update", false, "update committed gallery renders")

// TestGallery sweeps every fixture × profile × size point through
// the render seam (design decision 10, amendment B), then fails on
// any committed file under galleryDir the sweep did not just produce
// (an orphan left behind by a since-removed or renamed case). Each
// fixture × size point also commits a ground-map sidecar,
// name+".ground.txt" (task 12): one character per cell naming the
// theme.Ground the rendered frame's SGR background resolves to
// (galleryGroundMap), so a pane regression stays visible even where
// the render itself is blank. Run
// `go test ./internal/ui/ -run '^TestGallery$' -update` to accept a
// deliberate change; without it, a stray diff or an orphan fails the
// case.
func TestGallery(t *testing.T) {
	expected := make(map[string]bool)
	for _, c := range galleryCases {
		for _, sz := range c.sizes {
			groundName := c.fixture.Name + "-" + sz.String() + ".ground"
			expected[groundName+".txt"] = true
			t.Run(groundName, func(t *testing.T) {
				got := galleryGroundMap(t, c, sz)
				checkGallery(t, groundName, got, *update)
			})

			for _, p := range c.profiles() {
				name := c.fixture.Name + "-" + sz.String() + "-" + p.name
				expected[name+".txt"] = true
				t.Run(name, func(t *testing.T) {
					got := galleryRender(c, sz, p.theme)
					checkGallery(t, name, got, *update)
				})
			}
		}
	}
	checkNoOrphans(t, expected)
}

// TestGalleryGroundMapMatchesLayout cross-checks galleryGroundMap's
// render-derived map against layoutGroundMap's ComputeLayout claim
// for every ordinarily composited case: a component that paints the
// wrong ground moves the render (and so galleryGroundMap) while
// ComputeLayout's declared map stays put, so a mismatch here fails in
// process, with no dependency on a committed file (review round 1,
// finding 1). A stackTop or fullRegion case is skipped: the screen
// itself owns that whole region and is free to paint grounds
// ComputeLayout's backdrop declaration never claimed (Confirm's box,
// Help's panel background), which is exactly why Render bypasses
// per-pane compositing for them in the first place.
func TestGalleryGroundMapMatchesLayout(t *testing.T) {
	for _, c := range galleryCases {
		if c.stackTop || c.fullRegion {
			continue
		}
		for _, sz := range c.sizes {
			name := c.fixture.Name + "-" + sz.String()
			t.Run(name, func(t *testing.T) {
				rendered := galleryGroundMap(t, c, sz)
				declared := layoutGroundMap(c, sz)
				if rendered != declared {
					t.Errorf("ground map derived from the render disagrees with ComputeLayout's declaration:\nrendered:\n%s\ndeclared:\n%s", rendered, declared)
				}
			})
		}
	}
}

// TestGalleryCoversRegisteredScreens proves every currently
// registered screen type (ui.Registered, registry.go) appears among
// the gallery's fixtures: a screen that registers without also
// gaining a galleryCases entry fails here, rather than the matrix
// silently missing it (task 12's registry-driven promise). It proves
// only that much: a registered screen with at least one fixture, not
// that every rung or profile it renders distinctly at is swept (that
// completeness is a human read of galleryCases against the design
// language's rung table, not a mechanical check).
func TestGalleryCoversRegisteredScreens(t *testing.T) {
	th := galleryProfiles[0].theme
	swept := make(map[reflect.Type]bool)
	for _, c := range galleryCases {
		swept[reflect.TypeOf(c.fixture.Build(th))] = true
	}
	for _, e := range ui.Registered() {
		if !swept[e.Type] {
			t.Errorf("%s is registered but no gallery case builds it", e.Type)
		}
	}
}

// galleryRender renders c's own fixture at sz through the seam,
// themed th: ui.Render's ordinary chrome compositing, c.fullRegion set
// for a non-modal stacked screen's own compositing, or, when
// c.stackTop is set, the fixture's own View() directly (the same
// branch App.View takes for a StateModal stacked screen).
func galleryRender(c galleryCase, sz gallerySize, th theme.Theme) string {
	lm := ui.ComputeLayout(sz.width, sz.height, c.fixture.Banner.Active)
	screen := c.fixture.Build(th)
	updated, _ := screen.Update(ui.LayoutMsg{Layout: lm})
	scr := updated.(ui.Screen) //nolint:errcheck // a Screen's own Update always returns a Screen; the assertion's panic is the message
	if c.stackTop {
		return scr.View().Content
	}
	in := ui.RenderInput{Screen: scr, FullRegion: c.fullRegion, Layout: lm, Theme: th, Status: c.fixture.Status, Banner: c.fixture.Banner}
	return ui.Render(in).Content
}

// checkGallery compares got against testdata/gallery/<name>.txt,
// escaped the same way x/exp/golden escapes its own files (control
// codes quoted, one line at a time), so a committed render stays
// plain, reviewable text. updateFile writes got as the new committed
// file instead of comparing.
func checkGallery(t *testing.T, name, got string, updateFile bool) {
	t.Helper()
	path := filepath.Join(galleryDir, name+".txt")
	escaped := escapeGalleryOutput(got)

	if updateFile {
		if err := os.MkdirAll(galleryDir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(escaped), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read committed gallery render %s: %v (run `go test ./internal/ui/ -run '^TestGallery$' -update` to create it)", path, err)
	}
	if escaped != string(want) {
		t.Errorf("gallery render %s drifted from the committed file; run `go test ./internal/ui/ -run '^TestGallery$' -update` to accept", path)
	}
}

// checkNoOrphans fails on any file under galleryDir whose basename is
// not in expected: a stale committed render the current sweep no
// longer produces, left over from a removed or renamed fixture, size,
// or profile.
func checkNoOrphans(t *testing.T, expected map[string]bool) {
	t.Helper()
	entries, err := os.ReadDir(galleryDir)
	if err != nil {
		t.Fatalf("read %s: %v", galleryDir, err)
	}
	for _, e := range entries {
		if e.IsDir() || expected[e.Name()] {
			continue
		}
		t.Errorf("orphan gallery file %s: no case in the current sweep produces it, remove it", filepath.Join(galleryDir, e.Name()))
	}
}

// escapeGalleryOutput quotes each line's control codes and escape
// sequences, the newline itself excepted, so a committed gallery
// file stays plain, reviewable text rather than raw ANSI bytes.
func escapeGalleryOutput(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		q := strconv.Quote(line)
		lines[i] = strings.TrimSuffix(strings.TrimPrefix(q, `"`), `"`)
	}
	return strings.Join(lines, "\n")
}
