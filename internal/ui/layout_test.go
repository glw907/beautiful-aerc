package ui

import (
	"image"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

func TestComputeLayout_WidthBoundaries(t *testing.T) {
	t.Run("59 floor, 60 spartan: chrome appears", func(t *testing.T) {
		floor := ComputeLayout(59, 24, false)
		spartan := ComputeLayout(60, 24, false)

		if floor.Class != WidthFloor {
			t.Errorf("width 59: Class = %v, want WidthFloor", floor.Class)
		}
		if spartan.Class != WidthSpartan {
			t.Errorf("width 60: Class = %v, want WidthSpartan", spartan.Class)
		}
		if floor.FooterRows != 0 || floor.StatusRow.Rect != (image.Rectangle{}) {
			t.Errorf("width 59: chrome present at the floor: %+v", floor)
		}
		if spartan.FooterRows != 1 || spartan.StatusRow.Rect == (image.Rectangle{}) {
			t.Errorf("width 60: chrome absent above the floor: %+v", spartan)
		}
	})

	t.Run("79 spartan, 80 spartan: only content width grows", func(t *testing.T) {
		w79 := ComputeLayout(79, 24, false)
		w80 := ComputeLayout(80, 24, false)

		if w79.Class != WidthSpartan || w80.Class != WidthSpartan {
			t.Fatalf("both sides of this boundary must stay WidthSpartan: got %v, %v", w79.Class, w80.Class)
		}
		if _, ok := w79.Panes[PaneSidebar]; ok {
			t.Error("width 79: sidebar present inside spartan")
		}
		if _, ok := w80.Panes[PaneSidebar]; ok {
			t.Error("width 80: sidebar present inside spartan")
		}
		if w79.FooterRows != w80.FooterRows {
			t.Error("width 79->80 changed footer rows; spartan has no internal boundary")
		}
		if got, want := w80.Content().Rect.Dx()-w79.Content().Rect.Dx(), 1; got != want {
			t.Errorf("content width grew by %d cells across 79->80, want %d", got, want)
		}
		// 80x24 is section 9's completeness size: fully functional,
		// single content pane, no sidebar.
		if w80.Content().Rect != image.Rect(0, 1, 80, 23) {
			t.Errorf("80x24 Content = %v, want (0,1)-(80,23)", w80.Content().Rect)
		}
	})

	t.Run("99 spartan, 100 standard: sidebar appears", func(t *testing.T) {
		spartan := ComputeLayout(99, 24, false)
		standard := ComputeLayout(100, 24, false)

		if spartan.Class != WidthSpartan {
			t.Errorf("width 99: Class = %v, want WidthSpartan", spartan.Class)
		}
		if standard.Class != WidthStandard {
			t.Errorf("width 100: Class = %v, want WidthStandard", standard.Class)
		}
		if _, ok := spartan.Panes[PaneSidebar]; ok {
			t.Error("width 99: sidebar present below its rung")
		}
		if _, ok := standard.Panes[PaneSidebar]; !ok {
			t.Error("width 100: sidebar absent at its rung")
		}
		if spartan.Content().Rect.Min.X != 0 {
			t.Errorf("width 99: Content.Min.X = %d, want 0", spartan.Content().Rect.Min.X)
		}
		if standard.Content().Rect.Min.X != paneX {
			t.Errorf("width 100: Content.Min.X = %d, want %d", standard.Content().Rect.Min.X, paneX)
		}
		if spartan.FooterRows != standard.FooterRows {
			t.Error("sidebar boundary changed footer rows too")
		}
	})

	t.Run("139 standard, 140 wide: split appears", func(t *testing.T) {
		standard := ComputeLayout(139, 24, false)
		wide := ComputeLayout(140, 24, false)

		if standard.Class != WidthStandard {
			t.Errorf("width 139: Class = %v, want WidthStandard", standard.Class)
		}
		if wide.Class != WidthWide {
			t.Errorf("width 140: Class = %v, want WidthWide", wide.Class)
		}
		if _, ok := standard.Panes[PaneSplit]; ok {
			t.Error("width 139: split present below its rung")
		}
		if _, ok := wide.Panes[PaneSplit]; !ok {
			t.Error("width 140: split absent at its rung")
		}
		if len(standard.Dividers) != 1 {
			t.Errorf("width 139: %d dividers, want 1 (rail only)", len(standard.Dividers))
		}
		if len(wide.Dividers) != 2 || !wide.Dividers[1].Degrade {
			t.Errorf("width 140: Dividers = %+v, want a second degrade-only divider", wide.Dividers)
		}
		if _, ok := standard.Panes[PaneSidebar]; !ok {
			t.Error("width 139->140 boundary should not touch the sidebar")
		}
		if standard.Content().Rect.Max.X != 139 {
			t.Errorf("width 139: Content.Max.X = %d, want the full remainder 139", standard.Content().Rect.Max.X)
		}
		if wide.Content().Rect.Max.X != paneX+listWidth {
			t.Errorf("width 140: Content.Max.X = %d, want listEnd %d", wide.Content().Rect.Max.X, paneX+listWidth)
		}
	})
}

func TestComputeLayout_HeightBoundaries(t *testing.T) {
	t.Run("14 floor, 15 short: the floor notice becomes the app", func(t *testing.T) {
		floor := ComputeLayout(100, 14, true)
		short := ComputeLayout(100, 15, true)

		if floor.HeightClass != HeightFloor {
			t.Errorf("height 14: HeightClass = %v, want HeightFloor", floor.HeightClass)
		}
		if short.HeightClass != HeightShort {
			t.Errorf("height 15: HeightClass = %v, want HeightShort", short.HeightClass)
		}
		// Below the height floor: the centered notice is the whole
		// frame (C1, wireframe F4), asserted against its rendered copy
		// rather than rectangles (TestComputeLayout_HeightFloor and
		// assertFloorState already pin the geometry).
		th := theme.New(true, theme.ProfileTrueColor)
		got := ansi.Strip(renderFloorNotice(th, floor.Width, floor.Height))
		if !strings.Contains(got, "poplar needs at least 60x15") {
			t.Errorf("floor notice = %q, want the minimum-size line", got)
		}
		if !strings.Contains(got, "this window is 100x14") {
			t.Errorf("floor notice = %q, want the live-size line", got)
		}
		// Above it: full chrome returns.
		if short.FooterRows != 1 || short.StatusRow.Rect == (image.Rectangle{}) {
			t.Errorf("height 15: chrome absent above the floor: %+v", short)
		}
	})

	t.Run("19 short, 20 full: banner row appears", func(t *testing.T) {
		short := ComputeLayout(100, 19, true)
		full := ComputeLayout(100, 20, true)

		if short.HeightClass != HeightShort {
			t.Errorf("height 19: HeightClass = %v, want HeightShort", short.HeightClass)
		}
		if full.HeightClass != HeightFull {
			t.Errorf("height 20: HeightClass = %v, want HeightFull", full.HeightClass)
		}
		if short.BannerRow {
			t.Error("height 19: banner row granted below HeightFull")
		}
		if !full.BannerRow {
			t.Error("height 20: banner row withheld at HeightFull")
		}
		if full.Banner.Rect == (image.Rectangle{}) {
			t.Error("height 20: BannerRow true but Banner rect empty")
		}
	})
}

func TestComputeLayout_WidthFloor(t *testing.T) {
	lm := ComputeLayout(40, 24, true)

	if lm.Class != WidthFloor {
		t.Fatalf("Class = %v, want WidthFloor", lm.Class)
	}
	assertFloorState(t, lm, image.Rect(0, 0, 40, 24))
}

func TestComputeLayout_HeightFloor(t *testing.T) {
	lm := ComputeLayout(100, 14, true)

	if lm.HeightClass != HeightFloor {
		t.Fatalf("HeightClass = %v, want HeightFloor", lm.HeightClass)
	}
	assertFloorState(t, lm, image.Rect(0, 0, 100, 14))
}

func assertFloorState(t *testing.T, lm LayoutMode, want image.Rectangle) {
	t.Helper()
	if lm.BannerRow {
		t.Error("floor state must never show a banner")
	}
	if lm.FooterRows != 0 {
		t.Error("floor state must carry no footer, only the centered notice")
	}
	if lm.StatusRow.Rect != (image.Rectangle{}) || lm.Footer.Rect != (image.Rectangle{}) {
		t.Error("floor state must carry no chrome bands")
	}
	if lm.Content().Rect != want || lm.Main.Rect != want {
		t.Errorf("floor Content/Main = %v, want the full terminal %v", lm.Content().Rect, want)
	}
	if len(lm.Panes) != 1 {
		t.Errorf("floor Panes = %v, want exactly PaneContent", lm.Panes)
	}
}

// TestComputeLayout_DegenerateHeights covers C1: a window so short it
// cannot fit even one row of chrome must still yield sane, non-negative
// geometry, not an overlapping status/footer row or a negative rectangle.
func TestComputeLayout_DegenerateHeights(t *testing.T) {
	cases := []struct{ w, h int }{
		{60, 0}, {60, 1}, {100, 0}, {100, 1},
	}
	for _, c := range cases {
		lm := ComputeLayout(c.w, c.h, true)
		if lm.HeightClass != HeightFloor {
			t.Errorf("%dx%d: HeightClass = %v, want HeightFloor", c.w, c.h, lm.HeightClass)
		}
		want := image.Rect(0, 0, c.w, c.h)
		assertFloorState(t, lm, want)
		if lm.Main.Rect.Min.Y > lm.Main.Rect.Max.Y || lm.Main.Rect.Min.X > lm.Main.Rect.Max.X {
			t.Errorf("%dx%d: negative-area Main rect %v", c.w, c.h, lm.Main.Rect)
		}
	}
}

// TestComputeLayout_RowTiling is C2's coverage guard: every row belongs
// to exactly one chrome band or Main, never zero (a gap) and never more
// than one (an overlap).
func TestComputeLayout_RowTiling(t *testing.T) {
	sizes := []struct{ w, h int }{
		{60, 24},
		{80, 24},
		{100, 24},
		{139, 24},
		{140, 24},
		{200, 40},
		{100, 15},
		{100, 19},
		{100, 20},
		{100, 30},
	}
	for _, sz := range sizes {
		for _, banner := range []bool{false, true} {
			lm := ComputeLayout(sz.w, sz.h, banner)
			if lm.Class == WidthFloor || lm.HeightClass == HeightFloor {
				continue // the floor state is one rect, not banded chrome
			}

			bannerRows := 0
			if lm.BannerRow {
				bannerRows = 1
			}
			if got, want := lm.Main.Rect.Min.Y, 1+bannerRows; got != want {
				t.Errorf("%dx%d banner=%v: Main.Min.Y = %d, want statusRows+bannerRows = %d", sz.w, sz.h, banner, got, want)
			}
			if got, want := lm.Main.Rect.Max.Y, sz.h-lm.FooterRows; got != want {
				t.Errorf("%dx%d banner=%v: Main.Max.Y = %d, want height-FooterRows = %d", sz.w, sz.h, banner, got, want)
			}

			for y := range sz.h {
				bands := 0
				if rowIn(lm.StatusRow, y) {
					bands++
				}
				if lm.BannerRow && rowIn(lm.Banner, y) {
					bands++
				}
				if rowIn(lm.Main, y) {
					bands++
				}
				if rowIn(lm.Footer, y) {
					bands++
				}
				if bands != 1 {
					t.Errorf("%dx%d banner=%v: row %d belongs to %d bands, want exactly 1", sz.w, sz.h, banner, y, bands)
				}
			}
		}
	}
}

func rowIn(pr PaneRect, y int) bool {
	return y >= pr.Rect.Min.Y && y < pr.Rect.Max.Y
}

func TestComputeLayout_Coverage(t *testing.T) {
	sizes := []struct{ w, h int }{
		{60, 24}, {80, 24}, {100, 24}, {139, 24}, {140, 24}, {200, 40}, {100, 15}, {100, 20},
	}
	for _, sz := range sizes {
		lm := ComputeLayout(sz.w, sz.h, true)
		if lm.Class == WidthFloor || lm.HeightClass == HeightFloor {
			continue
		}
		for id, pr := range lm.Panes {
			if !pr.Rect.In(lm.Main.Rect) {
				t.Errorf("%dx%d: pane %v rect %v escapes Main %v", sz.w, sz.h, id, pr.Rect, lm.Main.Rect)
			}
		}
		if lm.Main.Rect.Min.X != 0 || lm.Main.Rect.Max.X != sz.w {
			t.Errorf("%dx%d: Main does not span the full width: %v", sz.w, sz.h, lm.Main.Rect)
		}
	}
}

// TestComputeLayout_PinnedRectangles is M1: full image.Rectangle values
// pinned against the ratified shell exemplar's literal numbers, so a
// constant change cannot silently abandon that geometry.
func TestComputeLayout_PinnedRectangles(t *testing.T) {
	t.Run("120x26 standard, citing frame_standard(width=120)", func(t *testing.T) {
		lm := ComputeLayout(120, 26, false)
		checkRect(t, "StatusRow", lm.StatusRow.Rect, image.Rect(0, 0, 120, 1))
		checkRect(t, "Sidebar", lm.Panes[PaneSidebar].Rect, image.Rect(0, 1, 22, 25))
		checkRect(t, "Content", lm.Content().Rect, image.Rect(25, 1, 120, 25))
		checkRect(t, "Footer", lm.Footer.Rect, image.Rect(0, 25, 120, 26))
		if len(lm.Dividers) != 1 || lm.Dividers[0].X != 22 {
			t.Errorf("Dividers = %+v, want one at X=22", lm.Dividers)
		}
	})

	t.Run("150x26 wide, citing frame_wide(width=150)", func(t *testing.T) {
		lm := ComputeLayout(150, 26, false)
		checkRect(t, "Sidebar", lm.Panes[PaneSidebar].Rect, image.Rect(0, 1, 22, 25))
		checkRect(t, "Content", lm.Content().Rect, image.Rect(25, 1, 87, 25))
		if got, want := lm.Content().Rect.Dx(), listWidth; got != want {
			t.Errorf("Content width = %d, want listWidth %d", got, want)
		}
		// The list/reader gutter (87-89) belongs to Main alone: Content
		// stops at listEnd, not splitStart, so a selected-row ground
		// painted across Content.Rect cannot bleed into it (C3).
		checkRect(t, "Split", lm.Panes[PaneSplit].Rect, image.Rect(89, 2, 148, 24))
		if len(lm.Dividers) != 2 || lm.Dividers[1].X != 87 || !lm.Dividers[1].Degrade {
			t.Errorf("Dividers = %+v, want a second degrade divider at X=87", lm.Dividers)
		}
	})

	t.Run("80x24 spartan, citing frame_spartan(width=80)", func(t *testing.T) {
		lm := ComputeLayout(80, 24, false)
		checkRect(t, "Content", lm.Content().Rect, image.Rect(0, 1, 80, 23))
		if _, ok := lm.Panes[PaneSidebar]; ok {
			t.Error("80x24 spartan must carry no sidebar")
		}
		if len(lm.Dividers) != 0 {
			t.Errorf("80x24 spartan must carry no dividers, got %+v", lm.Dividers)
		}
	})
}

func checkRect(t *testing.T, name string, got, want image.Rectangle) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestComputeLayout_ReaderCard(t *testing.T) {
	t.Run("natural width under the cap", func(t *testing.T) {
		lm := ComputeLayout(150, 24, false)
		card := lm.Panes[PaneSplit]
		if card.Ground != theme.GroundPanel {
			t.Errorf("card Ground = %v, want GroundPanel (decision 11 elevates the reader card)", card.Ground)
		}
		if card.Rect.Dx() >= readerCardCap {
			t.Errorf("card width %d at width 150 should sit under the %d cap", card.Rect.Dx(), readerCardCap)
		}
	})

	t.Run("cap engages and centers the surplus", func(t *testing.T) {
		lm := ComputeLayout(400, 24, false)
		card := lm.Panes[PaneSplit]
		if card.Rect.Dx() != readerCardCap {
			t.Errorf("card width = %d, want the cap %d", card.Rect.Dx(), readerCardCap)
		}
		leftSlack := card.Rect.Min.X - lm.Dividers[1].X - cardGutter
		// rightExtra excludes theme.GapPane, the fixed margin the card
		// always carries on its far side mirroring cardGutter on its
		// near side; what remains should split evenly with leftSlack.
		rightExtra := 400 - card.Rect.Max.X - theme.GapPane
		if leftSlack < 0 || rightExtra < 0 {
			t.Fatalf("negative slack: left %d right %d", leftSlack, rightExtra)
		}
		if diff := leftSlack - rightExtra; diff < -2 || diff > 2 {
			t.Errorf("surplus not centered: left %d right %d", leftSlack, rightExtra)
		}
	})
}

func TestComputeLayout_Deterministic(t *testing.T) {
	sizes := []struct{ w, h int }{
		{40, 10}, {60, 14}, {80, 24}, {100, 19}, {139, 20}, {140, 24}, {300, 50},
	}
	for _, sz := range sizes {
		for _, banner := range []bool{false, true} {
			a := ComputeLayout(sz.w, sz.h, banner)
			b := ComputeLayout(sz.w, sz.h, banner)
			if !reflect.DeepEqual(a, b) {
				t.Errorf("ComputeLayout(%d, %d, %v) not deterministic:\n%+v\n%+v", sz.w, sz.h, banner, a, b)
			}
		}
	}
}

func TestDropPanes(t *testing.T) {
	specs := []paneSpec{
		{PaneContent, 10},
		{PaneSidebar, 20},
		{PaneSplit, 30},
	}

	tests := []struct {
		name  string
		width int
		want  []PaneID
	}{
		{"everything fits", 60, []PaneID{PaneContent, PaneSidebar, PaneSplit}},
		{"split drops first", 59, []PaneID{PaneContent, PaneSidebar}},
		{"sidebar drops second", 25, []PaneID{PaneContent}},
		{"content never drops, even under its own minimum", 5, []PaneID{PaneContent}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kept := dropPanes(tt.width, specs)
			var got []PaneID
			for _, s := range kept {
				got = append(got, s.id)
				if s.min != mustMin(specs, s.id) {
					t.Errorf("pane %v returned with min %d, want its declared %d", s.id, s.min, mustMin(specs, s.id))
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dropPanes(%d) = %v, want %v", tt.width, got, tt.want)
			}
		})
	}
}

func mustMin(specs []paneSpec, id PaneID) int {
	for _, s := range specs {
		if s.id == id {
			return s.min
		}
	}
	return -1
}
