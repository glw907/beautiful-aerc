package catkin

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

func TestRenderQuoteContinuationCarriesStyledPrefix(t *testing.T) {
	st := Styles{Quote: lipgloss.NewStyle().Italic(true)}
	src := "> alpha beta gamma delta epsilon zeta eta theta"
	got := Render(src, 16, 6, 0, 0, st, ModeNormal)
	rows := strings.Split(got, "\n")
	nonBlank := 0
	for _, r := range rows {
		if strings.TrimSpace(ansi.Strip(r)) == "" {
			continue
		}
		nonBlank++
		if !strings.Contains(r, "\x1b[3m") {
			t.Errorf("continuation row missing italic SGR: %q", r)
		}
		plain := ansi.Strip(r)
		if !strings.HasPrefix(plain, "> ") {
			t.Errorf("continuation row missing > prefix: %q", plain)
		}
	}
	if nonBlank < 2 {
		t.Fatalf("expected ≥2 continuation rows, got %d", nonBlank)
	}
}

func TestRenderQuoteCursorLandsOnContinuationRow(t *testing.T) {
	src := "> alpha beta gamma delta epsilon"
	// Place cursor inside "epsilon" (offset 28 in the source).
	got := Render(src, 14, 6, 0, 28, Styles{}, ModeNormal)
	rows := strings.Split(got, "\n")
	cursorRowIdx := -1
	for i, r := range rows {
		if strings.Contains(ansi.Strip(r), "█") {
			cursorRowIdx = i
			break
		}
	}
	if cursorRowIdx < 1 {
		t.Fatalf("cursor must land on a continuation row (>=1); got idx=%d rows=%q", cursorRowIdx, rows)
	}
	if !strings.HasPrefix(ansi.Strip(rows[cursorRowIdx]), "> ") {
		t.Errorf("cursor's continuation row missing > prefix: %q", rows[cursorRowIdx])
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
