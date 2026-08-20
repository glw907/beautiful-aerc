package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestApp_SyncStateMsgUpdatesStatusLine proves App absorbs
// SyncStateMsg into its own model state, so a.statusLine() (App.View's
// own input to Render) reflects it.
func TestApp_SyncStateMsgUpdatesStatusLine(t *testing.T) {
	app := NewApp(testDeps(t))

	msg := SyncStateMsg{State: SyncStateSyncing, Done: 4, Total: 10}
	updated, _ := app.Update(msg)
	app = mustApp(t, updated)

	if got := app.statusLine().Sync; got != msg {
		t.Errorf("statusLine().Sync = %+v, want %+v", got, msg)
	}
}

// TestApp_BackfillProgressMsgUpdatesStatusLine mirrors the sync-state
// case for backfill progress.
func TestApp_BackfillProgressMsgUpdatesStatusLine(t *testing.T) {
	app := NewApp(testDeps(t))

	msg := BackfillProgressMsg{Active: true, Done: 1, Total: 2}
	updated, _ := app.Update(msg)
	app = mustApp(t, updated)

	if got := app.statusLine().Backfill; got != msg {
		t.Errorf("statusLine().Backfill = %+v, want %+v", got, msg)
	}
}

// TestApp_OutboxCountMsgUpdatesStatusLine mirrors the sync-state case
// for the queued-outbox count.
func TestApp_OutboxCountMsgUpdatesStatusLine(t *testing.T) {
	app := NewApp(testDeps(t))

	updated, _ := app.Update(OutboxCountMsg{Queued: 3})
	app = mustApp(t, updated)

	if got := app.statusLine().Outbox; got != 3 {
		t.Errorf("statusLine().Outbox = %d, want 3", got)
	}
}

// TestApp_StoreChangedMsgReloadsThePlaceholders proves StoreChangedMsg
// reaches the mail and calendar placeholders' own update methods (via
// App's default updateChildren fallthrough) and re-issues their
// store-count load, rather than leaving a stale count on the screen
// once the bridge reports a write.
func TestApp_StoreChangedMsgReloadsThePlaceholders(t *testing.T) {
	app := NewApp(testDeps(t))

	updated, cmd := app.Update(StoreChangedMsg{})
	mustApp(t, updated)
	if cmd == nil {
		t.Fatal("Update(StoreChangedMsg{}) returned a nil Cmd, want the placeholders' own reload")
	}

	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("Update(StoreChangedMsg{})'s Cmd yielded %#v, want a tea.BatchMsg of every child's own Cmd", msg)
	}

	sawMailReload, sawCalendarReload := false, false
	for _, sub := range batch {
		if sub == nil {
			continue
		}
		switch sub().(type) {
		case mailStatsMsg:
			sawMailReload = true
		case eventCountMsg:
			sawCalendarReload = true
		}
	}
	if !sawMailReload {
		t.Error("no mailStatsMsg among the batched reload Cmds, want the mail placeholder to reload")
	}
	if !sawCalendarReload {
		t.Error("no eventCountMsg among the batched reload Cmds, want the calendar placeholder to reload")
	}
}

// TestApp_SpinnerTicksOnlyWhileSyncingOrBackfilling is QA-8's idle
// posture: entering the syncing state arms a spinner tick; leaving it
// stops the chain rather than rearming, and the one already-in-flight
// tick from before the transition is a harmless no-op once it lands.
func TestApp_SpinnerTicksOnlyWhileSyncingOrBackfilling(t *testing.T) {
	app := NewApp(testDeps(t))

	updated, cmd := app.Update(SyncStateMsg{State: SyncStateSynced})
	app = mustApp(t, updated)
	if cmd != nil {
		t.Fatal("SyncStateMsg{Synced} armed a Cmd, want none: QA-8 forbids a timer wakeup while quiescent")
	}

	updated, cmd = app.Update(SyncStateMsg{State: SyncStateSyncing, Done: 1, Total: 10})
	app = mustApp(t, updated)
	if cmd == nil {
		t.Fatal("SyncStateMsg{Syncing} armed no Cmd, want the spinner tick to start")
	}
	tickMsg, ok := cmd().(statusSpinnerTickMsg)
	if !ok {
		t.Fatalf("the armed Cmd yielded %#v, want statusSpinnerTickMsg", tickMsg)
	}
	gen := tickMsg.gen

	// A second progress message while still syncing must not replace
	// the pending tick: reconcileSpinner only arms on the false-to-true
	// edge, never on every update a run in progress delivers.
	updated, cmd = app.Update(SyncStateMsg{State: SyncStateSyncing, Done: 2, Total: 10})
	app = mustApp(t, updated)
	if cmd != nil {
		t.Fatal("a second Syncing update armed a new Cmd, want the existing tick chain left alone")
	}

	updated, cmd = app.tickSpinner(statusSpinnerTickMsg{gen: gen})
	app = mustApp(t, updated)
	if cmd == nil {
		t.Fatal("tickSpinner while still syncing armed no Cmd, want the chain to continue")
	}
	if app.statusLine().Spinner != 1 {
		t.Errorf("Spinner = %d, want 1 after one tick", app.statusLine().Spinner)
	}

	// Leaving the syncing state stops the chain: the in-flight tick
	// (still carrying gen) is now stale by App's own spinnerTicking
	// flag, so it must not rearm.
	updated, cmd = app.Update(SyncStateMsg{State: SyncStateSynced})
	app = mustApp(t, updated)
	if cmd != nil {
		t.Fatal("leaving Syncing armed a Cmd, want none")
	}

	updated, cmd = app.tickSpinner(statusSpinnerTickMsg{gen: gen})
	app = mustApp(t, updated)
	if cmd != nil {
		t.Fatal("tickSpinner after leaving the syncing state armed a Cmd, want QA-8's idle posture: no further timer wakeups")
	}

	// Restart: spinnerTicking is true again, on a fresh gen. A tick
	// carrying the OLD (pre-stop) gen must still be ignored here: this
	// is what the gen guard alone catches, and what makes the guard
	// falsifiable (task-6-findings-r1.md F13). spinnerTicking alone
	// would wrongly accept it, since it is true again by this point.
	staleGen := gen
	updated, cmd = app.Update(SyncStateMsg{State: SyncStateSyncing, Done: 3, Total: 10})
	app = mustApp(t, updated)
	if cmd == nil {
		t.Fatal("restarting Syncing armed no Cmd, want a fresh tick chain")
	}
	restarted, ok := cmd().(statusSpinnerTickMsg)
	if !ok {
		t.Fatalf("the restarted Cmd yielded %#v, want statusSpinnerTickMsg", restarted)
	}
	if restarted.gen == staleGen {
		t.Fatal("restarting Syncing reused the old gen, want a fresh one so a stale tick from before the stop is distinguishable")
	}

	updated, cmd = app.tickSpinner(statusSpinnerTickMsg{gen: staleGen})
	app = mustApp(t, updated)
	if cmd != nil {
		t.Fatal("a stale tick from before the restart armed a Cmd, want it ignored despite spinnerTicking being true again")
	}
	if app.statusLine().Spinner != 1 {
		t.Errorf("a stale tick from before the restart advanced Spinner to %d, want it unchanged at 1", app.statusLine().Spinner)
	}

	updated, cmd = app.tickSpinner(statusSpinnerTickMsg{gen: restarted.gen})
	app = mustApp(t, updated)
	if cmd == nil {
		t.Fatal("the current gen's tick armed no Cmd, want the chain to continue")
	}
	if app.statusLine().Spinner != 2 {
		t.Errorf("Spinner = %d, want 2 after the current gen's tick", app.statusLine().Spinner)
	}
}
