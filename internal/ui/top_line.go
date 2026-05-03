// SPDX-License-Identifier: MIT

package ui

import (
	"strings"
)

// TopLine renders the top frame edge: ──┬──╮.
type TopLine struct {
	styles Styles
}

// NewTopLine creates a TopLine with the given styles.
func NewTopLine(styles Styles) TopLine {
	return TopLine{styles: styles}
}

// View renders the top line at the given width. dividerCol is the
// column position of the panel divider (0 to skip the junction).
func (tl TopLine) View(width, dividerCol int) string {
	const rightEnd = "─╮"
	const rightEndWidth = 2

	fillWidth := width - rightEndWidth
	if fillWidth < 1 {
		fillWidth = 1
	}

	var buf strings.Builder
	for i := 0; i < fillWidth; i++ {
		if dividerCol > 0 && i == dividerCol {
			buf.WriteRune('┬')
		} else {
			buf.WriteRune('─')
		}
	}
	return tl.styles.TopLine.Render(buf.String() + rightEnd)
}
