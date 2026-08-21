package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

// UndoOffer is UX-9's undo attachment: a ToastMsg carrying one names
// the action just taken (Label, "3 messages archived") and drives
// what `u` emits within the countdown window (Undo). Pass 3's
// mutation triage constructs one per undoable action; this pass
// exercises the presentation with a fake action.
type UndoOffer struct {
	Label string
	Undo  tea.Cmd
}

// ToastMsg shows a toast in the status line's right segment (design
// decision 1): newest wins over one already showing, and every toast
// App absorbs logs exactly one ER-1 line (App.showToast). Offer
// drives the whole presentation this pass's toast rides: the
// countdown starts at undoWindowSeconds-1 (the pinned exemplar's
// dim "9s") and counts down to zero, and `u` within the window emits
// Offer.Undo.
type ToastMsg struct {
	Offer UndoOffer
}

// Toast is the status line's toast presentation, App.statusLine's
// whole view of "a toast is showing" (design decision 1): a zero
// Toast (Active false) renders nothing, so the ordinary sync/outbox
// segment applies unchanged. Undoable gates the undo hint and
// countdown (task-8-findings-r1.md F8): a toast with nothing to undo
// (App.toast's Offer.Undo == nil case) shows its label alone,
// never a hint advertising a dead `u`.
type Toast struct {
	Active    bool
	Label     string
	Remaining int
	Undoable  bool
}

// undoWindowSeconds is UX-9's undo window: `u` within it emits
// the open offer's Undo Cmd. The countdown itself starts one second
// short of this (the pinned exemplar's dim "9s") since the toast
// already carries its first second of visibility by the time it
// renders.
const undoWindowSeconds = 10

// toastTailSegs returns t's fixed right-hand tail: the undo hint,
// derived from GrammarKeys.Undo rather than a literal "u undo" that
// binding could drift from, and its dim countdown, when Undoable
// (F8). A toast with nothing to undo returns nil: label alone, no
// hint.
func toastTailSegs(t Toast) []rowSeg {
	if !t.Undoable {
		return nil
	}
	segs := append([]rowSeg{}, hintSegs(GrammarKeys.Undo)...)
	return append(segs, rowSeg{text: "  " + strconv.Itoa(t.Remaining) + "s", role: theme.RoleFgSubtle})
}

// toastRightSegs returns t's right-segment content at budget
// cells (task-8-findings-r1.md F1, CRITICAL): the tail (toastTailSegs)
// reserves its width first, with a theme.GapControl gap ahead of
// it (F6: the one gap unit, not a component-specific literal), and
// only the label truncates to what remains, with the ellipsis token.
// The tail is never what evicts: UX-9's visible-countdown MUST
// depends on it surviving whenever there is room for it at all.
func toastRightSegs(th theme.Theme, t Toast, budget int) []rowSeg {
	tail := toastTailSegs(t)
	tailWidth := ansi.StringWidth(segsPlainText(tail))
	gap := 0
	if tailWidth > 0 {
		gap = theme.GapControl
	}
	labelBudget := max(0, budget-tailWidth-gap)

	segs := truncateLastSeg(th, []rowSeg{{text: t.Label, role: theme.RoleFg}}, labelBudget)
	if tailWidth == 0 {
		return segs
	}
	segs = append(segs, rowSeg{text: strings.Repeat(" ", gap), role: theme.RoleFg})
	return append(segs, tail...)
}

// bareSyncWord returns the sync segment's label alone, with no
// progress count or spinner: the compressed form a toast's right
// segment shows beside it at widthStandardMin and up (the dispatch's
// own ruling: the sync state compresses to its bare word beside the
// toast, and yields entirely below that width). syncWord is the one
// place the four SY-5 copy strings live (task-8-findings-r1.md F5).
func bareSyncWord(sl StatusLine) rowSeg {
	text, role := syncWord(sl.Sync.State)
	return rowSeg{text: text, role: role}
}

// rightSegments returns the status line's right-segment content:
// fitRightSegments's ordinary sync/outbox pair, or, while a toast is
// showing, the toast's content plus the sync state's bare word
// beside it at widthStandardMin and up. The toast branch reserves its
// own theme.GapControl off budget up front (F1, CRITICAL): the gap
// between the cluster and the toast's content is never squeezed
// to zero the way an unreserved budget let it at narrow widths (the
// probed 60-column case). The bare sync word is appended only when it
// still fits within that same reserved budget, so this never returns
// content wider than budget regardless of width.
func rightSegments(th theme.Theme, sl StatusLine, dropTotal bool, width, budget int) []rowSeg {
	if !sl.Toast.Active {
		return fitRightSegments(th, sl, dropTotal, budget)
	}
	toastBudget := max(0, budget-theme.GapControl)
	segs := toastRightSegs(th, sl.Toast, toastBudget)
	if width < widthStandardMin {
		return segs
	}

	word := bareSyncWord(sl)
	wordSeg := []rowSeg{{text: strings.Repeat(" ", theme.GapControl), role: theme.RoleFg}, word}
	segsWidth := ansi.StringWidth(segsPlainText(segs))
	wordWidth := ansi.StringWidth(segsPlainText(wordSeg))
	if segsWidth+wordWidth > toastBudget {
		return segs
	}
	return append(segs, wordSeg...)
}
