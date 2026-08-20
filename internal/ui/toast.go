package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/theme"
)

// UndoOffer is UX-9's undo attachment: a ToastMsg carrying one names
// the action just taken (Label, "3 messages archived") and drives
// what `u` emits within the countdown window (Undo). Pass 3's own
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
// countdown starts at undoWindowSeconds-1 (the pinned exemplar's own
// dim "9s") and counts down to zero, and `u` within the window emits
// Offer.Undo.
type ToastMsg struct {
	Offer UndoOffer
}

// Toast is the status line's own toast presentation, App.statusLine's
// whole view of "a toast is showing" (design decision 1): a zero
// Toast (Active false) renders nothing, so the ordinary sync/outbox
// segment applies unchanged.
type Toast struct {
	Active    bool
	Label     string
	Remaining int
}

// undoWindowSeconds is UX-9's own undo window: `u` within it emits
// the open offer's Undo Cmd. The countdown itself starts one second
// short of this (the pinned exemplar's own dim "9s") since the toast
// already carries its first second of visibility by the time it
// renders.
const undoWindowSeconds = 10

// toastSegs returns t's own right-segment content (design decision 1,
// the pinned exemplar's own toast row): the message, then the undo
// hint, derived from GrammarKeys.Undo rather than a literal "u undo"
// that binding could drift from, and its own dim countdown.
func toastSegs(t Toast) []rowSeg {
	segs := []rowSeg{
		{text: t.Label, role: theme.RoleFg},
		{text: strings.Repeat(" ", theme.GapControl+1), role: theme.RoleFg},
	}
	segs = append(segs, hintSegs(GrammarKeys.Undo)...)
	segs = append(segs, rowSeg{text: "  " + strconv.Itoa(t.Remaining) + "s", role: theme.RoleFgSubtle})
	return segs
}

// bareSyncWord returns the sync segment's own label alone, with no
// progress count or spinner: the compressed form a toast's own right
// segment shows beside it at widthStandardMin and up (the dispatch's
// own ruling: the sync state compresses to its bare word beside the
// toast, and yields entirely below that width).
func bareSyncWord(sl StatusLine) rowSeg {
	switch sl.Sync.State {
	case SyncStateSyncing:
		return rowSeg{text: "Syncing", role: theme.RoleFgMuted}
	case SyncStateOffline:
		return rowSeg{text: "Offline", role: theme.RoleWarn}
	case SyncStateBackingOff:
		return rowSeg{text: "Backing off", role: theme.RoleWarn}
	default:
		return rowSeg{text: "Synced", role: theme.RoleFgMuted}
	}
}

// rightSegments returns the status line's own right-segment content:
// fitRightSegments's ordinary sync/outbox pair, or, while a toast is
// showing, the toast's own content plus the sync state's bare word
// beside it at widthStandardMin and up, dropped entirely below it so
// the toast alone survives at narrower widths.
func rightSegments(th theme.Theme, sl StatusLine, dropTotal bool, width, budget int) []rowSeg {
	if !sl.Toast.Active {
		return fitRightSegments(th, sl, dropTotal, budget)
	}
	segs := toastSegs(sl.Toast)
	if width >= widthStandardMin {
		segs = append(segs, rowSeg{text: strings.Repeat(" ", theme.GapControl), role: theme.RoleFg}, bareSyncWord(sl))
	}
	return truncateLastSeg(th, segs, budget)
}
