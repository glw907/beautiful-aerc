package catkin

import "testing"

func TestClampViewportNoChange(t *testing.T) {
	got := ClampViewport(5, 20, 10, 30)
	if got != 5 {
		t.Errorf("ClampViewport stable: got %d, want 5", got)
	}
}

func TestClampViewportScrollsDown(t *testing.T) {
	got := ClampViewport(0, 20, 18, 100)
	if got != 2 {
		t.Errorf("ClampViewport scroll-down: got %d, want 2", got)
	}
}

func TestClampViewportScrollsUp(t *testing.T) {
	got := ClampViewport(10, 20, 5, 100)
	if got != 2 {
		t.Errorf("ClampViewport scroll-up: got %d, want 2", got)
	}
}

func TestClampViewportRespectsTotal(t *testing.T) {
	got := ClampViewport(5, 20, 5, 10)
	if got != 0 {
		t.Errorf("ClampViewport short doc: got %d, want 0", got)
	}
}
