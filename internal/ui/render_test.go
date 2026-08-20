package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/store"
	"github.com/glw907/poplar/internal/theme"
)

// renderTestScreen returns a loaded MailPlaceholder at lm, themed th:
// render_test.go's own fixture, kept independent of the fixtures
// package (an outside importer of this one) the way placeholder_test.go
// already builds MailPlaceholder by struct literal.
func renderTestScreen(t *testing.T, th theme.Theme, lm LayoutMode) MailPlaceholder {
	t.Helper()
	m := MailPlaceholder{theme: th, loaded: true, stats: store.MailStats{Messages: 36102, Mailboxes: 14}}
	m, _ = m.update(LayoutMsg{Layout: lm})
	return m
}

// TestRender_Purity proves QA-7's core contract: two calls with the
// same fixture, LayoutMode, and theme return a byte-identical Frame.
func TestRender_Purity(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	lm := ComputeLayout(100, 30, false)
	m := renderTestScreen(t, th, lm)

	a := Render(RenderInput{Screen: m, Layout: lm, Theme: th})
	b := Render(RenderInput{Screen: m, Layout: lm, Theme: th})
	if a != b {
		t.Error("Render is not pure: two calls with identical inputs diverged")
	}
}

// TestRender_FloorStateReturnsScreenViewVerbatim proves the floor
// state carries no chrome to compose (section 9): Render returns
// screen.View()'s own content and cursor unchanged rather than
// running it through the canvas.
func TestRender_FloorStateReturnsScreenViewVerbatim(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	lm := ComputeLayout(40, 10, false)
	m := renderTestScreen(t, th, lm)

	got := Render(RenderInput{Screen: m, Layout: lm, Theme: th})
	view := m.View()
	if got.Content != view.Content {
		t.Errorf("floor render content = %q, want screen.View().Content verbatim %q", got.Content, view.Content)
	}
	if got.Cursor != view.Cursor {
		t.Errorf("floor render cursor = %v, want screen.View().Cursor verbatim %v", got.Cursor, view.Cursor)
	}
}

// TestRender_CoversEveryRow proves the seam always emits exactly
// LayoutMode's own row count, at every profile and with a banner row
// both present and absent: the geometric half of the coverage
// invariant LayoutMode's own row-tiling test already proves
// (TestComputeLayout_RowTiling). A row's rendered width is checked
// separately, at ProfileTrueColor only
// (TestRender_TrueColorRowsFillTheirWidth): a blank, uncolored row at
// ANSI-16 or NO_COLOR is legitimately trailing-space-trimmed by the
// canvas (decision 11: a ground carries no distinguishing color
// below true color), so it renders shorter than sz.w without any
// cell going unaccounted for.
func TestRender_CoversEveryRow(t *testing.T) {
	sizes := []struct{ w, h int }{{80, 24}, {100, 30}, {150, 26}}
	profiles := []theme.Profile{theme.ProfileTrueColor, theme.ProfileANSI16, theme.ProfileNoColor}

	for _, sz := range sizes {
		for _, banner := range []bool{false, true} {
			for _, p := range profiles {
				th := theme.New(true, p)
				lm := ComputeLayout(sz.w, sz.h, banner)
				m := renderTestScreen(t, th, lm)
				got := Render(RenderInput{Screen: m, Layout: lm, Theme: th})

				if lines := strings.Split(got.Content, "\n"); len(lines) != sz.h {
					t.Errorf("%dx%d banner=%v profile=%v: %d rows, want %d", sz.w, sz.h, banner, p, len(lines), sz.h)
				}
			}
		}
	}
}

// TestRender_TrueColorRowsFillTheirWidth proves the coverage
// invariant's content half at ProfileTrueColor, with a banner row
// both present and absent: every ground (including a blank one)
// paints an explicit background that survives trailing-space
// trimming, so every row renders at exactly LayoutMode's width and
// no cell escapes uncovered.
func TestRender_TrueColorRowsFillTheirWidth(t *testing.T) {
	sizes := []struct{ w, h int }{{80, 24}, {100, 30}, {150, 26}}
	th := theme.New(true, theme.ProfileTrueColor)

	for _, sz := range sizes {
		for _, banner := range []bool{false, true} {
			lm := ComputeLayout(sz.w, sz.h, banner)
			m := renderTestScreen(t, th, lm)
			got := Render(RenderInput{Screen: m, Layout: lm, Theme: th})

			for y, line := range strings.Split(got.Content, "\n") {
				if w := ansi.StringWidth(line); w != sz.w {
					t.Errorf("%dx%d banner=%v: row %d display width %d, want %d", sz.w, sz.h, banner, y, w, sz.w)
				}
			}
		}
	}
}

// columnAt returns the rune at plain-text column x of an ANSI-styled
// row: ansi.Strip drops every escape sequence first, so an index
// into the result lines up with a terminal column for the single-
// width glyphs this test targets.
func columnAt(t *testing.T, row string, x int) rune {
	t.Helper()
	plain := []rune(ansi.Strip(row))
	if x >= len(plain) {
		return ' '
	}
	return plain[x]
}

// TestRender_DividerDegradeSubstitution proves LayoutMode's own rule
// (Divider.Degrade): the wide rung's list/reader boundary draws a
// glyph only under Theme.DrawsDividers (ANSI-16, NO_COLOR); at
// ProfileTrueColor it stays a blank gutter, Main's own fill. It
// checks the degrade divider's own column specifically, since the
// rail divider (always drawn) shares the same glyph and would give a
// false positive on a whole-row substring search.
func TestRender_DividerDegradeSubstitution(t *testing.T) {
	lm := ComputeLayout(150, 26, false)
	degradeDivider := lm.Dividers[1]
	if !degradeDivider.Degrade {
		t.Fatalf("Dividers[1].Degrade = false, want true for the wide-rung boundary this test targets")
	}
	glyphRow := degradeDivider.Y0 + 1

	trueColor := theme.New(true, theme.ProfileTrueColor)
	m := renderTestScreen(t, trueColor, lm)
	row := strings.Split(Render(RenderInput{Screen: m, Layout: lm, Theme: trueColor}).Content, "\n")[glyphRow]
	if got := columnAt(t, row, degradeDivider.X); got == []rune(trueColor.Glyphs().Divider)[0] {
		t.Errorf("true-color row %d column %d = %q, want the gutter left blank", glyphRow, degradeDivider.X, got)
	}

	ansi16 := theme.New(true, theme.ProfileANSI16)
	m = renderTestScreen(t, ansi16, lm)
	row = strings.Split(Render(RenderInput{Screen: m, Layout: lm, Theme: ansi16}).Content, "\n")[glyphRow]
	want := []rune(ansi16.Glyphs().Divider)[0]
	if got := columnAt(t, row, degradeDivider.X); got != want {
		t.Errorf("ANSI-16 row %d column %d = %q, want the divider glyph %q", glyphRow, degradeDivider.X, got, want)
	}
}
