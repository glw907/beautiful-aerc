// Package lipgloss is a styling test double: a stand-in for the
// real lipgloss module, giving the styling analyzer's testdata a
// package whose import path contains "lipgloss" without depending
// on the module.
package lipgloss

// Style is a stand-in for lipgloss.Style.
type Style struct{}

// NewStyle is a stand-in for lipgloss.NewStyle.
func NewStyle() Style { return Style{} }
