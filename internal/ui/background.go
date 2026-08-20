package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// DefaultDark is the isDark policy ResolveProfile returns and the
// root model's first frame renders on, before any
// tea.BackgroundColorMsg answers.
const DefaultDark = true

// BackgroundColorWait bounds how long the root model waits on the
// terminal's own answer before treating DefaultDark as final for a
// given frame. It does not block Init or drop a late answer: Init
// issues QueryBackgroundColor's Cmd and returns immediately, and a
// tea.BackgroundColorMsg arriving after the bound still updates
// isDark and repaints on the next Update/View cycle (technical
// design section 12).
const BackgroundColorWait = 100 * time.Millisecond

// BackgroundColorTimeoutMsg reports that BackgroundColorWait elapsed
// without a tea.BackgroundColorMsg. The root model needs no
// dedicated handling for it: DefaultDark already governs the frames
// rendered so far, and a tea.BackgroundColorMsg received afterward
// still corrects isDark.
type BackgroundColorTimeoutMsg struct{}

// QueryBackgroundColor is the Cmd a root model's Init issues to run
// the first-frame background-color policy: it requests the
// terminal's background color and starts the BackgroundColorWait
// bound in the same batch. The root model applies whichever
// tea.BackgroundColorMsg arrives, before or after the bound, by
// rebuilding its theme.Theme with msg.IsDark() and its resolved
// profile; the returned Update triggers the normal repaint.
func QueryBackgroundColor() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, backgroundColorTimeout())
}

func backgroundColorTimeout() tea.Cmd {
	return tea.Tick(BackgroundColorWait, func(time.Time) tea.Msg {
		return BackgroundColorTimeoutMsg{}
	})
}
