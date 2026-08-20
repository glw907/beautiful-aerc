package ui

import (
	"image"
	"strings"

	"github.com/glw907/poplar/internal/theme"
)

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
// string for the same three inputs every time (QA-7's purity
// contract).
//
// The signature departs from the survey's abstract shape (screen,
// state, LayoutMode, theme): a Screen is already its own state (elm-
// conventions rule 1), so a fourth "state" parameter would just be
// screen's own fields read back out from outside it. Passing lm and
// th explicitly, rather than reading them off screen, keeps every
// input to the frame's own compositing an argument a caller can see
// and vary, the same "fixture, LayoutMode, theme" triple the seam-
// purity acceptance criterion names.
func Render(screen Screen, lm LayoutMode, th theme.Theme) string {
	if lm.Class == WidthFloor || lm.HeightClass == HeightFloor {
		return screen.View().Content
	}

	canvas := theme.NewCanvas(lm.Width, lm.Height)
	canvas.Paint(lm.StatusRow.Rect, th.Blank(lm.StatusRow.Ground, lm.StatusRow.Rect.Dx(), lm.StatusRow.Rect.Dy()))
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
			canvas.Paint(pr.Rect, screen.View().Content)
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

	return canvas.Render()
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
