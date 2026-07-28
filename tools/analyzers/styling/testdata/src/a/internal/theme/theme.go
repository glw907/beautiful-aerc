// Package theme owns the compiled tokens; the styling boundary
// excludes it, so it may call lipgloss and hold non-ASCII glyphs
// directly.
package theme

import "lipgloss"

// Bullet is a compiled glyph token.
const Bullet = "•"

// Base is the compiled base style.
var Base = lipgloss.NewStyle()
