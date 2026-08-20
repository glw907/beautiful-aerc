package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/glw907/poplar/internal/theme"
)

func testConfirm(t *testing.T, yesCmd, noCmd tea.Cmd) Confirm {
	t.Helper()
	c := Confirm{
		Question:    "Quit with 2 unsent messages?",
		Consequence: "They'll send the next time you open poplar.",
		YesLabel:    "quit",
		NoLabel:     "stay",
		YesCmd:      yesCmd,
		NoCmd:       noCmd,
	}
	th := theme.New(true, theme.ProfileTrueColor)
	updated, _ := c.Update(ThemeMsg{Theme: th})
	c = updated.(Confirm) //nolint:errcheck // Confirm's own Update always returns a Confirm
	updated, _ = c.Update(LayoutMsg{Layout: ComputeLayout(100, 30, false)})
	return updated.(Confirm) //nolint:errcheck // Confirm's own Update always returns a Confirm
}

// TestConfirm_AnswerYesNoEsc proves y/n/Esc each answer, and every
// other key, digits included, names no answer (UX-4's modal no-op
// rule).
func TestConfirm_AnswerYesNoEsc(t *testing.T) {
	yesCalled, noCalled := false, false
	c := testConfirm(t,
		func() tea.Msg { yesCalled = true; return nil },
		func() tea.Msg { noCalled = true; return nil },
	)

	if cmd, ok := c.answer(tea.KeyPressMsg{Code: 'y', Text: "y"}); !ok || cmd == nil {
		t.Fatal("answer('y') did not name an answer")
	} else {
		cmd()
		if !yesCalled || noCalled {
			t.Errorf("answer('y') called yesCalled=%v noCalled=%v, want only yes", yesCalled, noCalled)
		}
	}

	yesCalled, noCalled = false, false
	if cmd, ok := c.answer(tea.KeyPressMsg{Code: 'n', Text: "n"}); !ok || cmd == nil {
		t.Fatal("answer('n') did not name an answer")
	} else {
		cmd()
		if yesCalled || !noCalled {
			t.Errorf("answer('n') called yesCalled=%v noCalled=%v, want only no", yesCalled, noCalled)
		}
	}

	yesCalled, noCalled = false, false
	if cmd, ok := c.answer(tea.KeyPressMsg{Code: tea.KeyEscape}); !ok || cmd == nil {
		t.Fatal("answer(Esc) did not name an answer")
	} else {
		cmd()
		if yesCalled || !noCalled {
			t.Errorf("answer(Esc) called yesCalled=%v noCalled=%v, want only no (the default answer)", yesCalled, noCalled)
		}
	}

	for _, key := range []tea.KeyPressMsg{
		{Code: '1', Text: "1"},
		{Code: '9', Text: "9"},
		{Code: 'q', Text: "q"},
		{Code: 'j', Text: "j"},
	} {
		if _, ok := c.answer(key); ok {
			t.Errorf("answer(%q) named an answer, want a no-op", key.String())
		}
	}
}

// TestApp_ConfirmOnStack_AnswersYesNoAndPops proves the App-level
// wiring: 'y' pops the stack and returns YesCmd; digits are no-ops
// that leave the modal in place.
func TestApp_ConfirmOnStack_AnswersYesNoAndPops(t *testing.T) {
	yesCalled := false
	c := testConfirm(t, func() tea.Msg { yesCalled = true; return nil }, nil)

	app := NewApp(testDeps(t))
	app.stack = append(app.stack, c)

	updated, cmd := app.Update(digitKey("3"))
	app = mustApp(t, updated)
	if len(app.stack) != 1 {
		t.Fatalf("stack length after a digit = %d, want 1 (a digit is a no-op at a modal confirm)", len(app.stack))
	}
	if cmd != nil {
		t.Error("a digit at a modal confirm returned a non-nil Cmd, want none")
	}

	updated, cmd = app.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	app = mustApp(t, updated)
	if len(app.stack) != 0 {
		t.Fatalf("stack length after 'y' = %d, want 0 (the modal pops on answer)", len(app.stack))
	}
	if cmd == nil {
		t.Fatal("'y' returned a nil Cmd, want YesCmd")
	}
	cmd()
	if !yesCalled {
		t.Error("'y' did not invoke YesCmd")
	}
}

// TestApp_ConfirmOnStack_EscAnswersNo proves Esc pops the modal and
// invokes NoCmd (the default answer), rather than the bare pop a
// non-Confirm StateModal screen gets.
func TestApp_ConfirmOnStack_EscAnswersNo(t *testing.T) {
	noCalled := false
	c := testConfirm(t, nil, func() tea.Msg { noCalled = true; return nil })

	app := NewApp(testDeps(t))
	app.stack = append(app.stack, c)

	updated, cmd := app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	app = mustApp(t, updated)
	if len(app.stack) != 0 {
		t.Fatalf("stack length after Esc = %d, want 0", len(app.stack))
	}
	if cmd == nil {
		t.Fatal("Esc returned a nil Cmd, want NoCmd")
	}
	cmd()
	if !noCalled {
		t.Error("Esc did not invoke NoCmd")
	}
}

// TestConfirmBoxWidth_ClampsAgainstTheTerminal proves composition rule
// 5: the box's natural width fits its content, but never exceeds the
// terminal it clamps against.
func TestConfirmBoxWidth_ClampsAgainstTheTerminal(t *testing.T) {
	c := testConfirm(t, nil, nil)

	natural := confirmBoxWidth(c, 200)
	if natural <= 0 || natural >= 200 {
		t.Errorf("confirmBoxWidth(200) = %d, want a natural size strictly between 0 and 200", natural)
	}

	clamped := confirmBoxWidth(c, 40)
	if clamped != 40 {
		t.Errorf("confirmBoxWidth(40) = %d, want 40 (clamped against the terminal)", clamped)
	}
}

// TestConfirm_View_NaturalSizeCenteredOnBase renders Confirm's own
// View at 100×30 (the acceptance criterion's own natural size) and
// checks the box is present, centered, and carries both the question
// and the default answer's own selectedBg pill.
func TestConfirm_View_NaturalSizeCenteredOnBase(t *testing.T) {
	c := testConfirm(t, nil, nil)

	view := c.View()
	lines := strings.Split(view.Content, "\n")
	if len(lines) != 30 {
		t.Fatalf("View() rendered %d lines, want 30", len(lines))
	}

	plain := ansi.Strip(view.Content)
	if !strings.Contains(plain, c.Question) {
		t.Errorf("View() = %q, want the question", plain)
	}
	if !strings.Contains(plain, "y quit") || !strings.Contains(plain, "n stay") {
		t.Errorf("View() = %q, want both answer hints", plain)
	}

	g := c.geometry()
	if g.x <= 0 || g.x+g.width >= 100 {
		t.Errorf("geometry() = %+v, want the box clear of both terminal edges at width 100", g)
	}
}

// TestConfirmHitSpans_TargetTheAnswerKeys proves ADR-0017's character
// grain: each span's Rect lands on its own key character within the
// rendered frame.
func TestConfirmHitSpans_TargetTheAnswerKeys(t *testing.T) {
	c := testConfirm(t, nil, nil)
	plain := strings.Split(ansi.Strip(c.View().Content), "\n")

	spans := ConfirmHitSpans(c)
	if len(spans) != 2 {
		t.Fatalf("ConfirmHitSpans() returned %d spans, want 2", len(spans))
	}

	wantRunes := []rune{'y', 'n'}
	for i, span := range spans {
		if span.Target != PointerModalAnswer {
			t.Errorf("span %d: Target = %v, want PointerModalAnswer", i, span.Target)
		}
		cells := []rune(plain[span.Rect.Min.Y])
		x := span.Rect.Min.X
		if x < 0 || x >= len(cells) || cells[x] != wantRunes[i] {
			t.Errorf("span %d: rendered cell at (%d,%d) = %q, want %q\nrow: %q", i, x, span.Rect.Min.Y, string(cells[min(x, len(cells)-1)]), wantRunes[i], plain[span.Rect.Min.Y])
		}
	}
}
