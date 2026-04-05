package picker

import (
	"strings"
	"testing"
)

func TestExtractURLs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"single URL", "Visit https://example.com today", []string{"https://example.com"}},
		{"multiple URLs", "See https://a.com and https://b.com/path", []string{"https://a.com", "https://b.com/path"}},
		{"deduplicates", "https://a.com then https://a.com again", []string{"https://a.com"}},
		{"strips trailing punctuation", "Visit https://example.com.", []string{"https://example.com"}},
		{"http and https", "http://old.com and https://new.com", []string{"http://old.com", "https://new.com"}},
		{"no URLs", "just plain text", nil},
		{"URL with query params", "Go to https://example.com/page?foo=bar&baz=1", []string{"https://example.com/page?foo=bar&baz=1"}},
		{"strips ANSI codes from URLs", "Visit \033[38;2;97;110;136mhttps://example.com\033[0m today", []string{"https://example.com"}},
		{"extracts full URL from OSC 8 hyperlink", "\033]8;;https://example.com/very/long/path\033\\https://example.com/ver…\033]8;;\033\\", []string{"https://example.com/very/long/path"}},
		{"OSC 8 and plain URLs deduped", "see https://example.com and \033]8;;https://example.com\033\\example\033]8;;\033\\", []string{"https://example.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractURLs(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("count: got %d, want %d\ngot: %v", len(got), len(tt.want), got)
				return
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("[%d]: got %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

func TestFormatLine(t *testing.T) {
	colors := &Colors{
		Number:   "\033[38;2;129;161;193m",
		URL:      "\033[38;2;97;110;136m",
		Selected: "\033[48;2;57;67;83m\033[38;2;229;233;240m",
		Reset:    "\033[0m",
	}

	t.Run("unselected", func(t *testing.T) {
		got := FormatLine(1, "https://example.com", false, colors)
		if !strings.Contains(got, "1") || !strings.Contains(got, "https://example.com") {
			t.Errorf("missing number or URL: %q", got)
		}
	})

	t.Run("selected", func(t *testing.T) {
		got := FormatLine(1, "https://example.com", true, colors)
		if !strings.Contains(got, colors.Selected) {
			t.Errorf("missing selected color: %q", got)
		}
	})

	t.Run("number 10 shows 0", func(t *testing.T) {
		got := FormatLine(10, "https://example.com", false, colors)
		if !strings.Contains(got, "0") {
			t.Errorf("10th item should show 0: %q", got)
		}
	})

	t.Run("number beyond 10 shows space", func(t *testing.T) {
		got := FormatLine(11, "https://example.com", false, colors)
		// Shortcut field is the second rune after the leading space; must be a space, not "11".
		// Strip ANSI codes first so we're checking rendered text only.
		clean := reANSI.ReplaceAllString(got, "")
		if len(clean) >= 2 && clean[1] != ' ' {
			t.Errorf("items beyond 10 should show space shortcut: %q (clean: %q)", got, clean)
		}
	})
}
