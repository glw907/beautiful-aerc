package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// DefaultDark is the isDark policy ResolveProfile returns and the
// root model's first frame renders on; see QueryBackgroundColor for
// how a later answer corrects it.
const DefaultDark = true

// BackgroundColorWait bounds QueryBackgroundColor's timeout half.
const BackgroundColorWait = 100 * time.Millisecond

// BackgroundColorTimeoutMsg reports that BackgroundColorWait elapsed
// without a tea.BackgroundColorMsg answer (see QueryBackgroundColor).
// The root model's Update absorbs it to log the outcome once, at
// debug level ("query unanswered, staying dark"); DefaultDark
// already governs every frame rendered so far, so no other handling
// is required.
type BackgroundColorTimeoutMsg struct{}

// QueryBackgroundColor is the Cmd a root model's Init issues to run
// the first-frame background-color policy: it batches
// tea.RequestBackgroundColor with a BackgroundColorWait tick, so
// Init returns immediately and neither half blocks it. The root
// model applies whichever tea.BackgroundColorMsg its Update later
// receives, before or after the bound, by rebuilding its
// theme.Theme with msg.IsDark() and its resolved profile; the
// returned Update triggers the normal repaint. A bound with no
// answer yet yields BackgroundColorTimeoutMsg instead, described on
// its own type.
func QueryBackgroundColor() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, backgroundColorTimeout())
}

func backgroundColorTimeout() tea.Cmd {
	return tea.Tick(BackgroundColorWait, func(time.Time) tea.Msg {
		return BackgroundColorTimeoutMsg{}
	})
}
