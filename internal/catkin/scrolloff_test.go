package catkin

import (
	"strings"
	"testing"
)

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

func TestCursorVisualRowQuotedParagraph(t *testing.T) {
	src := "\n> alpha beta gamma delta epsilon\n"
	lines := strings.Split(src, "\n")
	ctxs := Classify(lines)
	cursorRow := 1
	cursorCol := len([]rune(lines[cursorRow]))
	vr, total := cursorVisualRow(lines, ctxs, 14, cursorRow, cursorCol)
	if total < 4 {
		t.Errorf("expected total visual rows ≥4, got %d", total)
	}
	if vr < 2 {
		t.Errorf("cursor at end of wrapped quoted line should sit on the last visual row of the wrap (≥2); got %d", vr)
	}
}

func TestCursorVisualRowPlainTotalMatchesSourceLineCount(t *testing.T) {
	src := "one\ntwo\nthree"
	lines := strings.Split(src, "\n")
	ctxs := Classify(lines)
	_, total := cursorVisualRow(lines, ctxs, 80, 0, 0)
	if total != 3 {
		t.Errorf("plain short lines: expected total=3, got %d", total)
	}
}

func TestApplyScrollOffKeepsCursorVisibleInLongQuote(t *testing.T) {
	src := "padding\n> alpha beta gamma delta epsilon zeta eta theta iota\npadding"
	m := New().WithValue(src).WithSize(16, 4)
	target := len("padding\n") + len("> alpha beta gamma delta epsilon zeta eta theta iota")
	m.buf = m.buf.WithRuneOffset(target)
	m = applyScrollOff(m)
	lines := strings.Split(src, "\n")
	ctxs := Classify(lines)
	cursorRow, cursorCol := offsetToRowCol(src, target)
	cvr, _ := cursorVisualRow(lines, ctxs, 16, cursorRow, cursorCol)
	if cvr < m.viewportTop || cvr >= m.viewportTop+m.height {
		t.Errorf("cursor visual row %d outside viewport [%d, %d)", cvr, m.viewportTop, m.viewportTop+m.height)
	}
}
