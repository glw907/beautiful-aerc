package ui

import (
	"image"
	"reflect"
	"testing"

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
		if spartan.Content.Rect.Min.X != 0 {
			t.Errorf("width 99: Content.Min.X = %d, want 0", spartan.Content.Rect.Min.X)
		}
		if standard.Content.Rect.Min.X != paneX {
			t.Errorf("width 100: Content.Min.X = %d, want %d", standard.Content.Rect.Min.X, paneX)
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
	})
}

func TestComputeLayout_HeightBoundaries(t *testing.T) {
	t.Run("14 floor, 15 short: chrome unchanged, content grows one row", func(t *testing.T) {
		floor := ComputeLayout(100, 14, true)
		short := ComputeLayout(100, 15, true)

		if floor.HeightClass != HeightFloor {
			t.Errorf("height 14: HeightClass = %v, want HeightFloor", floor.HeightClass)
		}
		if short.HeightClass != HeightShort {
			t.Errorf("height 15: HeightClass = %v, want HeightShort", short.HeightClass)
		}
		if floor.BannerRow || short.BannerRow {
			t.Error("banner requested under 20 rows must demote to a toast, never a row")
		}
		if floor.FooterRows != 1 || short.FooterRows != 1 {
			t.Error("footer must survive to the height floor")
		}
		if got, want := short.Main.Rect.Dy()-floor.Main.Rect.Dy(), 1; got != want {
			t.Errorf("content height grew by %d rows across the boundary, want %d", got, want)
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

func TestComputeLayout_Floor(t *testing.T) {
	lm := ComputeLayout(40, 24, true)

	if lm.Class != WidthFloor {
		t.Fatalf("Class = %v, want WidthFloor", lm.Class)
	}
	if lm.BannerRow {
		t.Error("floor state must never show a banner")
	}
	if lm.FooterRows != 0 {
		t.Error("floor state must carry no footer, only the centered notice")
	}
	if lm.StatusRow.Rect != (image.Rectangle{}) || lm.Footer.Rect != (image.Rectangle{}) {
		t.Error("floor state must carry no chrome bands")
	}
	want := image.Rect(0, 0, 40, 24)
	if lm.Content.Rect != want || lm.Main.Rect != want {
		t.Errorf("floor Content/Main = %v, want the full terminal %v", lm.Content.Rect, want)
	}
	if len(lm.Panes) != 1 {
		t.Errorf("floor Panes = %v, want exactly PaneContent", lm.Panes)
	}
}

func TestComputeLayout_Coverage(t *testing.T) {
	sizes := []struct{ w, h int }{
		{60, 24}, {80, 24}, {100, 24}, {139, 24}, {140, 24}, {200, 40}, {100, 15}, {100, 20},
	}
	for _, sz := range sizes {
		lm := ComputeLayout(sz.w, sz.h, true)
		if lm.Class == WidthFloor {
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
		// rightExtra excludes cardMargin, the fixed margin the card
		// always carries on its far side mirroring cardGutter on its
		// near side; what remains should split evenly with leftSlack.
		rightExtra := 400 - card.Rect.Max.X - cardMargin
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
