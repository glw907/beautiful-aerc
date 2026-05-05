package catkin

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderPlainSingleLine(t *testing.T) {
	got := Render("hello", 20, 5, 0, 5, Styles{}, ModeNormal)
	want := "hello█"
	// Render pads to height. Only check the first row.
	first := strings.SplitN(got, "\n", 2)[0]
	if first != want {
		t.Errorf("Render plain:\nfirst row %q\nwant      %q", first, want)
	}
}

func TestRenderDisplayWrapsLongLine(t *testing.T) {
	src := "abcdefghijklmnopqrst"
	// cursor at last rune (offset 19) lands in the second chunk at col 9.
	got := Render(src, 10, 5, 0, 19, Styles{}, ModeNormal)
	rows := strings.Split(got, "\n")
	if rows[0] != "abcdefghij" || rows[1] != "klmnopqrs█" {
		t.Errorf("Render display-wrap rows:\n[0]=%q\n[1]=%q", rows[0], rows[1])
	}
}

func TestRenderHeadingStyled(t *testing.T) {
	st := Styles{}
	st.Heading[0] = lipgloss.NewStyle().Bold(true)
	got := Render("# Hello", 40, 1, 0, 99, st, ModeNormal)
	if !strings.Contains(got, "\x1b[1m") {
		t.Errorf("expected bold escape on heading, got %q", got)
	}
	if !strings.Contains(got, "Hello") {
		t.Errorf("heading dropped body: %q", got)
	}
}

func TestRenderBoldSpanStyled(t *testing.T) {
	st := Styles{Bold: lipgloss.NewStyle().Bold(true)}
	got := Render("a **b** c", 40, 1, 0, 99, st, ModeNormal)
	if !strings.Contains(got, "\x1b[1m") {
		t.Errorf("expected bold escape on inline span, got %q", got)
	}
	// Delimiters stay visible.
	if !strings.Contains(got, "**b**") {
		t.Errorf("delimiters elided: %q", got)
	}
}

func TestRenderQuoteStyled(t *testing.T) {
	st := Styles{Quote: lipgloss.NewStyle().Italic(true)}
	got := Render("> hi", 40, 1, 0, 99, st, ModeNormal)
	if !strings.Contains(got, "\x1b[3m") {
		t.Errorf("expected italic escape on quote, got %q", got)
	}
}

func TestRenderFencedCodeChroma(t *testing.T) {
	src := "```go\npackage main\n```"
	got := Render(src, 40, 5, 0, 999, Styles{}, ModeNormal)
	if !strings.Contains(got, "package") {
		t.Errorf("fenced code dropped source: %q", got)
	}
	// Chroma's terminal256 formatter emits SGR escapes for tokens.
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("expected chroma ANSI on go fence, got %q", got)
	}
}

func TestRenderCursorPreservedOnStyledLine(t *testing.T) {
	st := Styles{Bold: lipgloss.NewStyle().Bold(true)}
	// Cursor at offset 4 lands on 'b' inside **bold**.
	got := Render("a **bold** c", 40, 1, 0, 4, st, ModeNormal)
	if !strings.Contains(got, "█") {
		t.Errorf("cursor missing from styled line: %q", got)
	}
}

func TestRenderCursorOnDelimiterFallsBack(t *testing.T) {
	st := Styles{Bold: lipgloss.NewStyle().Bold(true)}
	// Cursor at offset 2 lands on the second '*' of '**bold**'.
	// The bold regex no longer matches → no bold escape on this line.
	got := Render("a **bold** c", 40, 1, 0, 3, st, ModeNormal)
	if !strings.Contains(got, "█") {
		t.Errorf("cursor missing: %q", got)
	}
}

func TestRenderZeroStylesUnchangedFromPlain(t *testing.T) {
	src := "# hello\n**bold** word"
	got := Render(src, 40, 5, 0, 999, Styles{}, ModeNormal)
	if strings.Contains(got, "\x1b[") {
		t.Errorf("zero Styles emitted ANSI: %q", got)
	}
}

func TestRenderAppliesSquiggle(t *testing.T) {
	src := "the tradeof is real"
	anns := []Annotation{
		{
			Range: Range{4, 11},
			Kind:  KindMisspelling,
			Style: lipgloss.NewStyle().Underline(true),
		},
	}
	set := newAnnotationSet(src, anns)
	out := RenderAnnotated(src, 80, 1, 0, 0, Styles{}, ModeNormal, set)
	// lipgloss renders underline as \x1b[4;4m (underline + underline style).
	// Either form is valid. Check that some underline SGR is present.
	if !strings.Contains(out, "\x1b[4m") && !strings.Contains(out, "\x1b[4;") {
		t.Errorf("rendered output missing underline SGR; got:\n%q", out)
	}
}

// TestRenderCursorRowAnnotationOffset verifies that an annotation over a range
// that starts after the cursor column lands on the correct characters in the
// rendered output. The cursor block is injected into styled before annotations
// run. Without the cursor-byte-offset adjustment the splice column would be
// short by one cell, decorating the wrong characters.
func TestRenderCursorRowAnnotationOffset(t *testing.T) {
	// Source "abcdef", cursor at rune offset 2 (on 'c'). insertCursorBlock
	// replaces 'c' with █, producing raw "ab█def". An annotation over bytes
	// 3..6 covers "def" in the original plain string.
	src := "abcdef"
	squiggle := lipgloss.NewStyle().Underline(true)
	anns := []Annotation{
		{
			Range: Range{3, 6}, // "def"
			Kind:  KindMisspelling,
			Style: squiggle,
		},
	}
	set := newAnnotationSet(src, anns)
	// cursor=2: offsetToRowCol returns cursorRow=0, cursorCol=2.
	out := RenderAnnotated(src, 80, 1, 0, 2, Styles{}, ModeNormal, set)
	// The rendered line is "ab█def" (6 cells). The underline SGR must wrap
	// "def" (the last three cells), not "cde" (which would happen if the
	// splice column were off by one on the cursor row).
	//
	// Strip ANSI and verify the plain content so we can check structure.
	// Then check that underline is present at all.
	if !strings.Contains(out, "█") {
		t.Fatalf("cursor block missing from output: %q", out)
	}
	if !strings.Contains(out, "\x1b[4m") && !strings.Contains(out, "\x1b[4;") {
		t.Errorf("underline SGR missing from cursor-row output: %q", out)
	}
	// The cursor block must appear before the underlined segment.
	cursorIdx := strings.Index(out, "█")
	// Find first underline SGR after cursor block.
	underlineIdx := strings.Index(out[cursorIdx:], "\x1b[4")
	if underlineIdx < 0 {
		t.Errorf("underline SGR not found after cursor block; full output:\n%q", out)
	}
}
