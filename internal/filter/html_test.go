package filter

import (
	"strings"
	"testing"
)

func TestCleanPandocArtifacts(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trailing backslash", "hello\\\n", "hello\n"},
		{"escaped punctuation", "hello\\!", "hello!"},
		{"escaped period", "end\\.", "end."},
		{"no change", "normal text", "normal text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanPandocArtifacts(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanImages(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"image link", "[![alt](img.png)](https://example.com)", "[alt](https://example.com)"},
		{"standalone image", "![logo](logo.png)\n", ""},
		{"empty text link", "[](https://example.com)\n", ""},
		{"empty url link", "[click here]()", "click here"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanImages(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"nbsp", "hello\u00a0world", "hello world"},
		{"zero-width chars", "he\u200cllo\u200bwor\uFEFFld", "helloworld"},
		{"trailing spaces on blank line", "hello\n   \nworld", "hello\n\nworld"},
		{"excessive blank lines", "hello\n\n\n\nworld", "hello\n\nworld"},
		{"leading blank lines", "\n\n\nhello", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeWhitespace(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHighlightMarkdown(t *testing.T) {
	colors := &markdownColors{
		Heading: "1;32",
		Bold:    "1",
		Italic:  "3",
		Rule:    "2",
		Reset:   "0",
	}
	tests := []struct {
		name  string
		input string
		check func(string) bool
		desc  string
	}{
		{
			"heading",
			"## Hello World",
			func(s string) bool { return strings.Contains(s, "\033[1;32m") && strings.Contains(s, "Hello World") },
			"should contain heading color + text",
		},
		{
			"bold",
			"this is **bold** text",
			func(s string) bool { return strings.Contains(s, "\033[1m") && strings.Contains(s, "bold") },
			"should contain bold ANSI + text",
		},
		{
			"italic",
			"this is *italic* text",
			func(s string) bool { return strings.Contains(s, "\033[3m") && strings.Contains(s, "italic") },
			"should contain italic ANSI + text",
		},
		{
			"horizontal rule dashes",
			"---",
			func(s string) bool { return strings.Contains(s, "\033[2m") },
			"should contain rule color",
		},
		{
			"horizontal rule underscores",
			"___",
			func(s string) bool { return strings.Contains(s, "\033[2m") },
			"should contain rule color",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := highlightMarkdown(tt.input, colors)
			if !tt.check(got) {
				t.Errorf("%s: got %q", tt.desc, got)
			}
		})
	}
}

func TestStyleLinks(t *testing.T) {
	colors := &linkColors{
		Text:  "38;2;136;192;208",
		URL:   "38;2;97;110;136",
		Reset: "0",
	}
	tests := []struct {
		name      string
		input     string
		clean     bool
		checkText string
	}{
		{
			"markdown mode",
			"[Click here](https://example.com)",
			false,
			"\033[38;2;136;192;208m[Click here]\033[0m\033[38;2;97;110;136m(https://example.com)\033[0m",
		},
		{
			"clean mode",
			"[Click here](https://example.com)",
			true,
			"Click here",
		},
		{
			"strip leading/trailing spaces in text",
			"[ Click here ](https://example.com)",
			false,
			"[Click here]",
		},
		{
			"strip colorize ANSI from URL",
			"[Click](\033[4;33mhttps://example.com\033[0m)",
			false,
			"(https://example.com)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := styleLinks(tt.input, colors, tt.clean)
			if !strings.Contains(got, tt.checkText) {
				t.Errorf("output %q does not contain %q", got, tt.checkText)
			}
		})
	}
}
