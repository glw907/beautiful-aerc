package ui

import (
	"image"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

const statusLineTestWidth = 100

func plainStatusLine(sl StatusLine, th theme.Theme) string {
	return ansi.Strip(renderStatusLine(sl, th, statusLineTestWidth))
}

// TestRenderStatusLine_ClusterNamesTheActiveSurface proves decision
// 1's compact cluster: the active surface is named in the row, every
// sibling is a bare digit, and every surface name comes from
// surfaceNames rather than a hardcoded literal wherever it appears
// (BACKLOG #62's hardcoded-hint defect shape).
func TestRenderStatusLine_ClusterNamesTheActiveSurface(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)

	tests := []struct {
		active Surface
		want   string
	}{
		{SurfaceMail, "1 Mail  2 3 4"},
		{SurfaceCalendar, "1 2 Calendar  3 4"},
		{SurfaceContacts, "1 2 3 People  4"},
		{SurfaceConfig, "1 2 3 4 Config"},
	}
	for _, tt := range tests {
		got := plainStatusLine(StatusLine{Active: tt.active}, th)
		if !strings.Contains(got, tt.want) {
			t.Errorf("renderStatusLine(active=%v) = %q, want it to contain %q", tt.active, got, tt.want)
		}
	}
}

// TestRenderStatusLine_SY5States proves all four SY-5 states render
// distinct, decision-7-matching text: synced dim, syncing with a
// spinner and known progress, offline, and backing off with its own
// retry countdown.
func TestRenderStatusLine_SY5States(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)

	tests := []struct {
		name string
		sync SyncStateMsg
		want string
	}{
		{"synced", SyncStateMsg{State: SyncStateSynced}, "synced"},
		{"syncing with a known total", SyncStateMsg{State: SyncStateSyncing, Done: 4312, Total: 36102}, "Syncing 4,312 of 36,102"},
		{"syncing with no known total", SyncStateMsg{State: SyncStateSyncing, Done: 512}, "Syncing 512"},
		{"offline", SyncStateMsg{State: SyncStateOffline}, "offline"},
		{"backing off", SyncStateMsg{State: SyncStateBackingOff, Retry: 12}, "backing off " + th.Glyphs().Separator + " retry 12s"},
	}
	seen := make(map[string]bool)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := plainStatusLine(StatusLine{Sync: tt.sync}, th)
			if !strings.Contains(got, tt.want) {
				t.Errorf("renderStatusLine(%+v) = %q, want it to contain %q", tt.sync, got, tt.want)
			}
			if seen[got] {
				t.Errorf("state %q rendered the same row as an earlier state, want every SY-5 state to render distinctly", tt.name)
			}
			seen[got] = true
		})
	}
}

// TestRenderStatusLine_SyncingShowsTheSpinnerFrame proves the spinner
// glyph advances with StatusLine.Spinner, cycling through theme's own
// frame set (never a literal glyph of the row's own).
func TestRenderStatusLine_SyncingShowsTheSpinnerFrame(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	frames := th.Spinner()

	for i, frame := range frames {
		sl := StatusLine{Sync: SyncStateMsg{State: SyncStateSyncing, Done: 1}, Spinner: i}
		got := plainStatusLine(sl, th)
		if !strings.Contains(got, frame) {
			t.Errorf("Spinner=%d: renderStatusLine() = %q, want it to contain frame %q", i, got, frame)
		}
	}
}

// TestRenderStatusLine_BackfillRidesTheSyncSegment proves decision
// 7's backfill behavior: Active takes over the segment with its own
// "bodies" progress text, and the underlying sync state's own text is
// no longer shown, so a rate-limited backfill never reads as a silent
// stall.
func TestRenderStatusLine_BackfillRidesTheSyncSegment(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	sl := StatusLine{
		Sync:     SyncStateMsg{State: SyncStateSynced},
		Backfill: BackfillProgressMsg{Active: true, Done: 18204, Total: 36102},
	}
	got := plainStatusLine(sl, th)
	if !strings.Contains(got, "bodies 18,204 of 36,102") {
		t.Errorf("renderStatusLine() = %q, want the backfill progress text", got)
	}
	if strings.Contains(got, "synced") {
		t.Errorf("renderStatusLine() = %q, want the underlying synced state hidden while backfill is active", got)
	}
}

// TestRenderStatusLine_OutboxQueuedShowsBesideSyncState proves
// decision 7's queued-outbox count: shown only when nonzero, beside
// the sync state.
func TestRenderStatusLine_OutboxQueuedShowsBesideSyncState(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)

	idle := plainStatusLine(StatusLine{Sync: SyncStateMsg{State: SyncStateSynced}}, th)
	if strings.Contains(idle, "queued") {
		t.Errorf("renderStatusLine() with a zero outbox = %q, want no queued text", idle)
	}

	queued := plainStatusLine(StatusLine{Sync: SyncStateMsg{State: SyncStateSynced}, Outbox: 2}, th)
	if !strings.Contains(queued, "2 queued") {
		t.Errorf("renderStatusLine() with Outbox=2 = %q, want %q", queued, "2 queued")
	}
	if !strings.Contains(queued, "synced") {
		t.Errorf("renderStatusLine() with Outbox=2 = %q, want the sync state still shown beside it", queued)
	}
}

// TestRenderStatusLine_NeverExceedsWidth is the property test BACKLOG
// #61 owes: over a wide range of long sync-segment inputs at the 60-
// column floor, the row's own display width is exactly 60, never
// more, and the ellipsis token appears whenever the content would
// otherwise have overflowed (marked truncation, never a silent clip).
func TestRenderStatusLine_NeverExceedsWidth(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	const width = 60
	ellipsis := th.Glyphs().Ellipsis

	long := []SyncStateMsg{
		{State: SyncStateSyncing, Done: 1, Total: 2},
		{State: SyncStateSyncing, Done: 123456789012345, Total: 987654321098765},
		{State: SyncStateBackingOff, Retry: 1},
		{State: SyncStateBackingOff, Retry: 999999999},
		{State: SyncStateOffline},
	}
	truncated := 0
	for _, sync := range long {
		for outbox := range 3 {
			sl := StatusLine{Sync: sync, Outbox: outbox}
			row := renderStatusLine(sl, th, width)
			plain := ansi.Strip(row)
			if got := ansi.StringWidth(plain); got != width {
				t.Fatalf("renderStatusLine(%+v, outbox=%d) display width = %d, want exactly %d\nrow: %q", sync, outbox, got, width, plain)
			}

			// natural is the row's own would-be width with no
			// truncation at all: the same three terms
			// renderStatusLine sums before ever consulting width.
			natural := ansi.StringWidth(segsPlainText(clusterSegs(sl.Active))) +
				ansi.StringWidth(segsPlainText(syncSegments(th, sl))) + theme.PadBand
			if natural > width {
				truncated++
				if !strings.Contains(plain, ellipsis) {
					t.Errorf("renderStatusLine(%+v, outbox=%d) natural width %d > %d with no ellipsis marker: %q", sync, outbox, natural, width, plain)
				}
			}
		}
	}
	if truncated == 0 {
		t.Fatal("no case in this table actually overflowed width, so the ellipsis-marking assertion never ran")
	}
}

// TestStatusLineHitSpans_MatchesTheRenderedDigitColumns proves the
// hit spans ADR-0017's character grain requires: each span's Rect
// lands on the exact column the rendered string's own digit
// character sits at.
func TestStatusLineHitSpans_MatchesTheRenderedDigitColumns(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)

	for active := SurfaceMail; active <= SurfaceConfig; active++ {
		row := ansi.Strip(renderStatusLine(StatusLine{Active: active}, th, 100))
		statusRow := image.Rect(0, 5, 100, 6) // an arbitrary non-zero origin, proving translation
		spans := StatusLineHitSpans(active, statusRow)

		if len(spans) != 4 {
			t.Fatalf("StatusLineHitSpans(%v) returned %d spans, want 4", active, len(spans))
		}
		for i, span := range spans {
			if span.Rect.Min.Y != statusRow.Min.Y || span.Rect.Max.Y != statusRow.Min.Y+1 {
				t.Errorf("digit %d: Rect = %v, want a 1-row span at Y=%d", i+1, span.Rect, statusRow.Min.Y)
			}
			x := span.Rect.Min.X - statusRow.Min.X
			want := rune('1' + i)
			if x < 0 || x >= len(row) || rune(row[x]) != want {
				t.Errorf("active=%v digit %d: span X=%d, row column there = %q, want %q\nrow: %q", active, i+1, x, safeRune(row, x), want, row)
			}
			if !span.Verb.Enabled() || span.Verb.Help().Desc != GrammarKeys.SurfaceSwitch.Help().Desc {
				t.Errorf("digit %d: Verb = %v, want GrammarKeys.SurfaceSwitch", i+1, span.Verb)
			}
		}
	}
}

func safeRune(s string, i int) rune {
	if i < 0 || i >= len(s) {
		return 0
	}
	return rune(s[i])
}
