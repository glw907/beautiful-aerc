package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// manualClock is newWheelFilter's now func, driven by hand so a test
// can place each tick at an exact instant without a real sleep.
type manualClock struct {
	t time.Time
}

func (c *manualClock) now() time.Time { return c.t }

func wheelTick(button tea.MouseButton) tea.Msg {
	return tea.MouseWheelMsg{Button: button}
}

// TestWheelFilter_BurstInsideWindowReachesUpdateAsOneMessage proves
// the coalescing half of ADR-0017's filter: 30 same-direction ticks
// arriving at the same instant all suppress (return nil), and a 31st
// tick arriving wheelWindow later flushes the 30-tick sum as exactly
// one WheelMsg.
func TestWheelFilter_BurstInsideWindowReachesUpdateAsOneMessage(t *testing.T) {
	clock := &manualClock{t: time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)}
	filter := newWheelFilter(clock.now)

	for i := range 30 {
		if got := filter(nil, wheelTick(tea.MouseWheelDown)); got != nil {
			t.Fatalf("tick %d of the burst returned %#v, want nil (suppressed)", i, got)
		}
	}

	clock.t = clock.t.Add(wheelWindow)
	got := filter(nil, wheelTick(tea.MouseWheelDown))
	msg, ok := got.(WheelMsg)
	if !ok {
		t.Fatalf("flushing tick returned %#v, want a WheelMsg", got)
	}
	if msg.Delta != 30 {
		t.Errorf("flushing WheelMsg.Delta = %d, want 30 (the burst's summed delta)", msg.Delta)
	}
}

// TestWheelFilter_DirectionFlipResets proves the direction-reset half
// of ADR-0017's filter: an opposite-direction tick flushes the
// pending sum immediately, without netting the new tick into it, and
// the new tick opens its own sum rather than joining the old one.
func TestWheelFilter_DirectionFlipResets(t *testing.T) {
	clock := &manualClock{t: time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)}
	filter := newWheelFilter(clock.now)

	for i := range 5 {
		if got := filter(nil, wheelTick(tea.MouseWheelUp)); got != nil {
			t.Fatalf("up-tick %d returned %#v, want nil (suppressed)", i, got)
		}
	}

	// Still inside the window, but the opposite direction: the flip
	// must flush the up-sum now rather than waiting out the window.
	got := filter(nil, wheelTick(tea.MouseWheelDown))
	msg, ok := got.(WheelMsg)
	if !ok {
		t.Fatalf("direction-flip tick returned %#v, want a WheelMsg", got)
	}
	if msg.Delta != -5 {
		t.Errorf("flushed WheelMsg.Delta = %d, want -5 (the up-sum, unmixed with the flip tick)", msg.Delta)
	}

	// The flip tick opened a fresh sum of its own; one more down-tick
	// accumulates onto it rather than the discarded up-sum.
	if got := filter(nil, wheelTick(tea.MouseWheelDown)); got != nil {
		t.Fatalf("tick after the flip returned %#v, want nil (suppressed, accumulating the new sum)", got)
	}
	clock.t = clock.t.Add(wheelWindow)
	got = filter(nil, wheelTick(tea.MouseWheelDown))
	msg, ok = got.(WheelMsg)
	if !ok {
		t.Fatalf("final flush returned %#v, want a WheelMsg", got)
	}
	if msg.Delta != 2 {
		t.Errorf("final flushed WheelMsg.Delta = %d, want 2 (the flip tick plus the one after it)", msg.Delta)
	}
}

// TestWheelFilter_PassesNonWheelMessagesUnchanged proves the filter
// only intercepts tea.MouseWheelMsg: any other message reaches Update
// untouched.
func TestWheelFilter_PassesNonWheelMessagesUnchanged(t *testing.T) {
	clock := &manualClock{t: time.Now()}
	filter := newWheelFilter(clock.now)

	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	if got := filter(nil, msg); got != msg {
		t.Errorf("filter(WindowSizeMsg) = %#v, want it passed through unchanged", got)
	}
}
