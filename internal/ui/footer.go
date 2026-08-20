package ui

import (
	"image"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

// hintSegs returns b's own footer hint atom (decision 8, the ratified
// shell exemplar): its key in RoleFg, then its description in
// RoleFgMuted with the atom's single separating space. Both runs
// derive from b.Help(), never a literal, so a hint can never drift
// from the binding it advertises (BACKLOG #62's defect class).
func hintSegs(b key.Binding) []rowSeg {
	return []rowSeg{
		{text: b.Help().Key, role: theme.RoleFg},
		{text: " " + b.Help().Desc, role: theme.RoleFgMuted},
	}
}

// footerLeftSegs returns entry's own rendered footer hints at width
// content columns (footerHints's width-maximal prefix), each hint's
// atom joined to its neighbor by a GapHint gap.
func footerLeftSegs(entry ScreenEntry, contentWidth int) []rowSeg {
	var segs []rowSeg
	for i, b := range footerHints(entry, contentWidth) {
		if i > 0 {
			segs = append(segs, rowSeg{text: strings.Repeat(" ", theme.GapHint), role: theme.RoleFg})
		}
		segs = append(segs, hintSegs(b)...)
	}
	return segs
}

// renderFooter renders entry's footer band exactly width cells wide
// (decision 8): the width-maximal prefix footerHints computes, inset
// PadBand from each edge, and GrammarKeys.Help pinned right as the
// constant pointer to the help overlay. footerHints's own reserve for
// the pinned hint (footerHelpHint's width plus GapPin) is what
// guarantees the gap between the last rendered hint and the pinned
// one never falls below GapPin, so this never has to clamp for room.
// width is renderFooter's own precondition: its only caller, Render,
// always passes lm.Footer.Rect.Dx(), and ComputeLayout (layout.go)
// never allocates a Footer band narrower than widthSpartanMin, so
// this never defends against a width no caller actually supplies.
func renderFooter(entry ScreenEntry, th theme.Theme, width int) string {
	contentWidth := width - 2*theme.PadBand
	leftSegs := footerLeftSegs(entry, contentWidth)
	leftWidth := ansi.StringWidth(segsPlainText(leftSegs))

	rightSegs := hintSegs(GrammarKeys.Help)
	rightWidth := ansi.StringWidth(segsPlainText(rightSegs))

	gap := contentWidth - leftWidth - rightWidth

	var out strings.Builder
	pad := th.Style(theme.RoleFg, chromeGround).Render(strings.Repeat(" ", theme.PadBand))
	out.WriteString(pad)
	writeSegs(&out, th, leftSegs)
	out.WriteString(th.Style(theme.RoleFg, chromeGround).Render(strings.Repeat(" ", gap)))
	writeSegs(&out, th, rightSegs)
	out.WriteString(pad)
	return out.String()
}

// FooterHitSpans returns one HitSpan per rendered footer hint,
// entry's committed prefix at footerRow's own width plus the pinned
// help hint (ADR-0017's character grain: each span covers exactly the
// hint's rendered cells, from its key's first column through its
// description's last), positioned within footerRow. state gates the
// whole set through registry.go's pointerLegalStates the same way
// StatusLineHitSpans does: PointerFooterHint is legal in every
// StateClass except StateModal (task-10-findings-r2.md's F2 corollary
// ruling: the modal renders full-terminal, App.View's own StateModal
// branch, so no footer band exists there to click), and an empty
// footerRow (the floor rung, which paints no chrome at all) also
// returns none, belt-and-braces against a caller that resolves spans
// without checking the rung first.
func FooterHitSpans(entry ScreenEntry, state StateClass, footerRow image.Rectangle) []HitSpan {
	if footerRow.Empty() || !slices.Contains(pointerLegalStates[PointerFooterHint], state) {
		return nil
	}

	width := footerRow.Dx()
	contentWidth := max(0, width-2*theme.PadBand)
	hints := footerHints(entry, contentWidth)

	origin := footerRow.Min
	y0, y1 := origin.Y, origin.Y+1
	x := origin.X + theme.PadBand

	spans := make([]HitSpan, 0, len(hints)+1)
	for _, b := range hints {
		w := ansi.StringWidth(hintText(b))
		spans = append(spans, HitSpan{
			Target: PointerFooterHint,
			Verb:   b,
			Rect:   image.Rect(x, y0, x+w, y1),
		})
		x += w + theme.GapHint
	}

	helpWidth := ansi.StringWidth(hintText(GrammarKeys.Help))
	helpX := origin.X + width - theme.PadBand - helpWidth
	spans = append(spans, HitSpan{
		Target: PointerFooterHint,
		Verb:   GrammarKeys.Help,
		Rect:   image.Rect(helpX, y0, helpX+helpWidth, y1),
	})
	return spans
}
