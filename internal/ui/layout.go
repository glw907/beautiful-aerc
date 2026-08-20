package ui

import (
	"image"
	"slices"

	"github.com/glw907/poplar/internal/theme"
)

// WidthClass is the terminal's column-width rung (design language
// section 9): floor, spartan, standard, or wide. Each boundary adds
// or drops exactly one capability.
type WidthClass int

// The four width classes, in ascending column order.
const (
	WidthFloor WidthClass = iota
	WidthSpartan
	WidthStandard
	WidthWide
)

// HeightClass is the terminal's row-height rung (design language
// section 9): under 15 rows is the floor, 15-19 compresses chrome,
// 20 and up carries full chrome.
type HeightClass int

// The three height classes, in ascending row order.
const (
	HeightFloor HeightClass = iota
	HeightShort
	HeightFull
)

// PaneID identifies a layout pane slot. Declaration order is
// composition rule 1's drop priority when width cannot host every
// pane a rung wants: PaneSplit drops before PaneSidebar; PaneContent
// is never dropped.
type PaneID int

// The pane slots LayoutMode composes, in drop-priority order.
const (
	PaneSplit PaneID = iota
	PaneSidebar
	PaneContent
)

// PaneRect pairs a pane's screen rectangle with the ground its
// content paints against (decision 11's ground grammar). LayoutMode
// owns this pairing so a consumer never guesses a ground for a
// rectangle it did not compute.
type PaneRect struct {
	Rect   image.Rectangle
	Ground theme.Ground
}

// Divider is one structural divider column (decision 11: pane
// separation is whitespace and grounds, plus exactly one structural
// line). Degrade reports whether the divider exists only as the
// degrade profiles' substitute for a ground-step boundary
// (Theme.DrawsDividers): the rail/list divider is always drawn;
// the wide rung's list/reader boundary is a gutter in truecolor and
// a divider only under Degrade.
type Divider struct {
	X       int
	Y0, Y1  int
	Degrade bool
}

// LayoutMode is the pure formula result of one terminal size:
// width and height class, chrome allocation, and a ground-carrying
// rectangle for every pane the current rung composes. ComputeLayout
// is the only constructor; every field is a value the formulas
// derive, never mutated after return.
type LayoutMode struct {
	Width, Height int
	Class         WidthClass
	HeightClass   HeightClass

	StatusRow PaneRect
	Banner    PaneRect
	BannerRow bool

	// Main is the GroundBase backdrop spanning the full main band
	// (every column, between the status/banner rows and the
	// footer). It is painted first so a gutter or margin cell a
	// pane rectangle does not individually name still carries a
	// ground; Content, Panes, and the reader card overlay it.
	Main PaneRect

	Content PaneRect
	Panes   map[PaneID]PaneRect

	Footer     PaneRect
	FooterRows int

	Dividers []Divider
}

// Geometry constants. railWidth, paneX, listWidth, and cardGutter are
// pinned values from the ratified shell exemplar
// (docs/poplar/design/2026-08-19-shell-exemplar/shell-exemplar.py),
// not spacing roles: they size a pane grid, the way a terminal
// width itself is not a spacing role. theme.PadCard, by contrast,
// is a spacing role (a card's own interior inset) and is used
// directly from the theme package below. That is the boundary this
// file holds: a value that names a role in the design language's
// spacing table comes from theme; a value that is this file's own
// grid arithmetic is a local constant.
const (
	railWidth = 22 // the rail pane's width
	paneX     = railWidth + 3
	listWidth = 62 // the wide rung's practical list width

	cardGutter = 2 // list-to-reader gutter; a divider only under Theme.DrawsDividers
	cardMargin = 2 // base-ground margin mirroring cardGutter on the card's far side

	readerContentCap = 100 // section 9's reader content cap, in content cells

	widthSpartanMin  = 60
	widthStandardMin = 100
	widthWideMin     = 140

	heightShortMin = 15
	heightFullMin  = 20

	sidebarMin = paneX // dropping the sidebar frees exactly this much
	contentMin = 30    // composition rule 3's preview-readable floor
	splitMin   = cardGutter + 40

	readerCardCap = readerContentCap + 2*theme.PadCard
)

// ComputeLayout computes the LayoutMode for a width×height terminal.
// bannerRow reports whether the caller currently has a banner to
// show; ComputeLayout grants it a row only at HeightFull, since under
// 20 rows a banner demotes to a toast instead (section 9). The
// result depends only on its three inputs.
func ComputeLayout(width, height int, bannerRow bool) LayoutMode {
	lm := LayoutMode{
		Width:       width,
		Height:      height,
		Class:       classifyWidth(width),
		HeightClass: classifyHeight(height),
	}

	if lm.Class == WidthFloor {
		lm.Main = PaneRect{Rect: image.Rect(0, 0, width, height), Ground: theme.GroundBase}
		lm.Content = lm.Main
		lm.Panes = map[PaneID]PaneRect{PaneContent: lm.Content}
		return lm
	}

	lm.StatusRow = statusBand(width, 0)
	y := 1
	lm.BannerRow = bannerRow && lm.HeightClass == HeightFull
	if lm.BannerRow {
		lm.Banner = statusBand(width, y)
		y++
	}

	lm.FooterRows = 1
	bottom := height - 1
	lm.Footer = statusBand(width, bottom)

	lm.Main = PaneRect{Rect: image.Rect(0, y, width, bottom), Ground: theme.GroundBase}
	lm.Panes = make(map[PaneID]PaneRect)

	specs := []paneSpec{{PaneContent, contentMin}}
	if lm.Class >= WidthStandard {
		specs = append(specs, paneSpec{PaneSidebar, sidebarMin})
	}
	if lm.Class == WidthWide {
		specs[0].min = listWidth
		specs = append(specs, paneSpec{PaneSplit, splitMin})
	}
	kept := dropPanes(width, specs)

	contentX0 := 0
	if hasPane(kept, PaneSidebar) {
		lm.Panes[PaneSidebar] = PaneRect{Rect: image.Rect(0, y, paneX, bottom), Ground: theme.GroundBase}
		lm.Dividers = append(lm.Dividers, Divider{X: railWidth, Y0: y, Y1: bottom})
		contentX0 = paneX
	}

	if hasPane(kept, PaneSplit) {
		listEnd := contentX0 + listWidth
		splitStart := listEnd + cardGutter
		lm.Panes[PaneContent] = PaneRect{Rect: image.Rect(contentX0, y, splitStart, bottom), Ground: theme.GroundBase}
		lm.Panes[PaneSplit] = readerCard(splitStart, width, y, bottom)
		lm.Dividers = append(lm.Dividers, Divider{X: listEnd, Y0: y, Y1: bottom, Degrade: true})
	} else {
		lm.Panes[PaneContent] = PaneRect{Rect: image.Rect(contentX0, y, width, bottom), Ground: theme.GroundBase}
	}
	lm.Content = lm.Panes[PaneContent]

	return lm
}

func classifyWidth(width int) WidthClass {
	switch {
	case width < widthSpartanMin:
		return WidthFloor
	case width < widthStandardMin:
		return WidthSpartan
	case width < widthWideMin:
		return WidthStandard
	default:
		return WidthWide
	}
}

func classifyHeight(height int) HeightClass {
	switch {
	case height < heightShortMin:
		return HeightFloor
	case height < heightFullMin:
		return HeightShort
	default:
		return HeightFull
	}
}

func statusBand(width, y int) PaneRect {
	return PaneRect{Rect: image.Rect(0, y, width, y+1), Ground: theme.GroundPanel}
}

// readerCard sizes the wide rung's floating reader card between x0
// and right, inset one row top and bottom for its base-ground
// margin. Its width is the lesser of the space available (less
// cardMargin, mirroring the gutter on its near side) and
// readerCardCap; the design language's surplus-becomes-margin rule
// (section 9) then centers the card in whatever width it did not
// need.
func readerCard(x0, right, y, bottom int) PaneRect {
	avail := right - x0 - cardMargin
	width := min(avail, readerCardCap)
	left := x0 + (avail-width)/2
	return PaneRect{
		Rect:   image.Rect(left, y+1, left+width, bottom-1),
		Ground: theme.GroundPanel,
	}
}

// paneSpec is a pane ComputeLayout wants to fit, with the minimum
// width it needs to be worth showing.
type paneSpec struct {
	id  PaneID
	min int
}

// dropOrder is composition rule 1's drop priority: PaneSplit drops
// before PaneSidebar. PaneContent never appears here, so dropPanes
// never drops it.
var dropOrder = []PaneID{PaneSplit, PaneSidebar}

// dropPanes returns the subset of specs whose combined minimums fit
// within width, dropping panes in dropOrder until they do. A
// surviving pane's min is exactly what it declared, so no pane
// dropPanes returns is ever sized below its minimum.
func dropPanes(width int, specs []paneSpec) []paneSpec {
	kept := slices.Clone(specs)
	for _, id := range dropOrder {
		if sumMin(kept) <= width {
			break
		}
		kept = slices.DeleteFunc(kept, func(s paneSpec) bool { return s.id == id })
	}
	return kept
}

func sumMin(specs []paneSpec) int {
	total := 0
	for _, s := range specs {
		total += s.min
	}
	return total
}

func hasPane(specs []paneSpec, id PaneID) bool {
	return slices.ContainsFunc(specs, func(s paneSpec) bool { return s.id == id })
}
