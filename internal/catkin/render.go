package catkin

import (
	"strings"
	"unicode/utf8"
)

// Render produces Catkin's view content: plain text + cursor block +
// display-level soft-wrap for source lines that exceed width. Pass 9
// applies no styling — that lands in 9a.
//
// width and height are the viewport dimensions in columns and rows.
// top is the source-line index of the first visible row; cursor is
// the rune offset of the cursor in src.
func Render(src string, width, height, top, cursor int) string {
	lines := strings.Split(src, "\n")
	cursorRow, cursorCol := offsetToRowCol(src, cursor)

	var visual []string
	for i := top; i < len(lines) && len(visual) < height; i++ {
		for _, w := range softWrap(lines[i], width) {
			if len(visual) >= height {
				break
			}
			visual = append(visual, w)
		}
	}

	if cursorRow >= top && cursorRow-top < len(visual) {
		visualRow, visualCol := mapToVisual(lines[cursorRow], cursorCol, width)
		row := (cursorRow - top) + visualRow
		if row < len(visual) {
			visual[row] = insertCursorBlock(visual[row], visualCol)
		}
	}

	for len(visual) < height {
		visual = append(visual, "")
	}

	return strings.Join(visual, "\n")
}

// softWrap breaks line into width-column chunks. Mid-token breaks are
// acceptable here: Reflow has already done token-aware wrapping on
// the source, so display overflow is a second-order edge.
func softWrap(line string, width int) []string {
	if width <= 0 || utf8.RuneCountInString(line) <= width {
		return []string{line}
	}
	var out []string
	runes := []rune(line)
	for len(runes) > width {
		out = append(out, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 {
		out = append(out, string(runes))
	}
	return out
}

func offsetToRowCol(src string, off int) (row, col int) {
	pos := 0
	for _, r := range src {
		if pos >= off {
			return row, col
		}
		if r == '\n' {
			row++
			col = 0
		} else {
			col++
		}
		pos++
	}
	return row, col
}

func mapToVisual(line string, srcCol, width int) (row, col int) {
	if width <= 0 {
		return 0, srcCol
	}
	return srcCol / width, srcCol % width
}

func insertCursorBlock(line string, col int) string {
	runes := []rune(line)
	if col >= len(runes) {
		return line + "█"
	}
	return string(runes[:col]) + "█" + string(runes[col+1:])
}
