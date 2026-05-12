package catkin

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func newModelWithMisspelling(t *testing.T) (Model, Range) {
	t.Helper()
	speller := newFixtureSpeller(t, nil)
	m := New().WithSize(80, 10).WithValue("the tradeof is real").WithAnnotator(NewSpellcheckAnnotator(speller, Styles{}))
	// Run annotators inline, bypassing the tick.
	anns := runAnnotators(m.annotators, m.buf.Value())
	m.annotations = newAnnotationSet(m.buf.Value(), anns)
	if len(anns) != 1 {
		t.Fatalf("expected one annotation, got %d", len(anns))
	}
	return m, anns[0].Range
}

func TestPopoverOpensOnRange(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf = m.buf.WithRuneOffset(r.Start) // cursor on misspelling
	m, _ = m.Update(tea.KeyPressMsg{Code: ';', Text: ";"})
	if !m.popover.open {
		t.Errorf("; on misspelling should open popover")
	}
	if m.popover.word != "tradeof" {
		t.Errorf("popover.word = %q, want \"tradeof\"", m.popover.word)
	}
}

func TestPopoverCloseOnEsc(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf = m.buf.WithRuneOffset(r.Start)
	m, _ = m.Update(tea.KeyPressMsg{Code: ';', Text: ";"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.popover.open {
		t.Errorf("Esc should close popover")
	}
}

func TestPopoverApplyReplacesRange(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf = m.buf.WithRuneOffset(r.Start)
	m, _ = m.Update(tea.KeyPressMsg{Code: ';', Text: ";"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // apply first suggestion
	if got := m.buf.Value(); got != "the tradeoff is real" {
		t.Errorf("after apply: %q, want \"the tradeoff is real\"", got)
	}
	if m.popover.open {
		t.Errorf("apply should close popover")
	}
}

func TestPopoverDoesNotOpenOffRange(t *testing.T) {
	m, _ := newModelWithMisspelling(t)
	m.buf = m.buf.WithRuneOffset(0) // cursor on "the", not on misspelling
	m, _ = m.Update(tea.KeyPressMsg{Code: ';', Text: ";"})
	if m.popover.open {
		t.Errorf("; off a misspelling should not open popover")
	}
}

func TestPopoverDigitJumpApply(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf = m.buf.WithRuneOffset(r.Start)
	m, _ = m.Update(tea.KeyPressMsg{Code: ';', Text: ";"})
	if len(m.popover.suggestions) < 1 {
		t.Skipf("no suggestions; cannot exercise digit jump")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	if m.popover.open {
		t.Errorf("digit jump should close popover")
	}
}

func TestPopoverAddToWordlist(t *testing.T) {
	dir := t.TempDir()
	listPath := dir + "/wordlist.txt"

	m, r := newModelWithMisspelling(t)
	m = m.WithUserWordlistPath(listPath)
	m.buf = m.buf.WithRuneOffset(r.Start)
	m, _ = m.Update(tea.KeyPressMsg{Code: ';', Text: ";"})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})

	got, err := os.ReadFile(listPath)
	if err != nil {
		t.Fatalf("read wordlist: %v", err)
	}
	if !strings.Contains(string(got), "tradeof") {
		t.Errorf("wordlist missing \"tradeof\":\n%s", got)
	}
	if m.popover.open {
		t.Errorf("AddToWordlist should close popover")
	}
}

func TestPopoverRenderShape(t *testing.T) {
	pop := popoverState{
		open:        true,
		word:        "tradeof",
		suggestions: []string{"tradeoff", "trade-off"},
		cursor:      0,
	}
	out := pop.render(40, Styles{})
	lines := strings.Split(out, "\n")
	// header + 2 suggestions + blank separator + actions + 2 borders = 7
	if len(lines) != 7 {
		t.Fatalf("popover lines = %d, want 7\n%s", len(lines), out)
	}
	// lines[0] is the top border. lines[1] is the header content row.
	if !strings.Contains(lines[1], "tradeof") {
		t.Errorf("header line missing word: %q", lines[1])
	}
	if !strings.Contains(out, "tradeoff") || !strings.Contains(out, "trade-off") {
		t.Errorf("render missing suggestions:\n%s", out)
	}
}

func TestPopoverRenderZeroSuggestions(t *testing.T) {
	pop := popoverState{open: true, word: "qzwxy", suggestions: nil}
	out := pop.render(40, Styles{})
	lines := strings.Split(out, "\n")
	// header + actions + 2 borders = 4
	if len(lines) != 4 {
		t.Fatalf("zero-suggestion popover lines = %d, want 4\n%s", len(lines), out)
	}
}

func TestPopoverPositionBelow(t *testing.T) {
	pop := popoverState{open: true, word: "x", suggestions: []string{"y"}}
	row, col := pop.position(2, 5, 80, 24)
	if row != 3 {
		t.Errorf("position row below cursor = %d, want 3", row)
	}
	if col != 5 {
		t.Errorf("position col = %d, want 5", col)
	}
}

func TestPopoverPositionAboveOnNoRoom(t *testing.T) {
	pop := popoverState{open: true, word: "x", suggestions: []string{"y", "z"}}
	// Cursor near bottom: popover would overflow if drawn below.
	row, _ := pop.position(22, 0, 80, 24)
	if row >= 22 {
		t.Errorf("position row near bottom = %d, want above cursor", row)
	}
}

func TestPopoverHorizontalClamp(t *testing.T) {
	pop := popoverState{open: true, word: "x", suggestions: []string{"y"}}
	_, col := pop.position(2, 78, 80, 24) // anchor near right edge
	if col+pop.width() > 80 {
		t.Errorf("position col+width = %d, want ≤ 80", col+pop.width())
	}
}

func TestOverlayPadsShortLines(t *testing.T) {
	// Body has two short lines. Popover requested at col 10 must render
	// at col 10 on the second line, not collapse to col 0.
	body := "abc\ndef"
	pop := "[POP]"
	out := overlay(body, pop, 1, 10)
	lines := strings.Split(out, "\n")
	if lines[0] != "abc" {
		t.Errorf("row 0 untouched: got %q", lines[0])
	}
	want := "def" + strings.Repeat(" ", 7) + "[POP]"
	if lines[1] != want {
		t.Errorf("row 1: got %q, want %q", lines[1], want)
	}
}

func TestOverlaySkipsOutOfRangeRows(t *testing.T) {
	// Row 0 stays untouched. Popover row 0 splices into body row 1.
	// Popover rows 1..2 fall outside the body and must not append.
	body := "abc\ndef"
	pop := "X\nY\nZ"
	out := overlay(body, pop, 1, 0)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("rows = %d, want 2: %q", len(lines), out)
	}
	if lines[0] != "abc" {
		t.Errorf("row 0: got %q, want %q", lines[0], "abc")
	}
	if lines[1] != "Xef" {
		t.Errorf("row 1: got %q, want %q", lines[1], "Xef")
	}
}

// modelWithPopoverOpen builds an 80×24 model with the spellcheck annotator
// registered, opens the popover on the first misspelling matching word
// (or the first misspelling if word is empty), and returns the model.
func modelWithPopoverOpen(t *testing.T, value, word string) Model {
	t.Helper()
	speller := newFixtureSpeller(t, nil)
	m := New().WithSize(80, 24).WithValue(value).WithAnnotator(NewSpellcheckAnnotator(speller, Styles{}))
	anns := runAnnotators(m.annotators, m.buf.Value())
	m.annotations = newAnnotationSet(m.buf.Value(), anns)
	off := -1
	for _, a := range anns {
		if a.Kind != KindMisspelling {
			continue
		}
		mp, _ := a.Payload.(MisspellingPayload)
		if word == "" || mp.Word == word {
			off = a.Range.Start
			break
		}
	}
	if off < 0 {
		t.Fatalf("no misspelling matching %q in %d annotations", word, len(anns))
	}
	m.buf = m.buf.WithRuneOffset(off)
	m, _ = m.WithFocus()
	m, _ = m.Update(tea.KeyPressMsg{Code: ';', Text: ";"})
	if !m.popover.open {
		t.Fatalf("popover did not open on %q", word)
	}
	return m
}

func TestPopoverRendersAtRightShiftedColumn(t *testing.T) {
	pad := strings.Repeat(" ", 73)
	m := modelWithPopoverOpen(t, pad+"tradeof", "tradeof")
	out := m.View()
	rows := strings.Split(out, "\n")
	// Popover footer line "a add  i ignore  esc x" is 22 cells; sized
	// with a 1-cell border on each side and the +2 in width() makes
	// the outer width 24 cells regardless of which suggestions the
	// speller returns. Pane is 80 cells wide, so the popover's top-
	// border row sits at col 80-24 = 56.
	const wantCol = 56
	border := rows[1]
	idx := strings.Index(border, "╭")
	if idx != wantCol {
		t.Errorf("popover top-border at col %d, want %d\nfull row: %q",
			idx, wantCol, border)
	}
}

func TestPopoverFlipsAboveAtBottomEdge(t *testing.T) {
	var ls []string
	for range 22 {
		ls = append(ls, "the quick brown fox")
	}
	ls = append(ls, "the tradeof here")
	m := modelWithPopoverOpen(t, strings.Join(ls, "\n"), "tradeof")
	out := m.View()
	rows := strings.Split(out, "\n")
	// Cursor is on the last visible row (22). Popover height ≥ 4 must
	// flip up. The top border should appear strictly above row 22.
	var topBorderRow = -1
	for i, r := range rows {
		if strings.Contains(r, "╭") {
			topBorderRow = i
			break
		}
	}
	if topBorderRow < 0 {
		t.Fatalf("no popover border found:\n%s", out)
	}
	if topBorderRow >= 22 {
		t.Errorf("popover top border at row %d, want < 22 (flip-above)", topBorderRow)
	}
}

func TestPopoverClosesOnCursorLeave(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf = m.buf.WithRuneOffset(r.Start)
	m, _ = m.WithFocus()
	m, _ = m.Update(tea.KeyPressMsg{Code: ';', Text: ";"})
	if !m.popover.open {
		t.Fatalf("setup: popover should be open")
	}
	// Move cursor outside the misspelling range with arrow-right
	// repeated past the word's end.
	for range 20 {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
		if !m.popover.open {
			break
		}
	}
	if m.popover.open {
		t.Errorf("popover should auto-close once cursor leaves the misspelling range")
	}
}
