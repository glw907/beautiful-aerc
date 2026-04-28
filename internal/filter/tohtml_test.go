// SPDX-License-Identifier: MIT

package filter

import (
	"strings"
	"testing"
)

func TestToHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "paragraph",
			input:    "Hello world",
			contains: "<p>Hello world</p>",
		},
		{
			name:     "bold",
			input:    "**Important**",
			contains: "<strong>Important</strong>",
		},
		{
			name:     "heading",
			input:    "## Title",
			contains: "<h2>Title</h2>",
		},
		{
			name:     "link",
			input:    "[Click](https://example.com)",
			contains: `<a href="https://example.com">Click</a>`,
		},
		{
			name:     "html document wrapper",
			input:    "Hello",
			contains: "<!DOCTYPE html>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			err := ToHTML(strings.NewReader(tt.input), &out)
			if err != nil {
				t.Fatalf("ToHTML returned error: %v", err)
			}
			got := out.String()
			if !strings.Contains(got, tt.contains) {
				t.Errorf("output does not contain %q\ngot:\n%s", tt.contains, got)
			}
		})
	}
}
