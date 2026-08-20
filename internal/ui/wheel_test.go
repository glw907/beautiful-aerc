package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// wheelTick builds a tea.MouseWheelMsg for the given button, at the
// given cell.
func wheelTick(button tea.MouseButton, x, y int) tea.MouseWheelMsg {
	return tea.MouseWheelMsg{Button: button, X: x, Y: y}
}

func TestWheelDelta(t *testing.T) {
	tests := []struct {
		button tea.MouseButton
		want   int
	}{
		{tea.MouseWheelUp, -1},
		{tea.MouseWheelDown, 1},
		{tea.MouseWheelLeft, 0},
		{tea.MouseWheelRight, 0},
	}
	for _, tt := range tests {
		if got := wheelDelta(tt.button); got != tt.want {
			t.Errorf("wheelDelta(%v) = %d, want %d", tt.button, got, tt.want)
		}
	}
}

func TestWheelSign(t *testing.T) {
	if wheelSign(-3) != -1 {
		t.Error("wheelSign(-3) != -1")
	}
	if wheelSign(3) != 1 {
		t.Error("wheelSign(3) != 1")
	}
	if wheelSign(0) != 1 {
		t.Error("wheelSign(0) != 1 (zero has no accumulated direction yet)")
	}
}

// TestApp_WheelSingleDetentFlushesAfterWindow proves ADR-0017
// revision 3's core guarantee over the filter design it replaces: an
// isolated tick, with no follow-up tick ever arriving, still reaches
// Update as one WheelMsg once its flush timer fires, at the tick's
// own coordinates.
func TestApp_WheelSingleDetentFlushesAfterWindow(t *testing.T) {
	app := NewApp(testDeps(t))

	updated, cmd := app.Update(wheelTick(tea.MouseWheelDown, 5, 7))
	app = mustApp(t, updated)
	if !app.wheel.open {
		t.Fatal("a single tick did not open a gesture")
	}
	if cmd == nil {
		t.Fatal("opening a gesture returned a nil Cmd, want the armed flush timer")
	}

	flush, ok := cmd().(wheelFlushMsg)
	if !ok {
		t.Fatalf("the armed timer yielded %#v, want wheelFlushMsg", flush)
	}

	updated, cmd = app.Update(flush)
	app = mustApp(t, updated)
	if app.wheel.open {
		t.Error("the flush timer left the gesture open")
	}
	if cmd == nil {
		t.Fatal("the flush timer's Cmd was nil, want the flushed WheelMsg")
	}
	msg, ok := cmd().(WheelMsg)
	if !ok {
		t.Fatalf("flush Cmd yielded %#v, want WheelMsg", msg)
	}
	if msg.Delta != 1 {
		t.Errorf("WheelMsg.Delta = %d, want 1 (one detent)", msg.Delta)
	}
	if msg.X != 5 || msg.Y != 7 {
		t.Errorf("WheelMsg coords = (%d,%d), want the gesture's first tick (5,7)", msg.X, msg.Y)
	}
}

// TestApp_WheelBurstCollapsesToOne proves a same-direction burst,
// including one that outlasts wheelWindow, still reaches Update as
// exactly one WheelMsg: every tick after the first accumulates
// silently onto the gesture the first tick opened, and the same
// flush timer that gesture armed at open time is what eventually
// delivers the whole sum.
func TestApp_WheelBurstCollapsesToOne(t *testing.T) {
	app := NewApp(testDeps(t))

	var flushTimer tea.Cmd
	for i := range 30 {
		updated, cmd := app.Update(wheelTick(tea.MouseWheelDown, 1, 1))
		app = mustApp(t, updated)
		if i == 0 {
			flushTimer = cmd
			continue
		}
		if cmd != nil {
			t.Fatalf("tick %d returned a non-nil Cmd, want nil (accumulating)", i)
		}
	}
	if app.wheel.sum != 30 {
		t.Fatalf("gesture sum = %d, want 30", app.wheel.sum)
	}

	flush, ok := flushTimer().(wheelFlushMsg)
	if !ok {
		t.Fatalf("flush timer yielded %#v, want wheelFlushMsg", flush)
	}
	updated, flushCmd := app.Update(flush)
	app = mustApp(t, updated)
	if flushCmd == nil {
		t.Fatal("flushing the gesture returned a nil Cmd")
	}
	msg, ok := flushCmd().(WheelMsg)
	if !ok {
		t.Fatalf("flush Cmd yielded %#v, want WheelMsg", msg)
	}
	if msg.Delta != 30 {
		t.Errorf("WheelMsg.Delta = %d, want 30 (the burst's summed delta)", msg.Delta)
	}
}

// TestApp_WheelDirectionFlipFlushesImmediately proves a
// direction-reversing tick flushes the open gesture right away,
// unmixed with the flip tick's own delta, and opens a fresh gesture
// carrying just the flip tick.
func TestApp_WheelDirectionFlipFlushesImmediately(t *testing.T) {
	app := NewApp(testDeps(t))

	for i := range 5 {
		updated, cmd := app.Update(wheelTick(tea.MouseWheelUp, 2, 3))
		app = mustApp(t, updated)
		if i > 0 && cmd != nil {
			t.Fatalf("tick %d returned a non-nil Cmd, want nil (accumulating)", i)
		}
	}
	if app.wheel.sum != -5 {
		t.Fatalf("gesture sum = %d, want -5", app.wheel.sum)
	}

	updated, cmd := app.Update(wheelTick(tea.MouseWheelDown, 9, 9))
	app = mustApp(t, updated)
	if cmd == nil {
		t.Fatal("a direction flip returned a nil Cmd, want an immediate flush plus the new gesture's flush timer")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("direction-flip Cmd yielded %#v, want a two-command batch", batch)
	}
	msg, ok := batch[0]().(WheelMsg)
	if !ok {
		t.Fatalf("the flip's first batched Cmd yielded %#v, want the flushed WheelMsg", msg)
	}
	if msg.Delta != -5 {
		t.Errorf("flushed WheelMsg.Delta = %d, want -5 (the up-sum, unmixed with the flip tick)", msg.Delta)
	}
	if !app.wheel.open || app.wheel.sum != 1 {
		t.Errorf("gesture after the flip = %+v, want an open gesture summing 1 (just the flip tick)", app.wheel)
	}
}

// TestApp_WheelStaleFlushTimerIgnored proves a flush timer armed for
// a gesture a direction flip already closed is recognized as stale
// (by generation) and ignored, rather than prematurely flushing or
// corrupting the gesture that replaced it.
func TestApp_WheelStaleFlushTimerIgnored(t *testing.T) {
	app := NewApp(testDeps(t))

	updated, staleTimer := app.Update(wheelTick(tea.MouseWheelUp, 1, 1))
	app = mustApp(t, updated)

	updated, _ = app.Update(wheelTick(tea.MouseWheelDown, 2, 2)) // flips, opens a new gesture
	app = mustApp(t, updated)
	if app.wheel.gen != 2 {
		t.Fatalf("gen after the flip = %d, want 2", app.wheel.gen)
	}

	stale, ok := staleTimer().(wheelFlushMsg)
	if !ok {
		t.Fatalf("stale timer yielded %#v, want wheelFlushMsg", stale)
	}
	updated, cmd := app.Update(stale)
	app = mustApp(t, updated)
	if cmd != nil {
		t.Error("a stale flush timer produced a non-nil Cmd, want it ignored")
	}
	if !app.wheel.open || app.wheel.gen != 2 || app.wheel.sum != 1 {
		t.Errorf("gesture after a stale flush = %+v, want the live gen-2 gesture untouched", app.wheel)
	}
}
