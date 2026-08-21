// Package lipgloss is a styling test double: a stand-in for the
// real lipgloss module, giving the styling analyzer's testdata a
// package whose import path contains "lipgloss" without depending
// on the module.
package lipgloss

// Style is a stand-in for lipgloss.Style.
type Style struct{}

// NewStyle is a stand-in for lipgloss.NewStyle.
func NewStyle() Style { return Style{} }

// Bold is a stand-in for one of Style's mutating attribute setters.
func (s Style) Bold(v bool) Style { return s }

// Foreground is a stand-in for one of Style's mutating attribute setters.
func (s Style) Foreground(c string) Style { return s }

// Underline is a stand-in for one of Style's mutating attribute setters.
func (s Style) Underline(v bool) Style { return s }

// Width is a stand-in for one of Style's mutating attribute setters.
func (s Style) Width(i int) Style { return s }

// GetBold is a stand-in for one of Style's read-only accessors.
func (s Style) GetBold() bool { return false }

// Render is a stand-in for Style's render method, which produces a
// string rather than mutating the receiver.
func (s Style) Render(str string) string { return str }
