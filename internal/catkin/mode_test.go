package catkin

import "testing"

func TestDisplayModeCycle(t *testing.T) {
	want := []DisplayMode{
		ModeTypewriter,
		ModeFocus,
		ModeFocusTypewriter,
		ModeNormal,
	}
	got := ModeNormal
	for i, w := range want {
		got = got.next()
		if got != w {
			t.Fatalf("step %d: got %d want %d", i, got, w)
		}
	}
}

func TestDisplayModeFlags(t *testing.T) {
	tests := []struct {
		mode       DisplayMode
		typewriter bool
		focus      bool
	}{
		{ModeNormal, false, false},
		{ModeTypewriter, true, false},
		{ModeFocus, false, true},
		{ModeFocusTypewriter, true, true},
	}
	for _, tt := range tests {
		if tt.mode.typewriter() != tt.typewriter || tt.mode.focus() != tt.focus {
			t.Fatalf("%v: got (tw=%v focus=%v) want (tw=%v focus=%v)", tt.mode, tt.mode.typewriter(), tt.mode.focus(), tt.typewriter, tt.focus)
		}
	}
}

func TestActiveParagraphRange(t *testing.T) {
	src := "para one\nstill one\n\npara two\n\npara three"
	ctxs := Classify(splitLines(src))
	first, last := activeParagraphRange(ctxs, 1)
	if first != 0 || last != 1 {
		t.Errorf("para 1: got [%d, %d] want [0, 1]", first, last)
	}
	first, last = activeParagraphRange(ctxs, 3)
	if first != 3 || last != 3 {
		t.Errorf("para 2: got [%d, %d] want [3, 3]", first, last)
	}
}

func TestClampViewportTypewriter(t *testing.T) {
	// height 10, cursor at line 50, total 100 → top should be 45.
	if got := clampViewportTypewriter(10, 50, 100); got != 45 {
		t.Errorf("got %d want 45", got)
	}
	// near top: cursor 2, height 10 → top clamped to 0.
	if got := clampViewportTypewriter(10, 2, 100); got != 0 {
		t.Errorf("near top: got %d want 0", got)
	}
	// near bottom: cursor 98, height 10, total 100 → top = 90.
	if got := clampViewportTypewriter(10, 98, 100); got != 90 {
		t.Errorf("near bottom: got %d want 90", got)
	}
}

func splitLines(s string) []string {
	out := []string{""}
	for _, r := range s {
		if r == '\n' {
			out = append(out, "")
			continue
		}
		out[len(out)-1] += string(r)
	}
	return out
}
