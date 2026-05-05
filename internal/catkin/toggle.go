package catkin

import (
	"strings"
	"unicode/utf8"
)

// toggleList toggles the "- " list prefix on the current line.
func toggleList(src string, cur int) (string, int) {
	return togglePrefix(src, cur, "- ")
}

// toggleQuote toggles the "> " quote prefix on the current line.
func toggleQuote(src string, cur int) (string, int) {
	return togglePrefix(src, cur, "> ")
}

func togglePrefix(src string, cur int, prefix string) (string, int) {
	lines, p := splitAtCursor(src, cur)
	line := lines[p.line]
	plen := utf8.RuneCountInString(prefix)
	if strings.HasPrefix(line, prefix) {
		lines[p.line] = line[len(prefix):]
		col := p.col - plen
		if col < 0 {
			col = 0
		}
		return joinAt(lines, loc{p.line, col})
	}
	lines[p.line] = prefix + line
	return joinAt(lines, loc{p.line, p.col + plen})
}

// isTaskLine reports whether line is a task-list item at any
// indentation depth.
func isTaskLine(line string) bool {
	return taskItemRE.MatchString(strings.TrimLeft(line, " "))
}

// toggleTask flips a task box between "[ ]" and "[x]" on a task
// line. Returns handled=false on non-task lines so the keystroke
// passes through.
func toggleTask(src string, cur int) (string, int, bool) {
	lines, p := splitAtCursor(src, cur)
	line := lines[p.line]
	if !isTaskLine(line) {
		return src, cur, false
	}
	switch {
	case strings.Contains(line, "[ ]"):
		lines[p.line] = strings.Replace(line, "[ ]", "[x]", 1)
	case strings.Contains(line, "[x]"), strings.Contains(line, "[X]"):
		r := strings.NewReplacer("[x]", "[ ]", "[X]", "[ ]")
		lines[p.line] = r.Replace(line)
	default:
		return src, cur, false
	}
	out, off := joinAt(lines, p)
	return out, off, true
}
