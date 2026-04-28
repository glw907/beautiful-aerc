// SPDX-License-Identifier: MIT

package filter

import (
	"strings"
	"testing"
)

func TestCleanPlainStripsCarriageReturn(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"crlf line endings", "line1\r\nline2\r\n"},
		{"standalone cr", "a\rb"},
		{"mixed crlf and lf", "first\r\nsecond\nthird\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanPlain(tt.input)
			if strings.Contains(got, "\r") {
				t.Errorf("CleanPlain() result contains \\r: %q", got)
			}
		})
	}
}

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
		{"html deep in file", "line1\nline2\n" + strings.Repeat("normal\n", 50) + "<div>late html</div>", false},
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
