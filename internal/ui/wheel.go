package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// wheelWindow is the wheel filter's sample window (ADR-0017, machine
// design section 8): an unfiltered burst is one store round trip per
// physical detent against QA-2's budget, and terminals disagree about
// ticks per detent.
const wheelWindow = 16 * time.Millisecond

// WheelMsg is the wheel filter's coalesced output: every tick folded
// into one running signed delta, ready for a scrollable pane's Update
// to apply as a single scroll step.
type WheelMsg struct {
	X, Y  int
	Delta int
}

// newWheelFilter returns the tea.WithFilter callback that coalesces a
// burst of tea.MouseWheelMsg ticks into WheelMsg (ADR-0017; the one
// recorded elm-conventions Rule 2 exception: pure, holds no model
// state). Each tick folds its signed unit delta into a running sum
// while the filter suppresses it (returns nil); the sum flushes as
// one WheelMsg the moment a further tick arrives wheelWindow or more
// after the sum opened, or reverses the sum's direction. Either way
// the flushing tick itself opens the next sum rather than joining the
// one it closes, so a burst collapses to exactly one message instead
// of one store round trip per physical detent.
//
// now stands in for time.Now so a test can drive the window without a
// real sleep; program construction passes time.Now itself.
func newWheelFilter(now func() time.Time) func(tea.Model, tea.Msg) tea.Msg {
	var opened time.Time
	var sum int

	return func(_ tea.Model, msg tea.Msg) tea.Msg {
		wheel, ok := msg.(tea.MouseWheelMsg)
		if !ok {
			return msg
		}
		mouse := wheel.Mouse()
		delta := wheelDelta(mouse.Button)
		if delta == 0 {
			return msg
		}

		t := now()
		if sum != 0 && (t.Sub(opened) >= wheelWindow || wheelSign(delta) != wheelSign(sum)) {
			out := WheelMsg{X: mouse.X, Y: mouse.Y, Delta: sum}
			opened, sum = t, delta
			return out
		}
		if sum == 0 {
			opened = t
		}
		sum += delta
		return nil
	}
}

// wheelDelta converts a wheel tick's button into its signed unit
// scroll delta: up moves content up (negative), down moves it down
// (positive). Left/right ticks poplar does not scroll on yield zero.
func wheelDelta(b tea.MouseButton) int {
	switch b {
	case tea.MouseWheelUp:
		return -1
	case tea.MouseWheelDown:
		return 1
	default:
		return 0
	}
}

func wheelSign(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}
