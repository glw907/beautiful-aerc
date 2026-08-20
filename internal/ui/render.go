package ui

import (
	"image"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/theme"
)

// Frame is Render's own result: the composed frame content, plus a
// cursor translated into the frame's coordinate space when screen's
// own View supplied one. Cursor is nil for every pass-2 screen (none
// accepts text entry yet); pass 4's first text-entry screen is the
// first to set one, which is why the translation exists now rather
// than as a signature break later.
type Frame struct {
	Content string
	Cursor  *tea.Cursor
}

// paneRenderOrder is the order Render composes LayoutMode's named
// panes in, after Main's own ground has already painted the whole
// band beneath them (LayoutMode.Main's own doc comment). Sidebar and
// Split get a blank ground fill this pass, since no chrome component
// reaches them until the chrome tasks that follow, while Content
// gets the screen's own already-rendered View.
var paneRenderOrder = []PaneID{PaneSidebar, PaneContent, PaneSplit}

// Render is poplar's pure render seam (survey amendment A): it
// composes screen's own View (already rendered against lm and th,
// since a caller applies both via LayoutMsg and ThemeMsg before
// calling Render) into the full frame lm describes, painting every
// band's own ground before a named pane's content overlays it, so
// every row LayoutMode allocates is accounted for (LayoutMode's own
// row-tiling and pane-containment guarantees). Below ProfileTrueColor
// a ground carries no distinguishing color (decision 11), so a
// blank, uncolored row's trailing cells render as trimmed whitespace
// rather than styled spaces. The coverage invariant's content half,
// every cell explicitly painted, holds at ProfileTrueColor, where
// every ground, including a blank one, sets an explicit background.
// Render runs no tea.Program and does no I/O; it returns the same
// Frame for the same four inputs every time (QA-7's purity contract).
// The three-argument shape recorded in the pass 2 plan's task 5b
// findings grew a fourth at task 6: status, App's own chrome state,
// is what the seam paints into StatusRow rather than a blank fill.
func Render(screen Screen, lm LayoutMode, th theme.Theme, status StatusLine) Frame {
	view := screen.View()

	if lm.Class == WidthFloor || lm.HeightClass == HeightFloor {
		return Frame{Content: view.Content, Cursor: view.Cursor}
	}

	canvas := theme.NewCanvas(lm.Width, lm.Height)
	dropTotal := lm.HeightClass != HeightFull
	canvas.Paint(lm.StatusRow.Rect, renderStatusLine(status, th, lm.StatusRow.Rect.Dx(), dropTotal))
	if lm.BannerRow {
		canvas.Paint(lm.Banner.Rect, th.Blank(lm.Banner.Ground, lm.Banner.Rect.Dx(), lm.Banner.Rect.Dy()))
	}
	canvas.Paint(lm.Main.Rect, th.Blank(lm.Main.Ground, lm.Main.Rect.Dx(), lm.Main.Rect.Dy()))
	canvas.Paint(lm.Footer.Rect, th.Blank(lm.Footer.Ground, lm.Footer.Rect.Dx(), lm.Footer.Rect.Dy()))

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
			continue // a true-color gutter: Main's own blank fill already covers it
		}
		rect := image.Rect(d.X, d.Y0, d.X+1, d.Y1)
		canvas.Paint(rect, dividerColumn(th, rect.Dy()))
	}

	return Frame{Content: canvas.Render(), Cursor: translateCursor(view.Cursor, lm.Content().Rect.Min)}
}

// translateCursor offsets c's position by origin, the content pane's
// own top-left corner: a screen's View reports its cursor in its own
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
// rung's degrade-only list/reader line both fall inside Main's own
// band).
func dividerColumn(th theme.Theme, rows int) string {
	glyph := th.Glyphs().Divider
	lines := make([]string, rows)
	for i := range lines {
		lines[i] = glyph
	}
	return th.Style(theme.RoleBorder, theme.GroundBase).Render(strings.Join(lines, "\n"))
}
