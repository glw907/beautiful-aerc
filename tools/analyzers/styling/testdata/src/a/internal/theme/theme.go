// Package theme owns the compiled tokens; the styling boundary
// excludes it, so it may call lipgloss and hold non-ASCII glyphs
// directly.
package theme

import "lipgloss"

// Bullet is a compiled glyph token.
const Bullet = "•"

// Base is the compiled base style, built by chaining a mutating
// method (Bold) off a freshly constructed one: this package is the
// styling boundary's home, so neither call is flagged.
var Base = lipgloss.NewStyle().Bold(true)
