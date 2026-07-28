package ui

import "lipgloss"

func bad() {
	_ = "•"                                    // want `non-ASCII literal outside internal/theme and internal/catkin`
	_ = "\x1b[31m"                              // want `ANSI escape literal outside internal/theme and internal/catkin`
	_ = lipgloss.NewStyle()                     // want `lipgloss call outside internal/theme and internal/catkin`
	_ = "café" /*poplar:allow-unicode corpus fixture keeps the original accent*/
}
