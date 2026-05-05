package catkin

import "testing"

func TestBracketMatchAt(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		col   int
		want  int
		found bool
	}{
		{"single star open", "*foo*", 0, 4, true},
		{"single star close", "*foo*", 4, 0, true},
		{"double star open inner", "**bold**", 1, 7, true},
		{"double star close outer", "**bold**", 7, 1, true},
		{"double star open outer", "**bold**", 0, 6, true},
		{"backtick", "`code`", 0, 5, true},
		{"link bracket", "[text](url)", 0, 5, true},
		{"link bracket close", "[text](url)", 5, 0, true},
		{"plain text", "hello", 2, 0, false},
		{"out of range", "*foo*", 99, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := bracketMatchAt(tt.line, tt.col)
			if ok != tt.found || (ok && got != tt.want) {
				t.Fatalf("got (%d, %v) want (%d, %v)", got, ok, tt.want, tt.found)
			}
		})
	}
}
