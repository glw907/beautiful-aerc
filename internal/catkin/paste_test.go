package catkin

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func newModelWithValue(val string, off int) Model {
	m := New()
	m.buf = m.buf.WithValue(val).WithRuneOffset(off)
	m.undo.seed(snap{val, off})
	return m
}

func TestPaste(t *testing.T) {
	tests := []struct {
		name       string
		initial    string
		cursor     int
		payload    string
		wantVal    string
		wantCursor int
	}{
		{
			name:       "empty payload no-op",
			initial:    "abc",
			cursor:     0,
			payload:    "",
			wantVal:    "abc",
			wantCursor: 0,
		},
		{
			name:       "plain paste at start",
			initial:    "abc",
			cursor:     0,
			payload:    "xy",
			wantVal:    "xyabc",
			wantCursor: 2,
		},
		{
			name:       "plain paste at end",
			initial:    "abc",
			cursor:     3,
			payload:    "xy",
			wantVal:    "abcxy",
			wantCursor: 5,
		},
		{
			name:       "plain paste mid-word non-url",
			initial:    "abc",
			cursor:     1,
			payload:    "X",
			wantVal:    "aXbc",
			wantCursor: 2,
		},
		{
			name:       "url wrap mid-word",
			initial:    "see foobar here",
			cursor:     7,
			payload:    "https://example.com",
			wantVal:    "see [foobar](https://example.com) here",
			wantCursor: 33,
		},
		{
			name:       "url paste outside word inserts literally",
			initial:    "see  here",
			cursor:     4,
			payload:    "https://example.com",
			wantVal:    "see https://example.com here",
			wantCursor: 23,
		},
		{
			name:       "url wrap at end of word",
			initial:    "see foo",
			cursor:     7,
			payload:    "https://example.com",
			wantVal:    "see [foo](https://example.com)",
			wantCursor: 30,
		},
		{
			name:       "url with internal whitespace inserts literally",
			initial:    "see foobar here",
			cursor:     7,
			payload:    "https://example.com extra",
			wantVal:    "see foohttps://example.com extrabar here",
			wantCursor: 32,
		},
		{
			name:       "mailto wrap",
			initial:    "Hi",
			cursor:     1,
			payload:    "mailto:a@b.c",
			wantVal:    "[Hi](mailto:a@b.c)",
			wantCursor: 18,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newModelWithValue(tt.initial, tt.cursor)
			m2, _ := m.Update(tea.PasteMsg{Content: tt.payload})
			if got := m2.Value(); got != tt.wantVal {
				t.Errorf("value: got %q, want %q", got, tt.wantVal)
			}
			if got := m2.buf.RuneOffset(); got != tt.wantCursor {
				t.Errorf("cursor: got %d, want %d", got, tt.wantCursor)
			}
		})
	}
}

func TestPaste_SingleUndo(t *testing.T) {
	m := newModelWithValue("hello world", 5)
	preCursor := m.buf.RuneOffset()
	preVal := m.Value()

	m, _ = m.Update(tea.PasteMsg{Content: " inserted"})
	if m.Value() == preVal {
		t.Fatal("paste had no effect")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
	if got := m.Value(); got != preVal {
		t.Errorf("after undo: value = %q, want %q", got, preVal)
	}
	if got := m.buf.RuneOffset(); got != preCursor {
		t.Errorf("after undo: cursor = %d, want %d", got, preCursor)
	}
}
