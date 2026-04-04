package filter

import "testing"

func TestDetectHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"plain text", "Hello world\nThis is a test", false},
		{"has div", "<div>Hello</div>", true},
		{"has html tag", "<html><body>test</body></html>", true},
		{"has br", "line one<br>line two", true},
		{"has table", "<table><tr><td>cell</td></tr></table>", true},
		{"has span", "text <span>styled</span> text", true},
		{"has p tag", "<p>paragraph</p>", true},
		{"angle bracket in text", "x < y and y > z", false},
		{"html deep in file", "line1\nline2\n" + repeatString("normal\n", 50) + "<div>late html</div>", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectHTML(tt.input)
			if got != tt.want {
				t.Errorf("detectHTML() = %v, want %v", got, tt.want)
			}
		})
	}
}

func repeatString(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
