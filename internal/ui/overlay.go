// SPDX-License-Identifier: MIT

// Vendored from github.com/yorukot/superfile (MIT) commit main/2025.
// Source: src/pkg/string_function/overplace.go
// Purpose: composite an overlay string at (x,y) atop a background string,
// preserving background ANSI outside the overlay rect.
//
// Rewritten to use only github.com/charmbracelet/x/ansi (already a poplar
// dependency) instead of muesli/reflow + muesli/ansi. The algorithm is
// identical: split both strings on "\n", splice each overlay row into the
// corresponding background row at the requested column offset, leave rows
// outside the overlay rect unchanged.
//
// Cell-width measurements use ansi.StringWidth, which is correct for
// non-icon ANSI text. The popover content has no SPUA-A Nerd Font icons
// so lipgloss.Width / ansi.StringWidth are both acceptable; ansi.StringWidth
// is chosen here because it also strips escape sequences before measuring,
// matching the Truncate/TruncateLeft semantics. Callers that pass
// icon-bearing strings should pre-measure with displayCells (iconwidth.go).
package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// PlaceOverlay composites fg (the overlay) at position (x, y) over bg
// (the background), preserving background content and ANSI styling outside
// the overlay rect.
//
//   - x and y are zero-based cell offsets from the top-left of bg.
//   - If the overlay extends past the right or bottom edge of bg it is
//     clipped — no panic.
//   - If fg is larger than bg in both dimensions, fg is returned as-is.
func PlaceOverlay(x, y int, fg, bg string) string {
	fgLines, fgWidth := splitLines(fg)
	bgLines, bgWidth := splitLines(bg)
	bgHeight := len(bgLines)
	fgHeight := len(fgLines)

	if fgWidth >= bgWidth && fgHeight >= bgHeight {
		return fg
	}

	// Clamp x/y so the overlay fits within the bg rect.
	x = min(max(x, 0), max(0, bgWidth-fgWidth))
	y = min(max(y, 0), max(0, bgHeight-fgHeight))

	var b strings.Builder
	for i, bgLine := range bgLines {
		if i > 0 {
			b.WriteByte('\n')
		}
		// Rows outside the overlay rect: pass through unchanged.
		if i < y || i >= y+fgHeight {
			b.WriteString(bgLine)
			continue
		}

		fgLine := fgLines[i-y]

		// Left segment: bg cells [0..x).
		// ansi.Truncate cuts to x visible cells, preserving ANSI opens.
		left := ansi.Truncate(bgLine, x, "")
		leftWidth := ansi.StringWidth(left)
		b.WriteString(left)
		// If bg line was shorter than x, pad with spaces.
		if leftWidth < x {
			b.WriteString(strings.Repeat(" ", x-leftWidth))
		}

		// Overlay segment.
		b.WriteString(fgLine)

		// Right segment: bg cells starting at x + overlay-width.
		skipCells := x + ansi.StringWidth(fgLine)
		right := ansi.TruncateLeft(bgLine, skipCells, "")
		b.WriteString(right)
	}

	return b.String()
}

// splitLines splits s on "\n" and returns the lines together with the
// display-cell width of the widest line.
func splitLines(s string) ([]string, int) {
	lines := strings.Split(s, "\n")
	widest := 0
	for _, l := range lines {
		if w := ansi.StringWidth(l); w > widest {
			widest = w
		}
	}
	return lines, widest
}

