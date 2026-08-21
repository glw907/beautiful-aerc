package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

// TestRenderStatusLine_ToastRidesTheRightSegment proves design
// decision 1: a showing toast renders its label and undo hint in
// the status line's right segment, and the sync state compresses to
// its bare word beside it at widthStandardMin and up.
func TestRenderStatusLine_ToastRidesTheRightSegment(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	sl := StatusLine{
		Sync:  SyncStateMsg{State: SyncStateSyncing, Done: 4312, Total: 36102},
		Toast: Toast{Active: true, Label: "3 messages archived", Remaining: 9, Undoable: true},
	}

	got := plainStatusLine(sl, th)
	if !strings.Contains(got, "3 messages archived") {
		t.Errorf("renderStatusLine() = %q, want the toast's label", got)
	}
	if !strings.Contains(got, "u undo") {
		t.Errorf("renderStatusLine() = %q, want the undo hint, derived from GrammarKeys.Undo", got)
	}
	if !strings.Contains(got, "9s") {
		t.Errorf("renderStatusLine() = %q, want the visible countdown", got)
	}
	if !strings.Contains(got, "Syncing") || strings.Contains(got, "4,312") {
		t.Errorf("renderStatusLine() = %q, want the sync state compressed to its bare word, no progress count", got)
	}
}

// TestRenderStatusLine_ToastYieldsSyncBelowStandardWidth proves the
// dispatch's ruling: below widthStandardMin the sync state yields
// entirely rather than compressing, so the toast alone survives.
func TestRenderStatusLine_ToastYieldsSyncBelowStandardWidth(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	sl := StatusLine{
		Sync:  SyncStateMsg{State: SyncStateSyncing, Done: 1, Total: 2},
		Toast: Toast{Active: true, Label: "archived", Remaining: 9},
	}

	wide := ansi.Strip(renderStatusLine(sl, th, widthStandardMin, false))
	if !strings.Contains(wide, "Syncing") {
		t.Errorf("renderStatusLine(%d) = %q, want the sync state's bare word at widthStandardMin", widthStandardMin, wide)
	}

	narrow := ansi.Strip(renderStatusLine(sl, th, widthStandardMin-1, false))
	if strings.Contains(narrow, "Syncing") {
		t.Errorf("renderStatusLine(%d) = %q, want the sync state to yield entirely below widthStandardMin", widthStandardMin-1, narrow)
	}
	if !strings.Contains(narrow, "archived") {
		t.Errorf("renderStatusLine(%d) = %q, want the toast itself to survive", widthStandardMin-1, narrow)
	}
}

// TestRenderStatusLine_ToastReservesGapAndCountdownSurvives is F1,
// CRITICAL (task-8-findings-r1.md): at 60 columns with a label too
// long to fit whole, the gap between the cluster and the toast's
// content is never squeezed to zero (the cluster must not run into
// the label), and truncation falls on the label: the countdown
// survives.
func TestRenderStatusLine_ToastReservesGapAndCountdownSurvives(t *testing.T) {
	th := theme.New(true, theme.ProfileTrueColor)
	sl := StatusLine{
		Toast: Toast{Active: true, Label: strings.Repeat("a very long archived-message toast label ", 3), Remaining: 9, Undoable: true},
	}

	const width = 60
	plain := ansi.Strip(renderStatusLine(sl, th, width, false))
	if got := ansi.StringWidth(plain); got != width {
		t.Fatalf("renderStatusLine(%d) display width = %d, want %d", width, got, width)
	}
	if !strings.Contains(plain, "9s") {
		t.Errorf("renderStatusLine(%d) = %q, want the countdown to survive", width, plain)
	}

	leftWidth := ansi.StringWidth(segsPlainText(clusterSegs(sl.Active)))
	budget := max(0, width-leftWidth-theme.PadBand)
	rightSegs := rightSegments(th, sl, false, width, budget)
	rightWidth := ansi.StringWidth(segsPlainText(rightSegs))
	if gap := width - leftWidth - rightWidth - theme.PadBand; gap <= 0 {
		t.Errorf("gap between the cluster and the toast content = %d, want > 0", gap)
	}
}

// TestBareSyncWord_DropsProgressForEveryState proves every SY-5 state
// compresses to its bare label, with no count, spinner, or retry
// text riding along.
func TestBareSyncWord_DropsProgressForEveryState(t *testing.T) {
	tests := []struct {
		sync SyncStateMsg
		want string
	}{
		{SyncStateMsg{State: SyncStateSynced}, "Synced"},
		{SyncStateMsg{State: SyncStateSyncing, Done: 1, Total: 2}, "Syncing"},
		{SyncStateMsg{State: SyncStateOffline}, "Offline"},
		{SyncStateMsg{State: SyncStateBackingOff, Retry: 12}, "Backing off"},
	}
	for _, tt := range tests {
		seg := bareSyncWord(StatusLine{Sync: tt.sync})
		if seg.text != tt.want {
			t.Errorf("bareSyncWord(%+v).text = %q, want %q", tt.sync, seg.text, tt.want)
		}
	}
}

// TestApp_ToastMsg_NewestWins proves design decision 1: a second
// ToastMsg while one is still showing replaces it outright, and bumps
// the countdown's gen so a tick from the replaced window is
// recognizable as stale.
func TestApp_ToastMsg_NewestWins(t *testing.T) {
	app := NewApp(testDeps(t))

	app = mustApp(t, first(app.Update(ToastMsg{Offer: UndoOffer{Label: "3 messages archived"}})))
	firstGen := app.toastGen

	updated, cmd := app.Update(ToastMsg{Offer: UndoOffer{Label: "1 message deleted"}})
	app = mustApp(t, updated)

	if app.statusLine().Toast.Label != "1 message deleted" {
		t.Errorf("Toast.Label = %q after a second ToastMsg, want the newest offer's label", app.statusLine().Toast.Label)
	}
	if app.toastGen == firstGen {
		t.Error("toastGen unchanged across a second ToastMsg, want a fresh gen so the replaced window's tick is stale")
	}
	if cmd == nil {
		t.Fatal("ToastMsg armed no Cmd, want the countdown's tick chain to start")
	}
}

// TestApp_ToastCountdown_TicksNineToZero is survey amendment E's
// discipline: the countdown's visible sequence, driven by injected
// toastTickMsg values at the message layer, never a real timer.
func TestApp_ToastCountdown_TicksNineToZero(t *testing.T) {
	app := NewApp(testDeps(t))

	updated, cmd := app.Update(ToastMsg{Offer: UndoOffer{Label: "archived"}})
	app = mustApp(t, updated)
	if app.statusLine().Toast.Remaining != 9 {
		t.Fatalf("Remaining after ToastMsg = %d, want 9 (the pinned exemplar's dim \"9s\")", app.statusLine().Toast.Remaining)
	}
	if cmd == nil {
		t.Fatal("ToastMsg armed no Cmd, want the countdown's tick chain to start")
	}
	// gen comes from App's field, never from invoking the armed
	// Cmd (a real tea.Tick(time.Second, ...) that would block this
	// test for a second): survey amendment E asserts at the message
	// layer only.
	gen := app.toastGen

	for want := 8; want >= 0; want-- {
		updated, cmd = app.tickToast(toastTickMsg{gen: gen})
		app = mustApp(t, updated)
		if got := app.statusLine().Toast.Remaining; got != want {
			t.Fatalf("Remaining = %d, want %d", got, want)
		}
		if !app.statusLine().Toast.Active {
			t.Fatalf("Toast.Active = false at Remaining=%d, want it still showing", want)
		}
		if want > 0 && cmd == nil {
			t.Fatalf("tickToast at Remaining=%d armed no Cmd, want the chain to continue", want)
		}
	}

	// The tick past zero closes the window.
	updated, cmd = app.tickToast(toastTickMsg{gen: gen})
	app = mustApp(t, updated)
	if app.statusLine().Toast.Active {
		t.Error("Toast.Active still true after the countdown's final tick, want the window closed")
	}
	if cmd != nil {
		t.Error("tickToast after closing the window armed a Cmd, want none")
	}
}

// TestApp_ToastTick_StaleGenIgnored mirrors
// TestApp_SpinnerTicksOnlyWhileSyncingOrBackfilling's stale-gen
// case: a tick from a window a newer toast already replaced changes
// nothing.
func TestApp_ToastTick_StaleGenIgnored(t *testing.T) {
	app := NewApp(testDeps(t))

	app = mustApp(t, first(app.Update(ToastMsg{Offer: UndoOffer{Label: "first"}})))
	staleGen := app.toastGen

	app = mustApp(t, first(app.Update(ToastMsg{Offer: UndoOffer{Label: "second"}})))

	updated, cmd := app.tickToast(toastTickMsg{gen: staleGen})
	app = mustApp(t, updated)
	if cmd != nil {
		t.Error("a stale-gen tick armed a Cmd, want it ignored")
	}
	if app.statusLine().Toast.Remaining != 9 || app.statusLine().Toast.Label != "second" {
		t.Errorf("a stale-gen tick changed the current toast: %+v", app.statusLine().Toast)
	}
}

// TestApp_UndoWithinWindowEmitsOfferCmd proves `u` inside the
// countdown window emits the offer's Undo Cmd and closes the
// window.
func TestApp_UndoWithinWindowEmitsOfferCmd(t *testing.T) {
	app := NewApp(testDeps(t))

	fired := false
	offer := UndoOffer{Label: "archived", Undo: func() tea.Msg { fired = true; return nil }}
	app = mustApp(t, first(app.Update(ToastMsg{Offer: offer})))

	updated, cmd := app.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	app = mustApp(t, updated)

	if cmd == nil {
		t.Fatal("'u' within the window returned a nil Cmd, want the offer's Undo")
	}
	cmd()
	if !fired {
		t.Error("the returned Cmd did not invoke the offer's Undo")
	}
	if app.statusLine().Toast.Active {
		t.Error("Toast.Active still true after 'u' answered it, want the window closed")
	}
}

// TestApp_UndoAfterExpiryIsNoop proves `u` after the window closes
// (Toast.Active already false) is a no-op: no Cmd, and the offer is
// never invoked.
func TestApp_UndoAfterExpiryIsNoop(t *testing.T) {
	app := NewApp(testDeps(t))

	fired := false
	offer := UndoOffer{Label: "archived", Undo: func() tea.Msg { fired = true; return nil }}
	updated, _ := app.Update(ToastMsg{Offer: offer})
	app = mustApp(t, updated)
	gen := app.toastGen

	for range undoWindowSeconds {
		updated, _ = app.tickToast(toastTickMsg{gen: gen})
		app = mustApp(t, updated)
	}
	if app.statusLine().Toast.Active {
		t.Fatal("Toast.Active still true after the whole window ticked out")
	}

	_, cmd := app.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	if cmd != nil {
		t.Error("'u' after expiry returned a non-nil Cmd, want a no-op")
	}
	if fired {
		t.Error("'u' after expiry invoked the expired offer's Undo")
	}
}

// TestApp_QuitWithOpenUndoWindowConfirms proves UX-9's lifecycle
// rule, carried from task 8's review to task 11: the window does not
// survive quit, and the toast says so. q at a surface root with an
// open undo window and an empty outbox pushes F7's modal confirm
// naming the offer rather than quitting straight through; 'n' leaves
// the window open and invokes nothing; 'y' logs one debug line,
// discards the window without invoking its Undo, and quits.
func TestApp_QuitWithOpenUndoWindowConfirms(t *testing.T) {
	app := NewApp(testDeps(t))

	fired := false
	offer := UndoOffer{Label: "archived", Undo: func() tea.Msg { fired = true; return nil }}
	app = mustApp(t, first(app.Update(ToastMsg{Offer: offer})))

	updated, cmd := app.Update(digitKey("q"))
	app = mustApp(t, updated)
	if cmd != nil {
		t.Fatal("q with an open undo window returned a non-nil Cmd, want it to push a confirm instead of quitting")
	}
	if len(app.stack) != 1 {
		t.Fatalf("stack length after q with an open undo window = %d, want 1", len(app.stack))
	}
	confirm, ok := app.stack[0].(Confirm)
	if !ok {
		t.Fatalf("stack top is %T, want Confirm", app.stack[0])
	}
	if !strings.Contains(confirm.Consequence, "archived") {
		t.Errorf("Consequence = %q, want it naming the open offer's label", confirm.Consequence)
	}

	// 'n' (or Esc) leaves the window open and fires nothing.
	stayed, cmd := answerConfirm(t, app, tea.KeyPressMsg{Code: 'n', Text: "n"})
	if len(stayed.stack) != 0 {
		t.Fatalf("stack length after 'n' = %d, want 0 (the modal pops on answer)", len(stayed.stack))
	}
	if cmd != nil {
		cmd()
	}
	if !stayed.statusLine().Toast.Active {
		t.Error("Toast.Active false after 'n', want the window still open")
	}
	if fired {
		t.Error("'n' invoked the open offer's Undo")
	}

	// 'y' emits quitYesMsg; App's case for it (not Confirm's YesCmd
	// itself) discards the window, logs once, and quits, so the answer
	// is evaluated fresh rather than snapshotted back when q first
	// pushed the modal.
	buf := captureDebugLog(t)
	answered, cmd := answerConfirm(t, app, tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("'y' returned a nil Cmd, want quitYesMsg")
	}
	msg := cmd()
	if _, ok := msg.(quitYesMsg); !ok {
		t.Fatalf("'y' yielded %#v, want quitYesMsg", msg)
	}

	updated, quitCmd := answered.Update(msg)
	answered = mustApp(t, updated)
	if quitCmd == nil {
		t.Fatal("quitYesMsg handling returned a nil Cmd, want tea.Quit")
	}
	if got := quitCmd(); got != (tea.QuitMsg{}) {
		t.Errorf("quitYesMsg handling yielded %#v, want tea.QuitMsg", got)
	}
	if fired {
		t.Error("'y' invoked the open offer's Undo, want it merely discarded")
	}
	if !strings.Contains(buf.String(), "quit discarded the open undo window") {
		t.Errorf("log = %q, want the one discard line", buf.String())
	}
	if answered.statusLine().Toast.Active {
		t.Error("Toast.Active still true after quitYesMsg, want the window discarded")
	}
}

// TestApp_QuitWithNotificationToastQuitsStraightThrough proves M3: a
// plain notification toast (an offer with no Undo Cmd, Toast.Undoable
// false) does not gate q the way an undo window does. q quits
// straight through with no confirm, since there is no undo to lose.
func TestApp_QuitWithNotificationToastQuitsStraightThrough(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(ToastMsg{Offer: UndoOffer{Label: "synced"}})))

	if app.statusLine().Toast.Undoable {
		t.Fatal("a notification offer with no Undo reported Undoable, want false (test setup)")
	}

	_, cmd := app.Update(digitKey("q"))
	if cmd == nil {
		t.Fatal("q with a notification toast returned a nil Cmd, want tea.Quit")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Errorf("q with a notification toast yielded %#v, want tea.QuitMsg", msg)
	}
}

// TestApp_QuitYesMsg_NoPhantomDiscardAfterTheWindowExpires proves the
// other half of m8: an undo window that expires while the quit
// confirm sits open (toastTickMsg keeps ticking regardless of what is
// on the stack) logs no discard line when y is finally answered,
// since nothing was actually open by then.
func TestApp_QuitYesMsg_NoPhantomDiscardAfterTheWindowExpires(t *testing.T) {
	app := NewApp(testDeps(t))
	offer := UndoOffer{Label: "archived", Undo: func() tea.Msg { return nil }}
	app = mustApp(t, first(app.Update(ToastMsg{Offer: offer})))

	app = mustApp(t, first(app.Update(digitKey("q"))))
	if len(app.stack) != 1 {
		t.Fatalf("stack length after q with an open undo window = %d, want 1", len(app.stack))
	}

	// Expire the window while the modal is still on the stack, exactly
	// as a real countdown does: toastTickMsg is a top-level App.Update
	// case, unconditional on what is showing.
	for range undoWindowSeconds {
		app = mustApp(t, first(app.Update(toastTickMsg{gen: app.toastGen})))
	}
	if app.statusLine().Toast.Active {
		t.Fatal("toast still active after undoWindowSeconds ticks, want it expired (test setup)")
	}

	buf := captureDebugLog(t)
	updated, quitCmd := app.Update(quitYesMsg{})
	_ = mustApp(t, updated)
	if quitCmd == nil {
		t.Fatal("quitYesMsg handling returned a nil Cmd, want tea.Quit")
	}
	if got := quitCmd(); got != (tea.QuitMsg{}) {
		t.Errorf("quitYesMsg handling yielded %#v, want tea.QuitMsg", got)
	}
	if buf.String() != "" {
		t.Errorf("log = %q, want no discard line: the window had already expired", buf.String())
	}
}

// TestApp_QuitWithQueuedOutboxConfirms proves the outbox half of
// CARRY 3: a nonzero outbox count at quit pushes F7's modal
// confirm ("Quit with N unsent messages?"), even with no undo window
// open.
func TestApp_QuitWithQueuedOutboxConfirms(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(OutboxCountMsg{Queued: 2})))

	updated, cmd := app.Update(digitKey("q"))
	app = mustApp(t, updated)
	if cmd != nil {
		t.Fatal("q with a queued outbox returned a non-nil Cmd, want it to push a confirm instead of quitting")
	}
	if len(app.stack) != 1 {
		t.Fatalf("stack length after q with a queued outbox = %d, want 1", len(app.stack))
	}
	confirm, ok := app.stack[0].(Confirm)
	if !ok {
		t.Fatalf("stack top is %T, want Confirm", app.stack[0])
	}
	if confirm.Question != "Quit with 2 unsent messages?" {
		t.Errorf("Question = %q, want it naming the queued count", confirm.Question)
	}
	if !strings.Contains(confirm.Consequence, "next time you open poplar") {
		t.Errorf("Consequence = %q, want the outbox's consequence line", confirm.Consequence)
	}
}

// TestApp_ToastLogsOneLine is ER-1's seam: every toast App
// absorbs produces exactly one log line, carrying the offer's label.
func TestApp_ToastLogsOneLine(t *testing.T) {
	buf := captureDebugLog(t)
	app := NewApp(testDeps(t))

	_, _ = app.Update(ToastMsg{Offer: UndoOffer{Label: "3 messages archived"}})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("log lines = %d, want exactly 1:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "3 messages archived") {
		t.Errorf("log line = %q, want it to carry the toast's label", lines[0])
	}
}

// TestApp_UndoEligible_GatesOnStackAndSwitchState is F2, CRITICAL
// (task-8-findings-r1.md): both probe cases against the extracted
// gate directly, a front outside StateDigitsSwitch and a non-empty
// stack, each independently disqualify `u` from answering the open
// undo window, even though an offer is open.
func TestApp_UndoEligible_GatesOnStackAndSwitchState(t *testing.T) {
	app := NewApp(testDeps(t))
	app.toastActive = true
	app.toastOffer = UndoOffer{Undo: func() tea.Msg { return nil }}

	digitsFront := ScreenEntry{SwitchState: StateDigitsSwitch}
	if !app.undoEligible(digitsFront) {
		t.Error("undoEligible() at a surface root, StateDigitsSwitch front, open toast = false, want true")
	}

	entryFront := ScreenEntry{SwitchState: StatePrintableEntry}
	if app.undoEligible(entryFront) {
		t.Error("undoEligible() with a StatePrintableEntry front = true, want false (F2 probe: the front gate)")
	}

	app.stack = append(app.stack, &fakeModal{})
	if app.undoEligible(digitsFront) {
		t.Error("undoEligible() with a non-empty stack = true, want false (F2 probe: the stack gate)")
	}
}

// TestApp_Undo_NoOpWithModalOnStack is F2's end-to-end case: `u`
// with an open undo window but a modal on the stack reaches the
// modal's Update (a no-op, since 'u' answers none of Confirm's
// own y/n/Esc grammar) rather than the open offer's Undo.
func TestApp_Undo_NoOpWithModalOnStack(t *testing.T) {
	app := NewApp(testDeps(t))

	fired := false
	offer := UndoOffer{Label: "archived", Undo: func() tea.Msg { fired = true; return nil }}
	app = mustApp(t, first(app.Update(ToastMsg{Offer: offer})))
	app.stack = append(app.stack, testConfirm(t, nil, nil))

	updated, cmd := app.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	app = mustApp(t, updated)
	if cmd != nil {
		t.Error("'u' with a modal on the stack returned a non-nil Cmd, want a no-op")
	}
	if fired {
		t.Error("'u' with a modal on the stack fired the open offer's Undo, want the gate to block it")
	}
	if !app.toastActive {
		t.Error("toastActive cleared by 'u' with a modal on the stack, want the window left open")
	}
}

// TestApp_ToastTickMsg_ReachesUpdate is F12 (task-8-findings-r1.md):
// at least one test drives toastTickMsg through App.Update itself,
// not App.tickToast directly, so deleting Update's switch case
// fails the suite.
func TestApp_ToastTickMsg_ReachesUpdate(t *testing.T) {
	app := NewApp(testDeps(t))
	app = mustApp(t, first(app.Update(ToastMsg{Offer: UndoOffer{Label: "archived"}})))
	gen := app.toastGen

	updated, _ := app.Update(toastTickMsg{gen: gen})
	app = mustApp(t, updated)

	if got := app.statusLine().Toast.Remaining; got != 8 {
		t.Errorf("Remaining after one toastTickMsg through App.Update = %d, want 8", got)
	}
}
