package ui

import (
	"image"
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

// Banner is App's own persistent notice state (design decision 2): at
// most one at a time, pushing the main band down one row while Active
// (LayoutMode.BannerRow), dismissed by Esc, before Esc's own back
// handling, or by its own dismiss glyph. It never steals focus:
// App.handleKey lets a focused text-entry context's own Esc through
// untouched instead of consuming it here.
type Banner struct {
	Active  bool
	Message string
}

// BannerMsg shows a banner (design decision 2): it replaces any
// banner already showing, since only one is ever on screen at once.
type BannerMsg struct {
	Message string
}

// renderBanner renders banner's row exactly width cells wide: the
// warn glyph and banner's own message on the left, "Esc dismiss" and
// the dismiss glyph right-aligned (the ratified exemplar's own
// banner_strip composition). A message too long for width is
// truncated with the ellipsis token, the same BACKLOG #61 discipline
// every chrome band holds to.
func renderBanner(banner Banner, th theme.Theme, width int) string {
	right := []rowSeg{
		{text: "Esc", role: theme.RoleFg},
		{text: " dismiss  ", role: theme.RoleFgMuted},
		{text: th.Glyphs().Dismiss, role: theme.RoleFgMuted},
		{text: strings.Repeat(" ", theme.PadBand), role: theme.RoleFg},
	}
	rightWidth := ansi.StringWidth(segsPlainText(right))

	left := []rowSeg{
		{text: strings.Repeat(" ", theme.PadBand), role: theme.RoleFg},
		{text: th.Glyphs().WarnMarker, role: theme.RoleWarn},
		{text: " " + banner.Message, role: theme.RoleFg},
	}
	left = truncateLastSeg(th, left, max(0, width-rightWidth))
	leftWidth := ansi.StringWidth(segsPlainText(left))

	gap := max(0, width-leftWidth-rightWidth)

	var out strings.Builder
	writeSegs(&out, th, left)
	out.WriteString(th.Style(theme.RoleFg, chromeGround).Render(strings.Repeat(" ", gap)))
	writeSegs(&out, th, right)
	return out.String()
}

// BannerHitSpans returns the banner's own dismiss glyph HitSpan
// (ADR-0017's banner-dismiss row), positioned within bannerRow, when
// banner is Active and state is one ADR-0017's own table
// (pointerLegalStates) allows PointerBannerDismiss to fire in. The
// glyph sits PadBand cells from the row's own right edge, the same
// fixed inset renderBanner's own right segment always closes with,
// so this never depends on having rendered first (StatusLineHitSpans's
// own precedent).
func BannerHitSpans(banner Banner, state StateClass, th theme.Theme, bannerRow image.Rectangle) []HitSpan {
	if !banner.Active || !slices.Contains(pointerLegalStates[PointerBannerDismiss], state) {
		return nil
	}
	w := ansi.StringWidth(th.Glyphs().Dismiss)
	x1 := bannerRow.Max.X - theme.PadBand
	x0 := x1 - w
	return []HitSpan{{
		Target: PointerBannerDismiss,
		Verb:   GrammarKeys.Back,
		Rect:   image.Rect(x0, bannerRow.Min.Y, x1, bannerRow.Min.Y+1),
	}}
}
