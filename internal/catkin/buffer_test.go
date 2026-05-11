package catkin

import (
	"testing"

	"charm.land/bubbles/v2/textarea"
)

func TestBufferRuneOffsetRoundTrip(t *testing.T) {
	b := NewBuffer(newEmptyTextarea()).WithValue("hello\nworld\nfoo").WithRuneOffset(7)
	if got := b.RuneOffset(); got != 7 {
		t.Errorf("RuneOffset round-trip: got %d, want 7", got)
	}
}

func TestBufferRuneOffsetClampsPastEnd(t *testing.T) {
	b := NewBuffer(newEmptyTextarea()).WithValue("abc").WithRuneOffset(100)
	if got := b.RuneOffset(); got > 3 {
		t.Errorf("RuneOffset past-end: got %d, want ≤3", got)
	}
}

func TestBufferRuneOffsetAtNewline(t *testing.T) {
	b := NewBuffer(newEmptyTextarea()).WithValue("hello\nworld\nfoo").WithRuneOffset(5)
	if got := b.RuneOffset(); got != 5 {
		t.Errorf("RuneOffset at newline boundary: got %d, want 5", got)
	}
}

func newEmptyTextarea() textarea.Model {
	t := textarea.New()
	t.SetWidth(40)
	t.SetHeight(10)
	return t
}
