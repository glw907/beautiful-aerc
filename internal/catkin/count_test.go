package catkin

import "testing"

func TestWordCount(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"", 0},
		{"   ", 0},
		{"hello", 1},
		{"hello world", 2},
		{"  hello\tworld\nfoo  ", 3},
	}
	for _, tt := range tests {
		if got := wordCount(tt.s); got != tt.want {
			t.Errorf("wordCount(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}

func TestModelCounts(t *testing.T) {
	m := New()
	m.SetValue("hello world")
	if got := m.WordCount(); got != 2 {
		t.Errorf("WordCount = %d, want 2", got)
	}
	if got := m.CharCount(); got != 11 {
		t.Errorf("CharCount = %d, want 11", got)
	}
}
