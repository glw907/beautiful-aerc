package ui_test

import (
	"image"
	"strings"

	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
)

// groundChars names each theme.Ground's own character in a gallery
// ground-map sidecar (task 12, the review finding behind it): a
// degrade profile's own render paints no background at all
// (render.go's own coverage-invariant comment), so the ANSI-stripped
// text of a ground-structured design is blank whitespace, and a
// text-only lane alone would be blind to a pane regression there. The
// sidecar is structural, derived from LayoutMode rather than parsed
// out of rendered bytes, so it carries the same pane shape at every
// profile.
var groundChars = map[theme.Ground]byte{
	theme.GroundBase:     'B',
	theme.GroundPanel:    'P',
	theme.GroundSelected: 'S',
	theme.GroundCode:     'C',
}

// galleryGroundMap renders c's own ground structure at sz: one
// character per cell naming the theme.Ground LayoutMode declares for
// that cell. The floor state and a stackTop screen (Confirm, this
// pass's only one) both bypass LayoutMode's own pane composition
// entirely and render their own View directly (galleryRender's own
// doc comment); floor's own placeholder still paints GroundBase
// across the whole terminal (composePlaceholder, on
// layout.Content().Rect, which floor's own ComputeLayout branch sets
// to the full terminal), and Confirm's View unconditionally paints
// GroundBase before laying its own box on top, whose border glyphs
// already carry its shape in the plain render itself.
func galleryGroundMap(c galleryCase, sz gallerySize) string {
	lm := ui.ComputeLayout(sz.width, sz.height, c.fixture.Banner.Active)
	whole := image.Rect(0, 0, sz.width, sz.height)
	g := newGroundGrid(sz.width, sz.height)

	switch {
	case lm.Class == ui.WidthFloor || lm.HeightClass == ui.HeightFloor:
		g.fill(whole, theme.GroundBase)
	case c.stackTop:
		g.fill(whole, theme.GroundBase)
	default:
		g.fill(lm.StatusRow.Rect, lm.StatusRow.Ground)
		if lm.BannerRow {
			g.fill(lm.Banner.Rect, lm.Banner.Ground)
		}
		g.fill(lm.Main.Rect, lm.Main.Ground)
		g.fill(lm.Footer.Rect, lm.Footer.Ground)
		if !c.fullRegion {
			for _, id := range []ui.PaneID{ui.PaneSidebar, ui.PaneContent, ui.PaneSplit} {
				if pr, ok := lm.Panes[id]; ok {
					g.fill(pr.Rect, pr.Ground)
				}
			}
		}
	}
	return g.render()
}

// groundGrid is galleryGroundMap's own width×height scratch buffer.
type groundGrid struct {
	height int
	cells  [][]byte
}

func newGroundGrid(width, height int) *groundGrid {
	cells := make([][]byte, height)
	for y := range cells {
		cells[y] = make([]byte, width)
	}
	return &groundGrid{height: height, cells: cells}
}

// fill sets every cell within rect to ground's own character.
func (g *groundGrid) fill(rect image.Rectangle, ground theme.Ground) {
	ch := groundChars[ground]
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			g.cells[y][x] = ch
		}
	}
}

func (g *groundGrid) render() string {
	lines := make([]string, g.height)
	for y, row := range g.cells {
		lines[y] = string(row)
	}
	return strings.Join(lines, "\n")
}
