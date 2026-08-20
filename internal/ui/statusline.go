package ui

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

// HitSpan pairs a rendered frame's rectangle with the keyboard verb a
// click there accelerates (ADR-0017's character grain) and the
// PointerTarget it stands in for: a chrome component registers one
// per clickable cell at render time, and task 10's dispatcher
// resolves a click coordinate against the set a screen's own render
// produced.
type HitSpan struct {
	Target PointerTarget
	Verb   key.Binding
	Rect   image.Rectangle
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
	Toast    Toast
}

// surfaceNames names each Surface's cluster label, in
// GrammarKeys.SurfaceSwitch's own key order: index 0 pairs with that
// binding's first key, and so on. "People" is wireframe F8's own
// reviewed copy for the contacts surface (task 5a).
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

// chromeGround is the ground both of this package's chrome bands, the
// status line and the footer, paint their rowSeg runs against: the
// one statement task-7-findings-r1.md's F8 collapsed three separately
// declared GroundPanel constants into.
const chromeGround = theme.GroundPanel

// clusterSegs returns the surface cluster's own segments (decision
// 1): a leading padBand inset, then each surface's digit in order,
// the active one named in accent+bold with a wider gap after it, every
// sibling a bare dim digit with a narrow gap after it, and no gap
// after the last. Every digit comes from GrammarKeys.SurfaceSwitch's
// own bound keys, never a literal '1'-'4' that binding could drift
// from (task-6-findings-r1.md F4, BACKLOG #62).
func clusterSegs(active Surface) []rowSeg {
	digits := GrammarKeys.SurfaceSwitch.Keys()
	segs := []rowSeg{{text: strings.Repeat(" ", theme.PadBand), role: theme.RoleFg}}
	for i, name := range surfaceNames {
		last := i == len(surfaceNames)-1
		gap := theme.GapLabel
		if Surface(i) == active {
			segs = append(segs, rowSeg{text: digits[i] + " " + name, role: theme.RoleAccent, bold: true})
			gap = theme.GapControl
		} else {
			segs = append(segs, rowSeg{text: digits[i], role: theme.RoleFgSubtle})
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
	digits := GrammarKeys.SurfaceSwitch.Keys()
	var xs [len(surfaceNames)]int
	x := theme.PadBand
	for i, name := range surfaceNames {
		xs[i] = x
		last := i == len(surfaceNames)-1
		if Surface(i) == active {
			x += ansi.StringWidth(digits[i] + " " + name)
			if !last {
				x += theme.GapControl
			}
			continue
		}
		x += ansi.StringWidth(digits[i])
		if !last {
			x += theme.GapLabel
		}
	}
	return xs
}

// StatusLineHitSpans returns one HitSpan per surface digit cell
// (ADR-0017 character grain), positioned within statusRow, when state
// is one of the states ADR-0017's own table (registry.go's
// pointerLegalStates) allows PointerSurfaceDigit to fire in. Every
// other state, a modal most notably, returns none: a digit click is
// never offered where the keyboard equivalent would be illegal too
// (task-6-findings-r1.md F8/RULING).
func StatusLineHitSpans(active Surface, state StateClass, statusRow image.Rectangle) []HitSpan {
	if !slices.Contains(pointerLegalStates[PointerSurfaceDigit], state) {
		return nil
	}
	xs := clusterDigitX(active)
	spans := make([]HitSpan, len(xs))
	for i, x := range xs {
		origin := statusRow.Min
		spans[i] = HitSpan{
			Target: PointerSurfaceDigit,
			Verb:   GrammarKeys.SurfaceSwitch,
			Rect:   image.Rect(origin.X+x, origin.Y, origin.X+x+1, origin.Y+1),
		}
	}
	return spans
}

// progressText renders label plus done/total (decision 7): "of total"
// when a total is known, done alone when only a count is known, and
// the label alone when neither is (Done==0 && Total==0): the bridge's
// own honest starting value until a later pass adds live progress
// tracking to sync.Health (task-6-findings-r1.md F7). An unknown "0"
// would read as a cycle stalled at zero rather than one that has not
// reported a count yet.
func progressText(label string, done, total int64) string {
	switch {
	case total > 0:
		return label + " " + formatCount(done) + " of " + formatCount(total)
	case done > 0:
		return label + " " + formatCount(done)
	default:
		return label
	}
}

// backingOffText renders the backing-off state's own text (decision
// 7, sentence case per task-6-findings-r1.md F15): "Backing off ·
// retry Ns".
func backingOffText(th theme.Theme, retrySeconds int) string {
	return "Backing off " + th.Glyphs().Separator + " retry " + strconv.Itoa(retrySeconds) + "s"
}

// syncStateSeg returns the sync segment's own single styled run for
// sl (decision 7, sentence case per F15): backfill progress takes
// over the segment while Active, otherwise SY-5's four states each
// render their own text and role. dropTotal drops a known total from
// the progress text (F6: the short-height rung, per wireframe F3).
func syncStateSeg(th theme.Theme, sl StatusLine, dropTotal bool) rowSeg {
	if sl.Backfill.Active {
		return backfillSeg(th, sl, dropTotal)
	}
	switch sl.Sync.State {
	case SyncStateSyncing:
		spinner := th.Spinner()[sl.Spinner%len(th.Spinner())]
		return rowSeg{text: spinner + " " + progressText("Syncing", sl.Sync.Done, syncTotal(sl.Sync.Total, dropTotal)), role: theme.RoleFgMuted}
	case SyncStateOffline:
		return rowSeg{text: "Offline", role: theme.RoleWarn}
	case SyncStateBackingOff:
		return rowSeg{text: backingOffText(th, sl.Sync.Retry), role: theme.RoleWarn}
	default:
		return rowSeg{text: "Synced", role: theme.RoleFgMuted}
	}
}

// backfillSeg returns the backfill progress state's own styled run: a
// rate-limited backfill (Sync.State BackingOff or Offline while
// Backfill.Active) still rides the underlying sync connection's own
// warn state and retry text, rather than painting calm while the
// connection it depends on is degraded (task-6-findings-r1.md F2).
func backfillSeg(th theme.Theme, sl StatusLine, dropTotal bool) rowSeg {
	spinner := th.Spinner()[sl.Spinner%len(th.Spinner())]
	text := spinner + " " + progressText("Bodies", sl.Backfill.Done, syncTotal(sl.Backfill.Total, dropTotal))
	sep := th.Glyphs().Separator
	switch sl.Sync.State {
	case SyncStateBackingOff:
		return rowSeg{text: text + " " + sep + " retry " + strconv.Itoa(sl.Sync.Retry) + "s", role: theme.RoleWarn}
	case SyncStateOffline:
		return rowSeg{text: text + " " + sep + " offline", role: theme.RoleWarn}
	default:
		return rowSeg{text: text, role: theme.RoleFgMuted}
	}
}

// syncTotal returns total, or zero when dropTotal is set: the one
// place progressText's "of total" clause is suppressed at the
// short-height rung (F6).
func syncTotal(total int64, dropTotal bool) int64 {
	if dropTotal {
		return 0
	}
	return total
}

// outboxSegment returns the queued-outbox count's own styled run
// (decision 7: "beside the sync state"), and whether sl has one to
// show at all (Outbox <= 0 shows nothing).
func outboxSegment(sl StatusLine) (rowSeg, bool) {
	if sl.Outbox <= 0 {
		return rowSeg{}, false
	}
	text := formatCount(int64(sl.Outbox)) + " queued" + strings.Repeat(" ", theme.GapControl)
	return rowSeg{text: text, role: theme.RoleFgMuted}, true
}

// fitRightSegments returns the sync segment's own display-ordered
// runs that fit within budget: the outbox count first when shown,
// then the sync state. Truncation priority is the reverse of display
// order (task-6-findings-r1.md F12, decision 7: the outbox count
// rides beside the sync state, never instead of it): the sync state
// is primary and is dropped only as the very last resort (truncated
// with the ellipsis token); the outbox count is supplementary and is
// the first thing evicted under space pressure.
func fitRightSegments(th theme.Theme, sl StatusLine, dropTotal bool, budget int) []rowSeg {
	stateSeg := syncStateSeg(th, sl, dropTotal)
	stateWidth := ansi.StringWidth(stateSeg.text)

	if outboxSeg, ok := outboxSegment(sl); ok {
		if ansi.StringWidth(outboxSeg.text)+stateWidth <= budget {
			return []rowSeg{outboxSeg, stateSeg}
		}
	}
	if stateWidth <= budget {
		return []rowSeg{stateSeg}
	}
	return truncateLastSeg(th, []rowSeg{stateSeg}, budget)
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

// writeSegs writes segs to out, each styled against chromeGround.
func writeSegs(out *strings.Builder, th theme.Theme, segs []rowSeg) {
	for _, s := range segs {
		style := th.Style(s.role, chromeGround)
		if s.bold {
			style = th.EmphasizeRole(s.role, chromeGround)
		}
		out.WriteString(style.Render(s.text))
	}
}

// renderStatusLine renders sl as one styled row exactly width cells
// wide (BACKLOG #61): the surface cluster at the row's own origin,
// the sync segment right-aligned with a padBand margin. dropTotal
// drops a known progress total (F6, the short-height rung). The
// cluster is what orients every other chrome element (decision 1), so
// a width too narrow for both truncates the sync segment first; only
// the sync segment's own text is ever variable enough to overflow in
// practice (BACKLOG #62: every word here comes from a Surface name,
// a GrammarKeys binding, a theme glyph, or the message the bridge
// delivered, never a literal that binding or state could drift from).
// A showing toast (sl.Toast.Active) takes over the right segment
// instead (rightSegments): the sync state compresses to its bare word
// beside it, or yields entirely below widthStandardMin.
func renderStatusLine(sl StatusLine, th theme.Theme, width int, dropTotal bool) string {
	leftSegs := clusterSegs(sl.Active)
	leftWidth := ansi.StringWidth(segsPlainText(leftSegs))
	budget := max(0, width-leftWidth-theme.PadBand)

	rightSegs := rightSegments(th, sl, dropTotal, width, budget)
	rightWidth := ansi.StringWidth(segsPlainText(rightSegs))

	gap := max(0, width-leftWidth-rightWidth-theme.PadBand)

	var out strings.Builder
	writeSegs(&out, th, leftSegs)
	out.WriteString(th.Style(theme.RoleFg, chromeGround).Render(strings.Repeat(" ", gap)))
	writeSegs(&out, th, rightSegs)
	out.WriteString(th.Style(theme.RoleFg, chromeGround).Render(strings.Repeat(" ", theme.PadBand)))
	return out.String()
}
