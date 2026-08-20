package ui_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
	"github.com/glw907/poplar/internal/ui/fixtures"
)

// galleryDir is where the gallery's committed renders live (design
// decision 10, amendment B): one file per fixture × profile × size
// point, named <fixture>-<w>x<h>-<profile>.txt so a later sidecar
// (task 12's ground map) can sit beside a render, same basename,
// without a rename.
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

// galleryCases pairs each fixture with the size points it renders
// distinctly at: the four placeholders sweep a spartan and a
// standard rung (the acceptance criterion's own 80×24 and 100×30);
// Mail also sweeps the wide rung (150×26), the only size that
// exercises PaneSplit's compositing path (degrade divider vs. blank
// gutter), so that path lands in the gallery itself rather than a
// second, separate golden mechanism. Floor and Short each pin the
// one size that names their own layout state.
var galleryCases = []struct {
	fixture fixtures.Fixture
	sizes   []gallerySize
}{
	{fixtures.Mail, []gallerySize{{80, 24}, {100, 30}, {150, 26}}},
	{fixtures.Calendar, []gallerySize{{80, 24}, {100, 30}}},
	{fixtures.Contacts, []gallerySize{{80, 24}, {100, 30}}},
	{fixtures.Config, []gallerySize{{80, 24}, {100, 30}}},
	{fixtures.Floor, []gallerySize{{40, 10}}},
	{fixtures.Short, []gallerySize{{100, 16}}},
}

// update is the gallery's own regeneration flag: internal/ui's test
// binary owns it directly (no other file in this package persists a
// golden through a shared import), so `go test ./internal/ui/
// -run '^TestGallery$' -update` is the whole regeneration contract.
var update = flag.Bool("update", false, "update committed gallery renders")

// TestGallery sweeps every fixture × profile × size point through
// the render seam (design decision 10, amendment B), then fails on
// any committed file under galleryDir the sweep did not just produce
// (an orphan left behind by a since-removed or renamed case). Run
// `go test ./internal/ui/ -run '^TestGallery$' -update` to accept a
// deliberate change; without it, a stray diff or an orphan fails the
// case.
func TestGallery(t *testing.T) {
	expected := make(map[string]bool)
	for _, c := range galleryCases {
		for _, sz := range c.sizes {
			for _, p := range galleryProfiles {
				name := c.fixture.Name + "-" + sz.String() + "-" + p.name
				expected[name+".txt"] = true
				t.Run(name, func(t *testing.T) {
					got := galleryRender(c.fixture, sz, p.theme)
					checkGallery(t, name, got, *update)
				})
			}
		}
	}
	checkNoOrphans(t, expected)
}

// galleryRender renders fixture at sz through the seam, themed th.
func galleryRender(fixture fixtures.Fixture, sz gallerySize, th theme.Theme) string {
	lm := ui.ComputeLayout(sz.width, sz.height, false)
	screen := fixture.Build(th)
	updated, _ := screen.Update(ui.LayoutMsg{Layout: lm})
	scr := updated.(ui.Screen) //nolint:errcheck // a Screen's own Update always returns a Screen; the assertion's panic is the message
	return ui.Render(scr, lm, th).Content
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
