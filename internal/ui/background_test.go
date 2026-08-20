package ui

import (
	"image/color"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/glw907/poplar/internal/theme"
)

func TestBackgroundColorWaitConstant(t *testing.T) {
	if BackgroundColorWait != 100*time.Millisecond {
		t.Errorf("BackgroundColorWait = %v, want 100ms", BackgroundColorWait)
	}
}

// TestQueryBackgroundColorNeverAnswers proves a never-answering
// terminal still renders. It invokes only the timeout half of
// QueryBackgroundColor's batch, standing in for a terminal whose
// tea.BackgroundColorMsg answer never arrives, and asserts that half
// still resolves to BackgroundColorTimeoutMsg within the bound
// instead of hanging Update forever.
func TestQueryBackgroundColorNeverAnswers(t *testing.T) {
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
	case <-time.After(BackgroundColorWait * 10):
		t.Fatal("timeout command never returned; a never-answering terminal would hang the first frame")
	}
}

// TestBackgroundColorRepaint is the golden pair the repaint path
// promises (technical design section 12): the theme the root model
// renders on DefaultDark before any answer, and the theme it
// rebuilds from a later tea.BackgroundColorMsg answering light, are
// both fixed values from decision 6's palette, not two competing
// renders racing for the screen.
func TestBackgroundColorRepaint(t *testing.T) {
	profile := theme.ProfileTrueColor

	preAnswer := fgRGBA(t, theme.New(DefaultDark, profile))
	wantPreAnswer := color.RGBA{R: 0xD4, G: 0xD8, B: 0xDF, A: 0xff} // decision 6's dark fg
	if preAnswer != wantPreAnswer {
		t.Errorf("pre-answer fg = %#v, want %#v", preAnswer, wantPreAnswer)
	}
	if again := fgRGBA(t, theme.New(DefaultDark, profile)); again != preAnswer {
		t.Errorf("pre-answer fg is not stable across renders: %#v then %#v", preAnswer, again)
	}

	answer := tea.BackgroundColorMsg{Color: color.White}
	if answer.IsDark() {
		t.Fatal("color.White must resolve to a light background for this fixture")
	}
	postAnswer := fgRGBA(t, theme.New(answer.IsDark(), profile))
	wantPostAnswer := color.RGBA{R: 0x26, G: 0x2B, B: 0x33, A: 0xff} // decision 6's light fg
	if postAnswer != wantPostAnswer {
		t.Errorf("post-answer fg = %#v, want %#v", postAnswer, wantPostAnswer)
	}
}

func fgRGBA(t *testing.T, th theme.Theme) color.RGBA {
	t.Helper()
	c, ok := th.Style(theme.RoleFg, theme.GroundBase).GetForeground().(color.RGBA)
	if !ok {
		t.Fatalf("RoleFg foreground is not a color.RGBA at ProfileTrueColor")
	}
	return c
}
