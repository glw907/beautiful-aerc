package catkin

import tea "charm.land/bubbletea/v2"

// handleCommand intercepts the Catkin command vocabulary:
// Enter, Tab, Shift+Tab, Ctrl+B/I/K/L/Q, and Ctrl+Space.
func handleCommand(b Buffer, msg tea.KeyPressMsg) (handled bool, _ Buffer, _ tea.Cmd) {
	src := b.Value()
	cur := b.RuneOffset()
	var newSrc string
	var newCur int
	var ok bool

	switch msg.String() {
	case "enter":
		newSrc, newCur = smartEnter(src, cur)
	case "tab":
		newSrc, newCur = indentTab(src, cur)
	case "shift+tab":
		newSrc, newCur, ok = indentShiftTab(src, cur)
		if !ok {
			return false, b, nil
		}
	case "ctrl+b":
		newSrc, newCur = wrapWord(src, cur, "**")
	case "ctrl+i":
		newSrc, newCur = wrapWord(src, cur, "*")
	case "ctrl+k":
		newSrc, newCur = insertLinkSkeleton(src, cur)
	case "ctrl+l":
		newSrc, newCur = toggleList(src, cur)
	case "ctrl+q":
		newSrc, newCur = toggleQuote(src, cur)
	case "ctrl+@", "ctrl+ ":
		newSrc, newCur, ok = toggleTask(src, cur)
		if !ok {
			return false, b, nil
		}
	default:
		return false, b, nil
	}

	b = b.WithValue(newSrc).WithRuneOffset(newCur)
	return true, b, nil
}
