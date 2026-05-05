package catkin

import "testing"

func TestWordBoundaryForward(t *testing.T) {
	got := nextWordBoundary("the quick brown", 0)
	if got != 4 {
		t.Errorf("nextWordBoundary(0): got %d, want 4", got)
	}
	got = nextWordBoundary("the quick brown", 5)
	if got != 10 {
		t.Errorf("nextWordBoundary(5): got %d, want 10", got)
	}
	got = nextWordBoundary("the quick brown", 15)
	if got != 15 {
		t.Errorf("nextWordBoundary(end): got %d, want 15", got)
	}
	got = nextWordBoundary("the quick brown", 3)
	if got != 4 {
		t.Errorf("nextWordBoundary(3): got %d, want 4", got)
	}
}

func TestWordBoundaryBackward(t *testing.T) {
	got := prevWordBoundary("the quick brown", 15)
	if got != 10 {
		t.Errorf("prevWordBoundary(15): got %d, want 10", got)
	}
	got = prevWordBoundary("the quick brown", 10)
	if got != 4 {
		t.Errorf("prevWordBoundary(10): got %d, want 4", got)
	}
	got = prevWordBoundary("the quick brown", 0)
	if got != 0 {
		t.Errorf("prevWordBoundary(0): got %d, want 0", got)
	}
	got = prevWordBoundary("the quick brown", 9)
	if got != 4 {
		t.Errorf("prevWordBoundary(9): got %d, want 4", got)
	}
}
