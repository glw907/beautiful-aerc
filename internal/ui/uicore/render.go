package uicore

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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

// TruncateToWidth shortens s to at most width display cells, adding
// "…" when truncation occurs. Splits on rune boundaries, never
// inside a multi-byte glyph. Counts cells via lipgloss.Width so
// double-width CJK characters are accounted for correctly.
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
// [offset, offset+visible). Used by overlay scroll-window logic.
func ClampScrollOffset(cursor, visible, offset int) int {
	if cursor < offset {
		return cursor
	}
	if cursor >= offset+visible {
		return cursor - visible + 1
	}
	return offset
}

// CenterOverlay returns the top-left (x, y) cell coordinates that
// center box on a terminal of totalW × totalH cells. Used by
// overlay components to feed PlaceOverlay.
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
