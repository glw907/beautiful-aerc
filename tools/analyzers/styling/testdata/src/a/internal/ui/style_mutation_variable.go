package ui

import "lipgloss"

// mutateThroughVariable proves an intermediate variable hop does not
// hide the mutation from the receiver's declared type.
func mutateThroughVariable() lipgloss.Style {
	base := lipgloss.NewStyle() // want `lipgloss call outside internal/theme and internal/catkin`
	held := base
	return held.Underline(true) // want `mutating lipgloss.Style method "Underline" outside internal/theme and internal/catkin`
}
