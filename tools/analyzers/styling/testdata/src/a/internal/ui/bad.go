package ui

import "lipgloss"

func bad() {
	_ = "•"                                    // want `non-ASCII literal outside internal/theme and internal/catkin`
	_ = "\x1b[31m"                              // want `ANSI escape literal outside internal/theme and internal/catkin`
	_ = lipgloss.NewStyle()                     // want `lipgloss call outside internal/theme and internal/catkin`
	_ = "café" /*poplar:allow-unicode corpus fixture keeps the original accent*/
	_ = "\x1b[32m" /*poplar:allow-unicode this directive must not exempt an ANSI escape*/ // want `ANSI escape literal outside internal/theme and internal/catkin`
	_ = lipgloss.NewStyle() /*poplar:allow-unicode this directive must not exempt a lipgloss call*/ // want `lipgloss call outside internal/theme and internal/catkin`
	_ = "•\x1b[31m" /*poplar:allow-unicode a non-ASCII rune preceding the ESC byte must not exempt it*/ // want `ANSI escape literal outside internal/theme and internal/catkin`
}
