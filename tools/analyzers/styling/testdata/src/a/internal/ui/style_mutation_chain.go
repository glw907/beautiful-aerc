package ui

import "lipgloss"

// mutateChained proves a method chain's intermediate receiver, a
// call expression rather than a bare identifier, still resolves to
// lipgloss.Style through go/types.
func mutateChained() string {
	styled := lipgloss.NewStyle().Bold(true) // want `lipgloss call outside internal/theme and internal/catkin` `mutating lipgloss.Style method "Bold" outside internal/theme and internal/catkin`
	return styled.Render("hi")
}
