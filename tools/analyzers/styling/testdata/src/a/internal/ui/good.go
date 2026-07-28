package ui

// good uses only ASCII literals and never touches lipgloss
// directly; the theme package owns styling.
func good() string {
	return "hello, world"
}
