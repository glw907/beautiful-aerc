package catkin

import "testing"

func TestLooksLikeURL(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"https://example.com", true},
		{"http://x", true},
		{"mailto:a@b", true},
		{"  https://x  ", true},
		{"hello world", false},
		{"https://x with space", false},
		{"ftp://x", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := looksLikeURL(tt.in); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestWordAt(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		cur        int
		ok         bool
		start, end int
	}{
		{"middle of word", "hello world", 3, true, 0, 5},
		{"end of word", "hello world", 5, true, 0, 5},
		{"on space", "hello world", 6, true, 6, 11},
		{"in space gap", "a  b", 2, false, 0, 0},
		{"empty buffer", "", 0, false, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, e, ok := wordAt(tt.src, tt.cur)
			if ok != tt.ok || s != tt.start || e != tt.end {
				t.Fatalf("got (%d,%d,%v) want (%d,%d,%v)", s, e, ok, tt.start, tt.end, tt.ok)
			}
		})
	}
}
