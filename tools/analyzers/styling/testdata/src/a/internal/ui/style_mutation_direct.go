package ui

import "lipgloss"

// mutateDirect calls a mutating style method directly on the value a
// package-qualified call just returned: the shape task 6's review
// flagged, a method chained off a theme-returned Style value.
func mutateDirect() lipgloss.Style {
	base := lipgloss.NewStyle() // want `lipgloss call outside internal/theme and internal/catkin`
	return base.Bold(true)      // want `mutating lipgloss.Style method "Bold" outside internal/theme and internal/catkin`
}
