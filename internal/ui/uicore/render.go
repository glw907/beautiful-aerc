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

// SplitAndPad splits view on newlines and pads or truncates each row to width
// cells. Used by picker Box renderers to normalize list.View() output before
// passing it to ModalShell.Box.
func SplitAndPad(view string, width int) []string {
	rows := strings.Split(view, "\n")
	for i, row := range rows {
		rows[i] = PadOrTruncate(row, width)
	}
	return rows
}

// PickerListSize derives content width and list height for a modal picker box.
// boxW/boxH are the outer box dims. maxW caps the picker width; minW floors it.
// headerRows is the count of body rows reserved for title, footer, and borders,
// subtracted from boxH to give listH.
func PickerListSize(boxW, boxH, maxW, minW, headerRows int) (contentW, listH int) {
	bw := maxW
	if boxW-4 < bw {
		bw = boxW - 4
	}
	if bw < minW {
		bw = minW
	}
	contentW = bw - 2
	listH = boxH - headerRows
	if listH < 1 {
		listH = 1
	}
	return contentW, listH
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
