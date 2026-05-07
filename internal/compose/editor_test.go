package compose

import "testing"

func TestCatkinEditorImplementsEditor(t *testing.T) {
	var _ Editor = NewCatkinEditor()
}

func TestCatkinEditorValueRoundTrip(t *testing.T) {
	e := NewCatkinEditor()
	const want = "hello\nworld"
	e.SetValue(want)
	if got := e.Value(); got != want {
		t.Fatalf("Value() = %q, want %q", got, want)
	}
}

func TestCatkinEditorSetSizeNoPanic(t *testing.T) {
	e := NewCatkinEditor()
	e.SetSize(80, 24)
	e.SetWidth(72)
}
