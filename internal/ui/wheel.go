package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// wheelWindow bounds one wheel gesture's flush timer (ADR-0017
// revision 3, machine design section 8): at most one coalesced
// WheelMsg per wheelWindow of continuous scrolling, against QA-2's
// budget of one store round trip per tick otherwise.
const wheelWindow = 16 * time.Millisecond

// WheelMsg is one coalesced wheel gesture: every tick's signed delta
// folded into one running sum, at the coordinates of the gesture's
// first tick, ready for a scrollable pane's Update to apply as a
// single scroll step. App.handleWheel is what produces it; see its
// doc for the coalescing mechanism (ADR-0017 revision 3).
type WheelMsg struct {
	X, Y  int
	Delta int
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
