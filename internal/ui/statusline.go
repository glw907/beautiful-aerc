package ui

import (
	"image"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

// HitSpan pairs a rendered frame's rectangle with the keyboard verb a
// click there accelerates (ADR-0017's character grain): a chrome
// component registers one per clickable cell at render time, and
// task 10's dispatcher resolves a click coordinate against the set a
// screen's own render produced.
type HitSpan struct {
	Verb key.Binding
	Rect image.Rectangle
}

// StatusLine is the top band's own render state (design decisions 1
// and 7): the compact surface cluster (which surface is active) and
// the sync segment (SY-5's four states, backfill progress riding the
// same segment, and a queued-outbox count beside it). App.View builds
// one from its own model state every frame and hands it to Render,
// which paints it into LayoutMode's StatusRow band; the gallery's
// fixtures pin one directly, with no App or engine involved.
type StatusLine struct {
	Active   Surface
	Sync     SyncStateMsg
	Backfill BackfillProgressMsg
	Outbox   int
	Spinner  int
}

// surfaceNames names each Surface's cluster label, in surfaceForDigit's
// own digit order: index 0 is digit "1", and so on. "People" is
// wireframe F8's own reviewed copy for the contacts surface (task 5a).
var surfaceNames = [...]string{"Mail", "Calendar", "People", "Config"}

// rowSeg is one styled run within a status-row render: statusline.go's
// own composition unit, plain text plus the single role and weight it
// paints under (decision 7: one color per sync state, never a
// per-word rainbow).
type rowSeg struct {
	text string
	role theme.Role
	bold bool
}

// clusterSegs returns the surface cluster's own segments (decision
// 1): a leading padBand inset, then each surface's digit in order,
// the active one named in accent+bold with a wider gap after it, every
// sibling a bare dim digit with a narrow gap after it, and no gap
// after the last.
func clusterSegs(active Surface) []rowSeg {
	segs := []rowSeg{{text: strings.Repeat(" ", theme.PadBand), role: theme.RoleFg}}
	for i, name := range surfaceNames {
		digit := strconv.Itoa(i + 1)
		last := i == len(surfaceNames)-1
		gap := theme.GapLabel
		if Surface(i) == active {
			segs = append(segs, rowSeg{text: digit + " " + name, role: theme.RoleAccent, bold: true})
			gap = theme.GapControl
		} else {
			segs = append(segs, rowSeg{text: digit, role: theme.RoleFgSubtle})
		}
		if !last {
			segs = append(segs, rowSeg{text: strings.Repeat(" ", gap), role: theme.RoleFg})
		}
	}
	return segs
}

// clusterDigitX returns, for each surface in digit order, the column
// (relative to the row's own origin) its digit character starts at:
// the geometry clusterSegs's own construction produces, computed
// independently so a hit span never depends on having rendered first.
func clusterDigitX(active Surface) [len(surfaceNames)]int {
	var xs [len(surfaceNames)]int
	x := theme.PadBand
	for i, name := range surfaceNames {
		xs[i] = x
		last := i == len(surfaceNames)-1
		if Surface(i) == active {
			x += ansi.StringWidth(strconv.Itoa(i+1) + " " + name)
			if !last {
				x += theme.GapControl
			}
			continue
		}
		x++ // the bare digit
		if !last {
			x += theme.GapLabel
		}
	}
	return xs
}

// StatusLineHitSpans returns one HitSpan per surface digit cell
// (ADR-0017 character grain), positioned within statusRow: the status
// line's own registration, task 10's dispatcher resolves a click
// against.
func StatusLineHitSpans(active Surface, statusRow image.Rectangle) []HitSpan {
	xs := clusterDigitX(active)
	spans := make([]HitSpan, len(xs))
	for i, x := range xs {
		origin := statusRow.Min
		spans[i] = HitSpan{
			Verb: GrammarKeys.SurfaceSwitch,
			Rect: image.Rect(origin.X+x, origin.Y, origin.X+x+1, origin.Y+1),
		}
	}
	return spans
}

// progressText renders label plus done/total (decision 7): "of total"
// when a total is known, done alone otherwise, so an unknown total
// never renders as a stalled-looking blank.
func progressText(label string, done, total int64) string {
	if total > 0 {
		return label + " " + formatCount(done) + " of " + formatCount(total)
	}
	return label + " " + formatCount(done)
}

// syncStateSeg returns the sync segment's own single styled run for
// sl: backfill progress takes over the segment while Active (decision
// 7: a rate-limited backfill still rides the same backing-off warn
// state as the mail sync it shares an engine with, rather than
// stalling silently), otherwise SY-5's four states each render their
// own text and role.
func syncStateSeg(th theme.Theme, sl StatusLine) rowSeg {
	spinner := th.Spinner()[sl.Spinner%len(th.Spinner())]

	if sl.Backfill.Active {
		return rowSeg{text: spinner + " " + progressText("bodies", sl.Backfill.Done, sl.Backfill.Total), role: theme.RoleFgMuted}
	}
	switch sl.Sync.State {
	case SyncStateSyncing:
		return rowSeg{text: spinner + " " + progressText("Syncing", sl.Sync.Done, sl.Sync.Total), role: theme.RoleFgMuted}
	case SyncStateOffline:
		return rowSeg{text: "offline", role: theme.RoleWarn}
	case SyncStateBackingOff:
		return rowSeg{text: "backing off " + th.Glyphs().Separator + " retry " + strconv.Itoa(sl.Sync.Retry) + "s", role: theme.RoleWarn}
	default:
		return rowSeg{text: "synced", role: theme.RoleFgMuted}
	}
}

// syncSegments returns the sync segment's own ordered runs: a queued-
// outbox count first when nonzero (decision 7: "beside the sync
// state"), then the sync state's own text.
func syncSegments(th theme.Theme, sl StatusLine) []rowSeg {
	var segs []rowSeg
	if sl.Outbox > 0 {
		segs = append(segs, rowSeg{text: formatCount(int64(sl.Outbox)) + " queued  ", role: theme.RoleFgMuted})
	}
	return append(segs, syncStateSeg(th, sl))
}

// segsPlainText concatenates segs's own text, with no styling: the
// width the whole run occupies, and truncateLastSeg's own input.
func segsPlainText(segs []rowSeg) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteString(s.text)
	}
	return b.String()
}

// truncateLastSeg returns the prefix of segs that fits within
// maxWidth, marking the cut with the theme's ellipsis token (BACKLOG
// #61's overflow defect: the status line never renders past its own
// width). A segment that does not fit at all is dropped rather than
// rendered as a bare ellipsis with nothing of its own text left.
func truncateLastSeg(th theme.Theme, segs []rowSeg, maxWidth int) []rowSeg {
	if maxWidth <= 0 {
		return nil
	}
	ellipsis := th.Glyphs().Ellipsis
	var out []rowSeg
	used := 0
	for _, s := range segs {
		w := ansi.StringWidth(s.text)
		if used+w <= maxWidth {
			out = append(out, s)
			used += w
			continue
		}
		remain := maxWidth - used
		if remain <= 0 {
			break
		}
		clipped := ansi.Truncate(s.text, remain, ellipsis)
		out = append(out, rowSeg{text: clipped, role: s.role, bold: s.bold})
		break
	}
	return out
}

// writeSegs renders each of segs's own styled runs against ground and
// writes them to out, in order.
func writeSegs(out *strings.Builder, th theme.Theme, ground theme.Ground, segs []rowSeg) {
	for _, s := range segs {
		style := th.Style(s.role, ground)
		if s.bold {
			style = style.Bold(true)
		}
		out.WriteString(style.Render(s.text))
	}
}

// renderStatusLine renders sl as one styled row exactly width cells
// wide (BACKLOG #61): the surface cluster at the row's own origin,
// the sync segment right-aligned with a padBand margin. The cluster
// is what orients every other chrome element (decision 1), so a
// width too narrow for both truncates the sync segment first; only
// the sync segment's own text is ever variable enough to overflow in
// practice (BACKLOG #62: every word here comes from a Surface name,
// theme glyph, or the message the bridge delivered, never a literal
// that binding or state could drift from).
func renderStatusLine(sl StatusLine, th theme.Theme, width int) string {
	const ground = theme.GroundPanel

	leftSegs := clusterSegs(sl.Active)
	rightSegs := syncSegments(th, sl)

	leftWidth := ansi.StringWidth(segsPlainText(leftSegs))
	budget := max(0, width-leftWidth-theme.PadBand)

	rightWidth := ansi.StringWidth(segsPlainText(rightSegs))
	if rightWidth > budget {
		rightSegs = truncateLastSeg(th, rightSegs, budget)
		rightWidth = ansi.StringWidth(segsPlainText(rightSegs))
	}

	gap := max(0, width-leftWidth-rightWidth-theme.PadBand)

	var out strings.Builder
	writeSegs(&out, th, ground, leftSegs)
	out.WriteString(th.Style(theme.RoleFg, ground).Render(strings.Repeat(" ", gap)))
	writeSegs(&out, th, ground, rightSegs)
	out.WriteString(th.Style(theme.RoleFg, ground).Render(strings.Repeat(" ", theme.PadBand)))
	return out.String()
}
