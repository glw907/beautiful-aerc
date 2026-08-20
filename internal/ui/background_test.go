package ui

import (
	"image/color"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/theme"
)

// TestQueryBackgroundColorNeverAnswers proves a never-answering
// terminal still renders. It invokes only the timeout half of
// QueryBackgroundColor's batch, standing in for a terminal whose
// tea.BackgroundColorMsg answer never arrives, and asserts that half
// still resolves to BackgroundColorTimeoutMsg within a ceiling well
// past BackgroundColorWait, rather than hanging Update forever. The
// ceiling is a literal, not a multiple of BackgroundColorWait, so
// this guard still fails if the constant grows past it.
func TestQueryBackgroundColorNeverAnswers(t *testing.T) {
	const ceiling = 150 * time.Millisecond

	msg := QueryBackgroundColor()()
	batch, ok := msg.(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("QueryBackgroundColor() yielded %#v, want a two-command tea.BatchMsg", msg)
	}

	done := make(chan tea.Msg, 1)
	go func() { done <- batch[1]() }()

	select {
	case got := <-done:
		if _, ok := got.(BackgroundColorTimeoutMsg); !ok {
			t.Errorf("timeout command yielded %#v, want BackgroundColorTimeoutMsg", got)
		}
	case <-time.After(ceiling):
		t.Fatalf("timeout command did not return within %v; a never-answering terminal would hang the first frame", ceiling)
	}
}

// TestBackgroundColorRepaint proves the repaint path is deterministic
// rather than a race: the theme built on DefaultDark before any
// answer, and the theme rebuilt from a later tea.BackgroundColorMsg
// answering light, are each stable across repeated construction, and
// differ from each other for the same profile.
func TestBackgroundColorRepaint(t *testing.T) {
	profile := theme.ProfileTrueColor

	preAnswer := fg(theme.New(DefaultDark, profile))
	if again := fg(theme.New(DefaultDark, profile)); again != preAnswer {
		t.Errorf("pre-answer fg is not stable across renders: %v then %v", preAnswer, again)
	}

	answer := tea.BackgroundColorMsg{Color: color.White}
	if answer.IsDark() {
		t.Fatal("color.White must resolve to a light background for this fixture")
	}
	postAnswer := fg(theme.New(answer.IsDark(), profile))
	if again := fg(theme.New(answer.IsDark(), profile)); again != postAnswer {
		t.Errorf("post-answer fg is not stable across renders: %v then %v", postAnswer, again)
	}

	if preAnswer == postAnswer {
		t.Errorf("pre-answer and post-answer fg are identical (%v); the repaint changed nothing", preAnswer)
	}
}

func fg(th theme.Theme) color.Color {
	return th.Style(theme.RoleFg, theme.GroundBase).GetForeground()
}
