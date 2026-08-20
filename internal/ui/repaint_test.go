package ui_test

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
	"github.com/glw907/poplar/internal/ui/fixtures"
)

// repaintFixture and repaintSize are the mail fixture and standard
// rung the two carried tests below share with the gallery sweep
// (mail-100x30-truecolor-{dark,light}), so their expectations come
// from the gallery's own committed files rather than a second,
// duplicate golden (amendment B's no-double-coverage rule).
var repaintSize = gallerySize{100, 30}

// TestRender_NeverAnsweringTerminalStaysDark is the carried
// never-answering-terminal test (task 2's review, finding 3): the
// seam renders on ui.DefaultDark with no tea.BackgroundColorMsg ever
// delivered, and that render matches the gallery's own committed
// dark file for the same fixture and size: the terminal that never
// answers gets exactly the frame the gallery already pins.
func TestRender_NeverAnsweringTerminalStaysDark(t *testing.T) {
	th := theme.New(ui.DefaultDark, theme.ProfileTrueColor)
	got := galleryRender(fixtures.Mail, repaintSize, th)
	assertMatchesGalleryFile(t, "mail-100x30-truecolor-dark", got)
}

// TestRender_BackgroundColorRepaintGoldenPair is the carried golden
// pair (task 2's review, finding 3): the pre-answer dark render and
// the post-answer light render of the same fixture each match one
// half of the gallery's own already-committed pair, proving the
// repaint a tea.BackgroundColorMsg answer drives is the same
// deterministic content the gallery already pins.
func TestRender_BackgroundColorRepaintGoldenPair(t *testing.T) {
	darkTh := theme.New(ui.DefaultDark, theme.ProfileTrueColor)
	darkGot := galleryRender(fixtures.Mail, repaintSize, darkTh)
	assertMatchesGalleryFile(t, "mail-100x30-truecolor-dark", darkGot)

	answer := tea.BackgroundColorMsg{Color: color.White}
	lightTh := theme.New(answer.IsDark(), theme.ProfileTrueColor)
	lightGot := galleryRender(fixtures.Mail, repaintSize, lightTh)
	assertMatchesGalleryFile(t, "mail-100x30-truecolor-light", lightGot)
}

// TestGallery_TwoSweepsByteIdentical is QA-7's own assertion: two
// full in-process sweeps of the gallery matrix return byte-identical
// output, case for case, independent of any committed file.
func TestGallery_TwoSweepsByteIdentical(t *testing.T) {
	first := sweepGallery()
	second := sweepGallery()
	if len(first) != len(second) {
		t.Fatalf("sweep sizes differ: %d vs %d", len(first), len(second))
	}
	for name, a := range first {
		b, ok := second[name]
		if !ok {
			t.Errorf("%s: present in the first sweep, absent from the second", name)
			continue
		}
		if a != b {
			t.Errorf("%s: the two sweeps diverged", name)
		}
	}
}

// sweepGallery renders every galleryCases × galleryProfiles point
// once, keyed by the same name checkGallery persists under.
func sweepGallery() map[string]string {
	out := make(map[string]string)
	for _, c := range galleryCases {
		for _, sz := range c.sizes {
			for _, p := range galleryProfiles {
				name := c.fixture.Name + "-" + sz.String() + "-" + p.name
				out[name] = galleryRender(c.fixture, sz, p.theme)
			}
		}
	}
	return out
}

// assertMatchesGalleryFile asserts got, escaped the same way the
// gallery escapes its own renders, equals the committed gallery file
// name.txt.
func assertMatchesGalleryFile(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(galleryDir, name+".txt")
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read committed gallery render %s: %v", path, err)
	}
	if escapeGalleryOutput(got) != string(want) {
		t.Errorf("render does not match committed gallery file %s", path)
	}
}
