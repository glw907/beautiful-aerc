package ui

import (
	"fmt"
	"image"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/theme"
)

// Frame is Render's result: the composed frame content, plus a
// cursor translated into the frame's coordinate space when screen's
// View supplied one. Cursor is nil for every pass-2 screen (none
// accepts text entry yet); pass 4's first text-entry screen is the
// first to set one, which is why the translation exists now rather
// than as a signature break later.
type Frame struct {
	Content string
	Cursor  *tea.Cursor
}

// paneRenderOrder is the order Render composes LayoutMode's named
// panes in, after Main's ground has already painted the whole
// band beneath them (LayoutMode.Main's doc comment). Sidebar and
// Split get a blank ground fill this pass, since no chrome component
// reaches them until the chrome tasks that follow, while Content
// gets the screen's already-rendered View.
var paneRenderOrder = []PaneID{PaneSidebar, PaneContent, PaneSplit}

// RenderInput is Render's parameter set (conventions ruling,
// task-8-findings-r1.md: folded into a struct now rather than at a
// sixth positional parameter). Screen, Layout, and Theme are every
// screen's three inputs; Status and Banner are the two chrome
// bands' render state, App's whole answer to "what the top band
// and the banner row show this frame." FullRegion is task 9's
// addition: a screen pushed onto App's stack (the help overlay,
// first of its kind) owns no sidebar or split, so its
// content spans the whole Main band rather than landing in the
// narrower Content pane a surface's sidebar reservation would
// otherwise leave it squeezed against. The footer always follows
// Screen.Entry() itself: a caller sets Screen to whichever screen is
// actually in front, App's stack top included, so a separate Entry
// field would only ever restate what Screen already answers.
type RenderInput struct {
	Screen     Screen
	FullRegion bool
	Layout     LayoutMode
	Theme      theme.Theme
	Status     StatusLine
	Banner     Banner
}

// Render is poplar's pure render seam (survey amendment A): it
// composes in.Screen's View (already rendered against in.Layout
// and in.Theme, since a caller applies both via LayoutMsg and
// ThemeMsg before calling Render) into the full frame in.Layout
// describes, painting every band's ground before a named pane's
// content overlays it, so every row LayoutMode allocates is accounted
// for (LayoutMode's row-tiling and pane-containment guarantees).
// Below ProfileTrueColor a ground carries no distinguishing color
// (decision 11), so a blank, uncolored row's trailing cells render as
// trimmed whitespace rather than styled spaces. The coverage
// invariant's content half, every cell explicitly painted, holds at
// ProfileTrueColor, where every ground, including a blank one, sets
// an explicit background. Render runs no tea.Program and does no I/O;
// it returns the same Frame for the same RenderInput every time
// (QA-7's purity contract). The three-argument shape recorded in the
// pass 2 plan's task 5b findings grew a fourth at task 6: Status,
// App's chrome state, is what the seam paints into StatusRow
// rather than a blank fill. Task 7 stops treating Footer as a blank
// fill too: it paints in.Screen.Entry()'s registry-derived hints
// instead. Task 8 grows a fifth, Banner, painted into LayoutMode's
// Banner band whenever in.Layout.BannerRow is set, folding all five
// into this struct ahead of a sixth parameter, which task 9 adds as
// FullRegion. Below the floor, in.Screen never renders at all: the
// centered notice renderFloorNotice composes is the whole frame
// (design language section 9, wireframe F4), checked ahead of
// in.Screen.View() so a screen that would panic or garble below the
// floor never runs there.
func Render(in RenderInput) Frame {
	lm, th := in.Layout, in.Theme

	if lm.Class == WidthFloor || lm.HeightClass == HeightFloor {
		return Frame{Content: renderFloorNotice(th, lm.Width, lm.Height)}
	}

	view := in.Screen.View()
	canvas := theme.NewCanvas(lm.Width, lm.Height)
	dropTotal := lm.HeightClass != HeightFull
	canvas.Paint(lm.StatusRow.Rect, renderStatusLine(in.Status, th, lm.StatusRow.Rect.Dx(), dropTotal))
	if lm.BannerRow {
		canvas.Paint(lm.Banner.Rect, renderBanner(in.Banner, th, lm.Banner.Rect.Dx()))
	}
	canvas.Paint(lm.Main.Rect, th.Blank(lm.Main.Ground, lm.Main.Rect.Dx(), lm.Main.Rect.Dy()))
	canvas.Paint(lm.Footer.Rect, renderFooter(in.Screen.Entry(), th, lm.Footer.Rect.Dx()))

	origin := lm.Content().Rect.Min
	if in.FullRegion {
		origin = lm.Main.Rect.Min
		canvas.Paint(lm.Main.Rect, view.Content)
	} else {
		for _, id := range paneRenderOrder {
			pr, ok := lm.Panes[id]
			if !ok {
				continue
			}
			if id == PaneContent {
				canvas.Paint(pr.Rect, view.Content)
				continue
			}
			canvas.Paint(pr.Rect, th.Blank(pr.Ground, pr.Rect.Dx(), pr.Rect.Dy()))
		}

		for _, d := range lm.Dividers {
			if d.Degrade && !th.DrawsDividers() {
				continue // a true-color gutter: Main's blank fill already covers it
			}
			rect := image.Rect(d.X, d.Y0, d.X+1, d.Y1)
			canvas.Paint(rect, dividerColumn(th, rect.Dy()))
		}
	}

	return Frame{Content: canvas.Render(), Cursor: translateCursor(view.Cursor, origin)}
}

// translateCursor offsets c's position by origin, the content pane's
// top-left corner: a screen's View reports its cursor in its
// pane-relative coordinates, and Render's caller needs it in the
// full frame's. Every other Cursor field carries over unchanged. A
// nil c (every pass-2 screen) returns nil.
func translateCursor(c *tea.Cursor, origin image.Point) *tea.Cursor {
	if c == nil {
		return nil
	}
	translated := *c
	translated.Position = tea.Position{X: c.X + origin.X, Y: c.Y + origin.Y}
	return &translated
}

// dividerColumn renders a rows-tall column of th's divider glyph,
// one per line, styled RoleBorder over the base ground every
// LayoutMode divider sits within (the rail/list line and the wide
// rung's degrade-only list/reader line both fall inside Main's
// band).
func dividerColumn(th theme.Theme, rows int) string {
	glyph := th.Glyphs().Divider
	lines := make([]string, rows)
	for i := range lines {
		lines[i] = glyph
	}
	return th.Style(theme.RoleBorder, theme.GroundBase).Render(strings.Join(lines, "\n"))
}

// renderFloorNotice composes the floor state's whole frame (design
// language section 9, wireframe F4): centered, nothing else, naming
// both the minimum size poplar needs (widthSpartanMin, heightShortMin,
// the same constants ComputeLayout's own floor branch tests against)
// and the live width×height that fell under it. It is the one place
// that copy is composed, reached both by Render's floor branch (the
// gallery's route) and by App.View's own floor bypass ahead of the
// modal branch, so the two paths can never drift. The "x" separator
// matches gallerySize.String()'s existing ASCII convention rather
// than a typographic multiplication sign, which the styling analyzer
// would flag as a non-theme literal outside internal/theme.
func renderFloorNotice(th theme.Theme, width, height int) string {
	need := fmt.Sprintf("poplar needs at least %dx%d", widthSpartanMin, heightShortMin)
	got := fmt.Sprintf("this window is %dx%d", width, height)
	body := need + "\n" + got

	style := th.Center(th.Sized(th.Style(theme.RoleFg, theme.GroundBase), width, height))
	return style.Render(body)
}
