// Vendored from github.com/yorukot/superfile (MIT), originally
// src/pkg/string_function/overplace.go. Rewritten to use
// github.com/charmbracelet/x/ansi (already a poplar dependency) instead of
// muesli/reflow + muesli/ansi; the algorithm is unchanged.
//
// Cell-width measurements use ansi.StringWidth, which strips escape
// sequences before measuring and so matches the Truncate/TruncateLeft
// semantics. Overlay content is icon-free. Callers that pass icon-bearing
// strings should pre-measure with ansix.Width.
package uicore

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// PlaceOverlay composites fg over bg at zero-based cell offset (x, y),
// preserving background ANSI styling outside the overlay rect. An overlay
// extending past the right or bottom edge is clipped, not an error. If fg
// is larger than bg in both dimensions, fg is returned as-is.
func PlaceOverlay(x, y int, fg, bg string) string {
	fgLines, fgWidth := splitLines(fg)
	bgLines, bgWidth := splitLines(bg)
	bgHeight := len(bgLines)
	fgHeight := len(fgLines)

	if fgWidth >= bgWidth && fgHeight >= bgHeight {
		return fg
	}

	x = min(max(x, 0), max(0, bgWidth-fgWidth))
	y = min(max(y, 0), max(0, bgHeight-fgHeight))

	var b strings.Builder
	for i, bgLine := range bgLines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i < y || i >= y+fgHeight {
			b.WriteString(bgLine)
			continue
		}

		fgLine := fgLines[i-y]

		// ansi.Truncate cuts to x visible cells while preserving ANSI opens.
		left := ansi.Truncate(bgLine, x, "")
		leftWidth := ansi.StringWidth(left)
		b.WriteString(left)
		if leftWidth < x {
			b.WriteString(strings.Repeat(" ", x-leftWidth))
		}

		b.WriteString(fgLine)

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
