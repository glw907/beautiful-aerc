package catkin

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
)

func TestBufferRuneOffsetRoundTrip(t *testing.T) {
	b := NewBuffer(newEmptyTextarea())
	b.SetValue("hello\nworld\nfoo")
	b.SetRuneOffset(7)
	if got := b.RuneOffset(); got != 7 {
		t.Errorf("RuneOffset round-trip: got %d, want 7", got)
	}
}

func TestBufferRuneOffsetClampsPastEnd(t *testing.T) {
	b := NewBuffer(newEmptyTextarea())
	b.SetValue("abc")
	b.SetRuneOffset(100)
	if got := b.RuneOffset(); got > 3 {
		t.Errorf("RuneOffset past-end: got %d, want ≤3", got)
	}
}

func newEmptyTextarea() textarea.Model {
	t := textarea.New()
	t.SetWidth(40)
	t.SetHeight(10)
	return t
}
