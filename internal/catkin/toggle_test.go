package catkin

import "testing"

func TestToggleList(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		cur     int
		wantSrc string
		wantCur int
	}{
		{"add list prefix", "abc", 1, "- abc", 3},
		{"strip list prefix", "- abc", 4, "abc", 2},
		{"strip from start", "- abc", 0, "abc", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSrc, gotCur := toggleList(tt.src, tt.cur)
			if gotSrc != tt.wantSrc || gotCur != tt.wantCur {
				t.Errorf("toggleList: got (%q, %d), want (%q, %d)", gotSrc, gotCur, tt.wantSrc, tt.wantCur)
			}
		})
	}
}

func TestToggleQuote(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		cur     int
		wantSrc string
		wantCur int
	}{
		{"add quote prefix", "hello", 5, "> hello", 7},
		{"strip quote prefix", "> hello", 4, "hello", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSrc, gotCur := toggleQuote(tt.src, tt.cur)
			if gotSrc != tt.wantSrc || gotCur != tt.wantCur {
				t.Errorf("toggleQuote: got (%q, %d), want (%q, %d)", gotSrc, gotCur, tt.wantSrc, tt.wantCur)
			}
		})
	}
}

func TestToggleTask(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		cur         int
		wantSrc     string
		wantHandled bool
	}{
		{"check unchecked", "- [ ] todo", 7, "- [x] todo", true},
		{"uncheck checked", "- [x] todo", 7, "- [ ] todo", true},
		{"uncheck capital X", "- [X] todo", 7, "- [ ] todo", true},
		{"non-task line ignored", "- plain", 4, "- plain", false},
		{"paragraph ignored", "abc", 1, "abc", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSrc, _, gotHandled := toggleTask(tt.src, tt.cur)
			if gotSrc != tt.wantSrc || gotHandled != tt.wantHandled {
				t.Errorf("toggleTask: got (%q, _, %v), want (%q, _, %v)", gotSrc, gotHandled, tt.wantSrc, tt.wantHandled)
			}
		})
	}
}
