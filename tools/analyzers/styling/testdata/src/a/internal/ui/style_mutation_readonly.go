package ui

import "lipgloss"

// readBold reads a style's Bold attribute rather than setting it:
// GetBold does not start with "Bold", so it never matches the
// mutating-method check, and this call carries no want comment.
func readBold(s lipgloss.Style) bool {
	return s.GetBold()
}
