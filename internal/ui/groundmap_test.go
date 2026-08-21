package ui_test

import (
	"fmt"
	"image"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui"
)

// groundChars names each theme.Ground's character in a gallery
// ground-map sidecar (task 12): a degrade profile paints no
// background at all (render.go's coverage-invariant comment), so the
// ANSI-stripped text of a ground-structured design is blank
// whitespace, and a text-only lane alone is blind to a pane
// regression there.
var groundChars = map[theme.Ground]byte{
	theme.GroundBase:     'B',
	theme.GroundPanel:    'P',
	theme.GroundSelected: 'S',
	theme.GroundCode:     'C',
}

// groundTheme is the Theme every ground map renders against:
// ProfileTrueColor is the only profile that paints a background at
// all, and Ground identity does not depend on isDark (the dark and
// light palettes name the same four grounds at different hex values),
// so dark is an arbitrary but fixed choice.
var groundTheme = theme.New(true, theme.ProfileTrueColor)

// groundHexIndex inverts groundTheme's palette (theme.GroundHex) so a
// rendered cell's background hex reports the Ground that painted it.
var groundHexIndex = buildGroundHexIndex()

func buildGroundHexIndex() map[string]byte {
	index := make(map[string]byte, len(groundChars))
	for g, ch := range groundChars {
		index[strings.ToUpper(theme.GroundHex(g, true))] = ch
	}
	return index
}

// galleryGroundMap renders c's fixture at sz through the render seam
// (galleryRender), at groundTheme, then reads the ground map straight
// out of the rendered frame's SGR backgrounds (groundMapFromRender):
// the render itself, not a LayoutMode claim about what it should have
// painted, is this map's source. A component that paints the wrong
// ground moves this map; deriving it from ComputeLayout instead would
// leave it agreeing with the very code that has the bug (review round
// 1, finding 1).
func galleryGroundMap(t *testing.T, c galleryCase, sz gallerySize) string {
	t.Helper()
	rendered := galleryRender(c, sz, groundTheme)
	return groundMapFromRender(t, rendered, sz.width, sz.height)
}

// sgrRe matches one SGR (Select Graphic Rendition) escape sequence:
// CSI, an optional semicolon-separated parameter list, and the final
// 'm'. It is the only escape shape theme.Style ever renders (no
// cursor movement, no OSC), so this is a purpose-built walker for
// this package's output, not a general terminal emulator.
var sgrRe = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// groundMapFromRender reads rendered's background SGR state, cell by
// cell, into a width×height grid of ground characters.
func groundMapFromRender(t *testing.T, rendered string, width, height int) string {
	t.Helper()
	lines := strings.Split(rendered, "\n")
	if len(lines) != height {
		t.Fatalf("rendered frame has %d lines, want %d", len(lines), height)
	}

	grid := newGroundGrid(width, height)
	for y, line := range lines {
		writeGroundRow(t, grid, y, line)
	}
	return grid.render()
}

// writeGroundRow fills row y of grid from line's background SGR
// state. State resets at the start of every line: theme.Style never
// carries a background across a line boundary (galleryRender's
// output always closes a line with a bare reset before its newline).
func writeGroundRow(t *testing.T, grid *groundGrid, y int, line string) {
	t.Helper()
	var bg string
	col, pos := 0, 0
	for _, m := range sgrRe.FindAllStringSubmatchIndex(line, -1) {
		col = fillRun(t, grid, y, col, line[pos:m[0]], bg)
		bg = applySGR(bg, line[m[2]:m[3]])
		pos = m[1]
	}
	fillRun(t, grid, y, col, line[pos:], bg)
}

// fillRun advances col across text, one display cell per rune width,
// writing bg's Ground character (via groundHexIndex) into every cell
// it covers. An empty bg while text still has cells to paint violates
// the coverage invariant (decision 6: every cell explicitly painted
// at ProfileTrueColor), so it fails the test rather than leaving a
// gap in the map.
func fillRun(t *testing.T, grid *groundGrid, y, col int, text, bg string) int {
	t.Helper()
	for _, r := range text {
		w := ansi.StringWidth(string(r))
		ch, ok := groundHexIndex[bg]
		if !ok && w > 0 {
			t.Fatalf("row %d col %d: rune %q has no active background (raw %q)", y, col, r, bg)
		}
		for range w {
			if col < grid.width {
				grid.set(y, col, ch)
			}
			col++
		}
	}
	return col
}

// applySGR applies params (an SGR sequence's semicolon-separated
// field list, its leading CSI and trailing 'm' already stripped) to
// bg, returning the background hex ("RRGGBB", uppercase) now active.
// Only "48;2;r;g;b" (a truecolor background) and a bare reset ("0" or
// an empty params list) touch bg; every other field, including
// "38;2;r;g;b" (foreground), is skipped by its field count rather
// than read at all, and every attribute toggle (1, 22, 3, 23, 4, 24,
// ...) is ignored outright.
func applySGR(bg, params string) string {
	if params == "" {
		return ""
	}
	fields := strings.Split(params, ";")
	for i := 0; i < len(fields); i++ {
		switch fields[i] {
		case "0":
			bg = ""
		case "38":
			if i+1 < len(fields) && fields[i+1] == "2" {
				i += 4
			}
		case "48", "49":
			if fields[i] == "49" {
				bg = ""
				continue
			}
			if i+4 < len(fields) && fields[i+1] == "2" {
				bg = strings.ToUpper(fmt.Sprintf("%02x%02x%02x", atoiOr(fields[i+2]), atoiOr(fields[i+3]), atoiOr(fields[i+4])))
				i += 4
			}
		}
	}
	return bg
}

func atoiOr(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// groundGrid is galleryGroundMap's width×height scratch buffer.
type groundGrid struct {
	width, height int
	cells         [][]byte
}

func newGroundGrid(width, height int) *groundGrid {
	cells := make([][]byte, height)
	for y := range cells {
		cells[y] = make([]byte, width)
	}
	return &groundGrid{width: width, height: height, cells: cells}
}

func (g *groundGrid) set(y, x int, ch byte) { g.cells[y][x] = ch }

func (g *groundGrid) render() string {
	lines := make([]string, g.height)
	for y, row := range g.cells {
		lines[y] = string(row)
	}
	return strings.Join(lines, "\n")
}

// layoutGroundMap is the ground map ComputeLayout declares for c at
// sz: the bands and named panes it computes, each filled with its
// PaneRect.Ground, independent of any render. It exists only to
// cross-check galleryGroundMap's render-derived map
// (TestGalleryGroundMapMatchesLayout, which calls it only for a case
// that is neither stackTop nor fullRegion: either bypasses
// LayoutMode's pane composition and lets the screen paint grounds the
// backdrop declaration below never claimed, galleryRender's doc
// comment); nothing commits this map to a file. The floor state still
// bypasses pane composition too, but composePlaceholder's content
// still matches it: GroundBase across the whole terminal, the ground
// ComputeLayout's floor branch sets layout.Content().Rect to.
func layoutGroundMap(c galleryCase, sz gallerySize) string {
	lm := ui.ComputeLayout(sz.width, sz.height, c.fixture.Banner.Active)
	g := newGroundGrid(sz.width, sz.height)

	if lm.Class == ui.WidthFloor || lm.HeightClass == ui.HeightFloor {
		fillGround(g, image.Rect(0, 0, sz.width, sz.height), theme.GroundBase)
		return g.render()
	}

	fillGround(g, lm.StatusRow.Rect, lm.StatusRow.Ground)
	if lm.BannerRow {
		fillGround(g, lm.Banner.Rect, lm.Banner.Ground)
	}
	fillGround(g, lm.Main.Rect, lm.Main.Ground)
	fillGround(g, lm.Footer.Rect, lm.Footer.Ground)
	for _, id := range []ui.PaneID{ui.PaneSidebar, ui.PaneContent, ui.PaneSplit} {
		if pr, ok := lm.Panes[id]; ok {
			fillGround(g, pr.Rect, pr.Ground)
		}
	}
	return g.render()
}

func fillGround(g *groundGrid, rect image.Rectangle, ground theme.Ground) {
	ch := groundChars[ground]
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			g.set(y, x, ch)
		}
	}
}
