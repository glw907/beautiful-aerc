package picker

import (
	"regexp"
	"strings"
	"testing"

	"github.com/glw907/beautiful-aerc/internal/filter"
)

var reANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestFormatLine(t *testing.T) {
	colors := &Colors{
		Number:   "\033[38;2;129;161;193m",
		Label:    "\033[38;2;229;233;240m",
		URL:      "\033[38;2;97;110;136m",
		Selected: "\033[48;2;57;67;83m\033[38;2;229;233;240m",
		Reset:    "\033[0m",
	}

	t.Run("unselected shows number label and url", func(t *testing.T) {
		link := filter.FootnoteLink{Label: "Click here", URL: "https://example.com"}
		got := FormatLine(1, link, false, 80, 15, colors)
		clean := reANSI.ReplaceAllString(got, "")
		if !strings.Contains(clean, "1") {
			t.Errorf("missing number: %q", clean)
		}
		if !strings.Contains(clean, "Click here") {
			t.Errorf("missing label: %q", clean)
		}
		if !strings.Contains(clean, "https://example.com") {
			t.Errorf("missing URL: %q", clean)
		}
	})

	t.Run("selected has selection color", func(t *testing.T) {
		link := filter.FootnoteLink{Label: "Click", URL: "https://example.com"}
		got := FormatLine(1, link, true, 80, 15, colors)
		if !strings.Contains(got, colors.Selected) {
			t.Errorf("missing selected color: %q", got)
		}
	})

	t.Run("long URL truncated", func(t *testing.T) {
		link := filter.FootnoteLink{Label: "Go", URL: "https://example.com/very/long/path/that/exceeds/width"}
		got := FormatLine(1, link, false, 40, 10, colors)
		clean := reANSI.ReplaceAllString(got, "")
		if strings.Contains(clean, "exceeds") {
			t.Errorf("URL should be truncated: %q", clean)
		}
		if !strings.Contains(clean, "…") {
			t.Errorf("truncated URL should have ellipsis: %q", clean)
		}
	})

	t.Run("long label truncated", func(t *testing.T) {
		link := filter.FootnoteLink{Label: "This is a very long label text", URL: "https://example.com"}
		got := FormatLine(1, link, false, 80, 15, colors)
		clean := reANSI.ReplaceAllString(got, "")
		if strings.Contains(clean, "very long label text") {
			t.Errorf("label should be truncated: %q", clean)
		}
		if !strings.Contains(clean, "…") {
			t.Errorf("truncated label should have ellipsis: %q", clean)
		}
	})

	t.Run("number 10 shows 0", func(t *testing.T) {
		link := filter.FootnoteLink{Label: "Link", URL: "https://example.com"}
		got := FormatLine(10, link, false, 80, 15, colors)
		clean := reANSI.ReplaceAllString(got, "")
		if !strings.Contains(clean, "0") {
			t.Errorf("10th item should show 0: %q", clean)
		}
	})

	t.Run("number beyond 10 shows space", func(t *testing.T) {
		link := filter.FootnoteLink{Label: "Link", URL: "https://example.com"}
		got := FormatLine(11, link, false, 80, 15, colors)
		clean := reANSI.ReplaceAllString(got, "")
		if len(clean) >= 2 && clean[1] != ' ' {
			t.Errorf("items beyond 10 should show space shortcut: %q", clean)
		}
	})
}
