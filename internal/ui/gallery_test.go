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
// standard rung (the acceptance criterion's own 80×24 and 100×30),
// while Floor and Short each pin the one size that names their own
// layout state.
var galleryCases = []struct {
	fixture fixtures.Fixture
	sizes   []gallerySize
}{
	{fixtures.Mail, []gallerySize{{80, 24}, {100, 30}}},
	{fixtures.Calendar, []gallerySize{{80, 24}, {100, 30}}},
	{fixtures.Contacts, []gallerySize{{80, 24}, {100, 30}}},
	{fixtures.Config, []gallerySize{{80, 24}, {100, 30}}},
	{fixtures.Floor, []gallerySize{{40, 10}}},
	{fixtures.Short, []gallerySize{{100, 16}}},
}

// galleryUpdate reads the "-update" flag: the same flag x/exp/golden
// registers for the seam's own static goldens (repaint_test.go,
// static_golden_test.go), so `go test ./internal/ui/... -update`
// regenerates both mechanisms in one pass. It is read via
// flag.Lookup, never a second flag.Bool("update", ...) registration,
// since a package-level golden import already owns that flag name
// and a duplicate registration panics.
func galleryUpdate() bool {
	f := flag.Lookup("update")
	return f != nil && f.Value.String() == "true"
}

// TestGallery sweeps every fixture × profile × size point through
// the render seam (design decision 10, amendment B). Run
// `go test ./internal/ui/... -update` to accept a deliberate change;
// without it, a stray diff between a fresh sweep and the committed
// file fails the case.
func TestGallery(t *testing.T) {
	update := galleryUpdate()
	for _, c := range galleryCases {
		for _, sz := range c.sizes {
			for _, p := range galleryProfiles {
				name := c.fixture.Name + "-" + sz.String() + "-" + p.name
				t.Run(name, func(t *testing.T) {
					got := galleryRender(c.fixture, sz, p.theme)
					checkGallery(t, name, got, update)
				})
			}
		}
	}
}

// galleryRender renders fixture at sz through the seam, themed th.
func galleryRender(fixture fixtures.Fixture, sz gallerySize, th theme.Theme) string {
	lm := ui.ComputeLayout(sz.width, sz.height, false)
	screen := fixture.Build(th)
	updated, _ := screen.Update(ui.LayoutMsg{Layout: lm})
	scr, ok := updated.(ui.Screen)
	if !ok {
		panic(fmt.Sprintf("gallery: %T's own Update returned a non-Screen tea.Model", screen))
	}
	return ui.Render(scr, lm, th)
}

// checkGallery compares got against testdata/gallery/<name>.txt,
// escaped the same way x/exp/golden escapes its own files (control
// codes quoted, one line at a time), so both mechanisms commit
// equally readable, diffable text. update writes got as the new
// committed file instead of comparing.
func checkGallery(t *testing.T, name, got string, update bool) {
	t.Helper()
	path := filepath.Join(galleryDir, name+".txt")
	escaped := escapeGalleryOutput(got)

	if update {
		if err := os.MkdirAll(galleryDir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(escaped), 0o644); err != nil { //nolint:gosec // a committed gallery render is not sensitive
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read committed gallery render %s: %v (run `go test ./internal/ui/... -update` to create it)", path, err)
	}
	if escaped != string(want) {
		t.Errorf("gallery render %s drifted from the committed file; run `go test ./internal/ui/... -update` to accept", path)
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
