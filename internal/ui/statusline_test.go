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
	return ansi.Strip(renderStatusLine(sl, th, statusLineTestWidth, false))
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
// distinct, decision-7-matching text in sentence case
// (task-6-findings-r1.md F15): Synced dim, Syncing with a spinner and
// known progress, Offline, and Backing off with its retry
// countdown.
func TestRenderStatusLine_SY5States(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)

	tests := []struct {
		name string
		sync SyncStateMsg
		want string
	}{
		{"synced", SyncStateMsg{State: SyncStateSynced}, "Synced"},
		{"syncing with a known total", SyncStateMsg{State: SyncStateSyncing, Done: 4312, Total: 36102}, "Syncing 4,312 of 36,102"},
		{"syncing with no known total", SyncStateMsg{State: SyncStateSyncing, Done: 512}, "Syncing 512"},
		{"offline", SyncStateMsg{State: SyncStateOffline}, "Offline"},
		{"backing off", SyncStateMsg{State: SyncStateBackingOff, Retry: 12}, "Backing off " + th.Glyphs().Separator + " retry 12s"},
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
// glyph advances with StatusLine.Spinner, cycling through theme's
// frame set (never a literal glyph of the row's).
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

// TestRenderStatusLine_UnknownProgressRendersBareLabel proves F7:
// Done==0 && Total==0 (the bridge's honest starting value before
// any progress is known) renders the label alone, never a misleading
// "Syncing 0" that reads as a cycle stalled at zero.
func TestRenderStatusLine_UnknownProgressRendersBareLabel(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)

	got := plainStatusLine(StatusLine{Sync: SyncStateMsg{State: SyncStateSyncing}}, th)
	if !strings.Contains(got, "Syncing") {
		t.Errorf("renderStatusLine() = %q, want it to contain the bare label %q", got, "Syncing")
	}
	if strings.Contains(got, "Syncing 0") {
		t.Errorf("renderStatusLine() = %q, want no misleading zero count", got)
	}
}

// TestRenderStatusLine_BackfillRidesTheSyncSegment proves decision
// 7's backfill behavior: Active takes over the segment with its
// "Bodies" progress text (sentence case, F15), and the underlying
// synced state's text is no longer shown, so a rate-limited
// backfill never reads as a silent stall.
func TestRenderStatusLine_BackfillRidesTheSyncSegment(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	sl := StatusLine{
		Sync:     SyncStateMsg{State: SyncStateSynced},
		Backfill: BackfillProgressMsg{Active: true, Done: 18204, Total: 36102},
	}
	got := plainStatusLine(sl, th)
	if !strings.Contains(got, "Bodies 18,204 of 36,102") {
		t.Errorf("renderStatusLine() = %q, want the backfill progress text", got)
	}
	if strings.Contains(got, "Synced") {
		t.Errorf("renderStatusLine() = %q, want the underlying synced state hidden while backfill is active", got)
	}
}

// TestBackfillSeg_RateLimitedRendersWarn proves F2: a backfill active
// while the underlying sync connection is degraded (BackingOff or
// Offline) renders RoleWarn with a retry cue, rather than painting
// calm while the connection it depends on is throttled or down.
// Checked directly against the unexported rowSeg the render composes
// from, rather than an ANSI-styled string, since role is exactly what
// this proves.
func TestBackfillSeg_RateLimitedRendersWarn(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	backfill := BackfillProgressMsg{Active: true, Done: 100, Total: 200}

	tests := []struct {
		name     string
		sync     SyncStateMsg
		wantRole theme.Role
		wantText string
	}{
		{"backing off", SyncStateMsg{State: SyncStateBackingOff, Retry: 12}, theme.RoleWarn, th.Glyphs().Separator + " retry 12s"},
		{"offline", SyncStateMsg{State: SyncStateOffline}, theme.RoleWarn, th.Glyphs().Separator + " offline"},
		{"synced (the calm case)", SyncStateMsg{State: SyncStateSynced}, theme.RoleFgMuted, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seg := backfillSeg(th, StatusLine{Sync: tt.sync, Backfill: backfill}, false)
			if seg.role != tt.wantRole {
				t.Errorf("backfillSeg(%s).role = %v, want %v", tt.name, seg.role, tt.wantRole)
			}
			if !strings.Contains(seg.text, "Bodies 100 of 200") {
				t.Errorf("backfillSeg(%s).text = %q, want it to still carry the progress text", tt.name, seg.text)
			}
			if tt.wantText != "" && !strings.Contains(seg.text, tt.wantText) {
				t.Errorf("backfillSeg(%s).text = %q, want it to contain %q", tt.name, seg.text, tt.wantText)
			}
		})
	}
}

// TestRenderStatusLine_DropTotalHidesTheKnownTotal proves F6: at the
// short-height rung (wireframe F3), the sync segment drops a known
// total from both the ordinary syncing state and backfill progress,
// showing the count alone.
func TestRenderStatusLine_DropTotalHidesTheKnownTotal(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)

	syncing := StatusLine{Sync: SyncStateMsg{State: SyncStateSyncing, Done: 4312, Total: 36102}}
	if got := renderStatusLine(syncing, th, statusLineTestWidth, true); strings.Contains(ansi.Strip(got), "of 36,102") {
		t.Errorf("renderStatusLine(dropTotal=true) = %q, want the total dropped", ansi.Strip(got))
	}
	if got := renderStatusLine(syncing, th, statusLineTestWidth, false); !strings.Contains(ansi.Strip(got), "of 36,102") {
		t.Errorf("renderStatusLine(dropTotal=false) = %q, want the total kept", ansi.Strip(got))
	}

	backfill := StatusLine{Backfill: BackfillProgressMsg{Active: true, Done: 18204, Total: 36102}}
	if got := renderStatusLine(backfill, th, statusLineTestWidth, true); strings.Contains(ansi.Strip(got), "of 36,102") {
		t.Errorf("renderStatusLine(dropTotal=true) with backfill = %q, want the total dropped", ansi.Strip(got))
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
	if !strings.Contains(queued, "Synced") {
		t.Errorf("renderStatusLine() with Outbox=2 = %q, want the sync state still shown beside it", queued)
	}
}

// TestFitRightSegments_OutboxEvictsBeforeSyncState proves F12: under
// space pressure that the outbox count alone accounts for, the count
// is dropped whole and the sync state renders in full, untruncated
// (decision 7: the count rides beside the sync state, never instead
// of it), the reverse of the render order the two display in.
func TestFitRightSegments_OutboxEvictsBeforeSyncState(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	sl := StatusLine{Sync: SyncStateMsg{State: SyncStateSynced}, Outbox: 2}

	stateSeg := syncStateSeg(th, sl, false)
	stateWidth := ansi.StringWidth(stateSeg.text)
	outboxSeg, ok := outboxSegment(sl)
	if !ok {
		t.Fatal("outboxSegment(sl) reported no outbox segment for a nonzero Outbox")
	}

	// A budget that fits the sync state alone but not the state plus
	// the outbox count together.
	budget := stateWidth
	got := fitRightSegments(th, sl, false, budget)

	if len(got) != 1 || got[0].text != stateSeg.text {
		t.Fatalf("fitRightSegments() at a budget of %d = %+v, want the sync state alone, unmodified: %+v", budget, got, stateSeg)
	}
	if strings.Contains(segsPlainText(got), "queued") {
		t.Errorf("fitRightSegments() = %+v, want the outbox segment %+v evicted, not truncated into the row", got, outboxSeg)
	}
}

// TestRenderStatusLine_NeverExceedsWidth is the property test BACKLOG
// #61 owes: over a wide range of long sync-segment inputs at the 60-
// column floor, the row's display width is exactly 60, never
// more, and the ellipsis token appears whenever the sync state's
// text (F12: the segment that is truncated only as a last resort,
// after the outbox count has already been evicted) would otherwise
// have overflowed on its own.
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
			row := renderStatusLine(sl, th, width, false)
			plain := ansi.Strip(row)
			if got := ansi.StringWidth(plain); got != width {
				t.Fatalf("renderStatusLine(%+v, outbox=%d) display width = %d, want exactly %d\nrow: %q", sync, outbox, got, width, plain)
			}

			leftWidth := ansi.StringWidth(segsPlainText(clusterSegs(sl.Active)))
			budget := width - leftWidth - theme.PadBand
			stateWidth := ansi.StringWidth(syncStateSeg(th, sl, false).text)
			if stateWidth > budget {
				truncated++
				if !strings.Contains(plain, ellipsis) {
					t.Errorf("renderStatusLine(%+v, outbox=%d) sync-state width %d > budget %d with no ellipsis marker: %q", sync, outbox, stateWidth, budget, plain)
				}
			}
		}
	}
	if truncated == 0 {
		t.Fatal("no case in this table actually overflowed the sync state's own budget, so the ellipsis-marking assertion never ran")
	}
}

// TestStatusLineHitSpans_MatchesTheRenderedDigitColumns proves the
// hit spans ADR-0017's character grain requires: each span's Rect
// lands on the exact column the rendered string's digit character
// sits at, and that rune equals GrammarKeys.SurfaceSwitch's bound
// key for that position, never an independently hardcoded '1'-'4'
// that binding could drift from (task-6-findings-r1.md F4).
func TestStatusLineHitSpans_MatchesTheRenderedDigitColumns(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	keys := GrammarKeys.SurfaceSwitch.Keys()

	for active := SurfaceMail; active <= SurfaceConfig; active++ {
		row := ansi.Strip(renderStatusLine(StatusLine{Active: active}, th, 100, false))
		statusRow := image.Rect(0, 5, 100, 6) // an arbitrary non-zero origin, proving translation
		spans := StatusLineHitSpans(active, StateDigitsSwitch, statusRow)

		if len(spans) != 4 {
			t.Fatalf("StatusLineHitSpans(%v) returned %d spans, want 4", active, len(spans))
		}
		for i, span := range spans {
			if span.Target != PointerSurfaceDigit {
				t.Errorf("digit %d: Target = %v, want PointerSurfaceDigit", i+1, span.Target)
			}
			if span.Rect.Min.Y != statusRow.Min.Y || span.Rect.Max.Y != statusRow.Min.Y+1 {
				t.Errorf("digit %d: Rect = %v, want a 1-row span at Y=%d", i+1, span.Rect, statusRow.Min.Y)
			}
			x := span.Rect.Min.X - statusRow.Min.X
			want := rune(keys[i][0])
			if x < 0 || x >= len(row) || rune(row[x]) != want {
				t.Errorf("active=%v digit %d: span X=%d, row column there = %q, want the bound key %q\nrow: %q", active, i+1, x, safeRune(row, x), want, row)
			}
			if !span.Verb.Enabled() || span.Verb.Help().Desc != GrammarKeys.SurfaceSwitch.Help().Desc {
				t.Errorf("digit %d: Verb = %v, want GrammarKeys.SurfaceSwitch", i+1, span.Verb)
			}
		}
	}
}

// TestStatusLineHitSpans_NoneOutsideDigitsSwitch proves F8/RULING: a
// digit click is never offered in a state ADR-0017's table
// (registry.go's pointerLegalStates) does not allow PointerSurfaceDigit
// to fire in, a modal most notably, the same rule the keyboard digits
// themselves already obey (matchDigit, app.go).
func TestStatusLineHitSpans_NoneOutsideDigitsSwitch(t *testing.T) {
	statusRow := image.Rect(0, 0, 100, 1)

	for _, state := range []StateClass{StatePrintableEntry, StateModal} {
		if got := StatusLineHitSpans(SurfaceMail, state, statusRow); got != nil {
			t.Errorf("StatusLineHitSpans(state=%v) = %v, want none", state, got)
		}
	}
	if got := StatusLineHitSpans(SurfaceMail, StateDigitsSwitch, statusRow); len(got) != 4 {
		t.Errorf("StatusLineHitSpans(StateDigitsSwitch) returned %d spans, want 4", len(got))
	}
}

func safeRune(s string, i int) rune {
	if i < 0 || i >= len(s) {
		return 0
	}
	return rune(s[i])
}
