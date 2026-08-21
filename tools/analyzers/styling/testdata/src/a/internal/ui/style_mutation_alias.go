package ui

import lg "lipgloss"

// mutateAliased proves the local import name never hides a
// mutation: the receiver resolves through the method's declared
// type, not the identifier a caller happened to import lipgloss
// under.
func mutateAliased() lg.Style {
	base := lg.NewStyle()         // want `lipgloss call outside internal/theme and internal/catkin`
	return base.Foreground("red") // want `mutating lipgloss.Style method "Foreground" outside internal/theme and internal/catkin`
}
