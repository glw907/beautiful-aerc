package catkin

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newModelWithMisspelling(t *testing.T) (Model, Range) {
	t.Helper()
	speller := newFixtureSpeller(t, nil)
	m := New()
	m.SetSize(80, 10)
	m.SetValue("the tradeof is real")
	m.RegisterAnnotator(NewSpellcheckAnnotator(speller, Styles{}))
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
	m.buf.SetRuneOffset(r.Start) // cursor on misspelling
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{';'}})
	if !m.popover.open {
		t.Errorf("; on misspelling should open popover")
	}
	if m.popover.word != "tradeof" {
		t.Errorf("popover.word = %q, want \"tradeof\"", m.popover.word)
	}
}

func TestPopoverCloseOnEsc(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf.SetRuneOffset(r.Start)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{';'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.popover.open {
		t.Errorf("Esc should close popover")
	}
}

func TestPopoverApplyReplacesRange(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf.SetRuneOffset(r.Start)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{';'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // apply first suggestion
	if got := m.buf.Value(); got != "the tradeoff is real" {
		t.Errorf("after apply: %q, want \"the tradeoff is real\"", got)
	}
	if m.popover.open {
		t.Errorf("apply should close popover")
	}
}

func TestPopoverDoesNotOpenOffRange(t *testing.T) {
	m, _ := newModelWithMisspelling(t)
	m.buf.SetRuneOffset(0) // cursor on "the", not on misspelling
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{';'}})
	if m.popover.open {
		t.Errorf("; off a misspelling should not open popover")
	}
}

func TestPopoverDigitJumpApply(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf.SetRuneOffset(r.Start)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{';'}})
	if len(m.popover.suggestions) < 1 {
		t.Skipf("no suggestions; cannot exercise digit jump")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if m.popover.open {
		t.Errorf("digit jump should close popover")
	}
}

func TestPopoverAddToWordlist(t *testing.T) {
	dir := t.TempDir()
	listPath := dir + "/wordlist.txt"

	m, r := newModelWithMisspelling(t)
	m.SetUserWordlistPath(listPath)
	m.buf.SetRuneOffset(r.Start)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{';'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

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

func TestPopoverClosesOnCursorLeave(t *testing.T) {
	m, r := newModelWithMisspelling(t)
	m.buf.SetRuneOffset(r.Start)
	m.Focus()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{';'}})
	if !m.popover.open {
		t.Fatalf("setup: popover should be open")
	}
	// Move cursor outside the misspelling range with arrow-right
	// repeated past the word's end.
	for i := 0; i < 20; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
		if !m.popover.open {
			break
		}
	}
	if m.popover.open {
		t.Errorf("popover should auto-close once cursor leaves the misspelling range")
	}
}
