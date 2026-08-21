package theme

import (
	"image"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// plainLines strips every ANSI escape from s and splits it into
// lines, so a test can assert on plain-text cell content without
// caring how a style rendered it.
func plainLines(s string) []string {
	stripped := stripANSI(s)
	return strings.Split(stripped, "\n")
}

// stripANSI removes every CSI/SGR escape sequence from s: a small
// local equivalent of ansi.Strip, kept here rather than importing
// the ansi package into a theme package test that has no other use
// for it.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case inEscape:
			if r == 'm' {
				inEscape = false
			}
		case r == '\x1b':
			inEscape = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TestCanvasPaintOrigin proves Paint positions content at rect's
// origin: a canvas larger than the painted block leaves every other
// cell untouched (blank).
func TestCanvasPaintOrigin(t *testing.T) {
	c := NewCanvas(6, 3)
	c.Paint(image.Rect(2, 1, 5, 2), "XYZ")

	lines := plainLines(c.Render())
	if len(lines) < 2 {
		t.Fatalf("Render() has %d lines, want at least 2", len(lines))
	}
	if got := lines[1]; !strings.HasPrefix(got, "  XYZ") {
		t.Errorf("row 1 = %q, want \"XYZ\" starting at column 2", got)
	}
	if got := lines[0]; strings.Contains(got, "XYZ") {
		t.Errorf("row 0 = %q, want no content painted at row 1's origin", got)
	}
}

// TestCanvasPaintOverwriteOrder proves a later Paint call draws over
// an earlier one at the same position, the mechanism Render relies
// on to overlay a named pane's content on top of Main's blank
// fill.
func TestCanvasPaintOverwriteOrder(t *testing.T) {
	c := NewCanvas(4, 1)
	c.Paint(image.Rect(0, 0, 4, 1), "aaaa")
	c.Paint(image.Rect(1, 0, 3, 1), "bb")

	got := plainLines(c.Render())[0]
	if want := "abba"; got != want {
		t.Errorf("Render() row 0 = %q, want %q (the second Paint call overwriting the first's middle two cells)", got, want)
	}
}

// TestCanvasRenderPreservesStyle proves Render carries a Paint call's
// styling through: a background painted via lipgloss survives
// into the composed output.
func TestCanvasRenderPreservesStyle(t *testing.T) {
	c := NewCanvas(3, 1)
	styled := lipgloss.NewStyle().Background(hexColor("FF0000")).Render("abc")
	c.Paint(image.Rect(0, 0, 3, 1), styled)

	got := c.Render()
	if !strings.Contains(got, "255;0;0") && !strings.Contains(got, "FF0000") {
		t.Errorf("Render() = %q, want the painted background's color code to survive composition", got)
	}
}
