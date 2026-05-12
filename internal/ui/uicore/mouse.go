package uicore

import tea "charm.land/bubbletea/v2"

// RebuildMouseMsg returns msg with its Mouse payload replaced by
// mu. Used by mouse-event dispatch points that translate global
// coordinates into pane-local ones before forwarding to a
// sub-model.
func RebuildMouseMsg(msg tea.MouseMsg, mu tea.Mouse) tea.Msg {
	switch msg.(type) {
	case tea.MouseClickMsg:
		return tea.MouseClickMsg(mu)
	case tea.MouseReleaseMsg:
		return tea.MouseReleaseMsg(mu)
	case tea.MouseWheelMsg:
		return tea.MouseWheelMsg(mu)
	case tea.MouseMotionMsg:
		return tea.MouseMotionMsg(mu)
	}
	return nil
}
