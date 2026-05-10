package catkin

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Visible runes after the cursor on the cursor row must not duplicate.
func TestAdvCursorRowAnnotationVisibleChars(t *testing.T) {
	src := "abcdef"
	anns := []Annotation{{Range: Range{3, 6}, Style: lipgloss.NewStyle().Underline(true)}}
	set := newAnnotationSet(src, anns)
	out := RenderAnnotated(src, 80, 1, 0, 2, Styles{}, ModeNormal, set)
	plain := strings.TrimRight(ansi.Strip(out), " ")
	want := "ab█def"
	if plain != want {
		t.Errorf("plain = %q, want %q", plain, want)
	}
}

// An annotation spanning the cursor must not erase the cursor block.
func TestAdvAnnotationSpanningCursor(t *testing.T) {
	src := "hello world"
	anns := []Annotation{{Range: Range{0, 5}, Style: lipgloss.NewStyle().Underline(true)}}
	set := newAnnotationSet(src, anns)
	out := RenderAnnotated(src, 80, 1, 0, 3, Styles{}, ModeNormal, set)
	if !strings.Contains(out, "█") {
		t.Errorf("cursor block erased by spanning annotation: %q", ansi.Strip(out))
	}
	plain := strings.TrimRight(ansi.Strip(out), " ")
	want := "hel█o world"
	if plain != want {
		t.Errorf("plain = %q, want %q", plain, want)
	}
}

// Annotation abutting the cursor on the left.
func TestAdvAnnotationAbutsLeftOfCursor(t *testing.T) {
	src := "abcdefg"
	anns := []Annotation{{Range: Range{0, 4}, Style: lipgloss.NewStyle().Underline(true)}}
	set := newAnnotationSet(src, anns)
	out := RenderAnnotated(src, 80, 1, 0, 4, Styles{}, ModeNormal, set)
	plain := strings.TrimRight(ansi.Strip(out), " ")
	want := "abcd█fg"
	if plain != want {
		t.Errorf("plain = %q, want %q", plain, want)
	}
}

// Annotation abutting the cursor on the right.
func TestAdvAnnotationAbutsRightOfCursor(t *testing.T) {
	src := "abcdefg"
	anns := []Annotation{{Range: Range{5, 7}, Style: lipgloss.NewStyle().Underline(true)}}
	set := newAnnotationSet(src, anns)
	out := RenderAnnotated(src, 80, 1, 0, 4, Styles{}, ModeNormal, set)
	plain := strings.TrimRight(ansi.Strip(out), " ")
	want := "abcd█fg"
	if plain != want {
		t.Errorf("plain = %q, want %q", plain, want)
	}
}

// Two annotations, cursor between.
func TestAdvTwoAnnotationsCursorBetween(t *testing.T) {
	src := "abcdefgh"
	anns := []Annotation{
		{Range: Range{0, 3}, Style: lipgloss.NewStyle().Underline(true)},
		{Range: Range{5, 8}, Style: lipgloss.NewStyle().Underline(true)},
	}
	set := newAnnotationSet(src, anns)
	out := RenderAnnotated(src, 80, 1, 0, 4, Styles{}, ModeNormal, set)
	plain := strings.TrimRight(ansi.Strip(out), " ")
	want := "abcd█fgh"
	if plain != want {
		t.Errorf("plain = %q, want %q", plain, want)
	}
}

// Multi-byte runes mid-line. Annotation past the multi-byte char.
func TestAdvAnnotationAfterMultibyte(t *testing.T) {
	src := "café tradeof"
	// "café " = c(1) a(1) f(1) é(2 bytes) ' '(1) = 6 bytes, 5 cells.
	// "tradeof" starts at byte 6, ends at 13.
	anns := []Annotation{{Range: Range{6, 13}, Style: lipgloss.NewStyle().Underline(true)}}
	set := newAnnotationSet(src, anns)
	out := RenderAnnotated(src, 80, 1, 0, 0, Styles{}, ModeNormal, set)
	plain := strings.TrimRight(ansi.Strip(out), " ")
	// cursor at col 0 replaces 'c' with █.
	want := "█afé tradeof"
	if plain != want {
		t.Errorf("plain = %q, want %q", plain, want)
	}
	if !strings.Contains(out, "\x1b[4") {
		t.Errorf("missing underline SGR: %q", out)
	}
}

// Annotation across a soft-wrap boundary.
func TestAdvAnnotationAcrossSoftWrap(t *testing.T) {
	src := "abcdefghijklmnop"
	anns := []Annotation{{Range: Range{6, 12}, Style: lipgloss.NewStyle().Underline(true)}}
	set := newAnnotationSet(src, anns)
	out := RenderAnnotated(src, 8, 3, 0, 99, Styles{}, ModeNormal, set)
	rows := strings.Split(out, "\n")
	r0 := strings.TrimRight(ansi.Strip(rows[0]), " ")
	r1 := strings.TrimRight(ansi.Strip(rows[1]), " ")
	if r0 != "abcdefgh" {
		t.Errorf("row0 plain = %q, want %q", r0, "abcdefgh")
	}
	if r1 != "ijklmnop" {
		t.Errorf("row1 plain = %q, want %q", r1, "ijklmnop")
	}
	if !strings.Contains(rows[0]+rows[1], "\x1b[4") {
		t.Errorf("underline SGR missing across soft-wrap: %q", out)
	}
}

// Empty annotation list returns identical output to plain Render.
func TestAdvEmptyAnnotationList(t *testing.T) {
	src := "hello world"
	set := newAnnotationSet(src, nil)
	withSet := RenderAnnotated(src, 40, 1, 0, 99, Styles{}, ModeNormal, set)
	withoutSet := Render(src, 40, 1, 0, 99, Styles{}, ModeNormal)
	if withSet != withoutSet {
		t.Errorf("empty set diverged:\nwith    %q\nwithout %q", withSet, withoutSet)
	}
}

// Annotation at the very last column (no soft-wrap involved).
func TestAdvAnnotationAtLastColumn(t *testing.T) {
	src := "hello"
	anns := []Annotation{{Range: Range{4, 5}, Style: lipgloss.NewStyle().Underline(true)}}
	set := newAnnotationSet(src, anns)
	out := RenderAnnotated(src, 80, 1, 0, 99, Styles{}, ModeNormal, set)
	plain := strings.TrimRight(ansi.Strip(out), " ")
	// cursor past EOL → appended block
	want := "hello█"
	if plain != want {
		t.Errorf("plain = %q, want %q", plain, want)
	}
	if !strings.Contains(out, "\x1b[4") {
		t.Errorf("underline SGR missing: %q", out)
	}
}
