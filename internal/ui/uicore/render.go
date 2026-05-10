package uicore

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// PadOrTruncate pads s with spaces or truncates it to exactly width display cells.
func PadOrTruncate(s string, width int) string {
	w := lipgloss.Width(s)
	if w == width {
		return s
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return TruncateToWidth(s, width)
}

// TruncateToWidth shortens s to at most width display cells, adding "…"
// when truncation occurs. Cells are measured with lipgloss.Width and
// truncation respects rune boundaries.
func TruncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	const ellipsis = "…"
	if width == 1 {
		return ellipsis
	}
	limit := width - lipgloss.Width(ellipsis)
	out := make([]rune, 0, len(s))
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > limit {
			break
		}
		out = append(out, r)
		w += rw
	}
	return string(out) + ellipsis
}

// ClampScrollOffset returns offset adjusted so cursor lies within
// [offset, offset+visible).
func ClampScrollOffset(cursor, visible, offset int) int {
	if cursor < offset {
		return cursor
	}
	if cursor >= offset+visible {
		return cursor - visible + 1
	}
	return offset
}

// CenterOverlay returns the top-left cell coordinates that center box on a
// terminal of totalW × totalH cells.
func CenterOverlay(box string, totalW, totalH int) (int, int) {
	x := (totalW - lipgloss.Width(box)) / 2
	y := (totalH - lipgloss.Height(box)) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}
