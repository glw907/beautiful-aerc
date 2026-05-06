package catkin

import (
	"strings"
	"unicode/utf8"
)

// isListLine reports whether line is a list item at any
// indentation depth. The classifier only recognises top-level
// lists. Nested-list lines look like indented paragraphs from its
// view, so the indent commands check shape directly.
func isListLine(line string) bool {
	trimmed := strings.TrimLeft(line, " ")
	if trimmed == line && line == "" {
		return false
	}
	return taskItemRE.MatchString(trimmed) ||
		dashListRE.MatchString(trimmed) ||
		orderedListRE.MatchString(trimmed)
}

// indentTab handles the Tab key. On a list/task line it deepens
// the list nesting by prepending two spaces to the line. On any
// other line it inserts two spaces at the cursor.
func indentTab(src string, cur int) (string, int) {
	lines, p := splitAtCursor(src, cur)
	if isListLine(lines[p.line]) {
		lines[p.line] = "  " + lines[p.line]
		return joinAt(lines, loc{p.line, p.col + 2})
	}
	rs := []rune(lines[p.line])
	col := p.col
	if col > len(rs) {
		col = len(rs)
	}
	lines[p.line] = string(rs[:col]) + "  " + string(rs[col:])
	return joinAt(lines, loc{p.line, col + 2})
}

// indentShiftTab handles Shift+Tab. On a list/task line with at
// least two leading spaces it strips them (outdent). Otherwise
// returns the input unchanged with handled=false so compose.Model
// can claim the keystroke for focus routing at body (0, 0).
func indentShiftTab(src string, cur int) (newSrc string, newCur int, handled bool) {
	lines, p := splitAtCursor(src, cur)
	if !isListLine(lines[p.line]) {
		return src, cur, false
	}
	if !strings.HasPrefix(lines[p.line], "  ") {
		return src, cur, false
	}
	lines[p.line] = lines[p.line][2:]
	col := p.col - 2
	if col < 0 {
		col = 0
	}
	if col > utf8.RuneCountInString(lines[p.line]) {
		col = utf8.RuneCountInString(lines[p.line])
	}
	out, off := joinAt(lines, loc{p.line, col})
	return out, off, true
}
