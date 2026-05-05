package catkin

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// smartEnter inserts a newline at cur and continues the current
// line's quote/list/task prefix on the new line. On a prefix-only
// line, the prefix is stripped instead (terminating the block).
// Trailing-whitespace cleanup runs on the line being left, except
// for an exact two-space hard-break sequence.
func smartEnter(src string, cur int) (string, int) {
	lines, p := splitAtCursor(src, cur)
	ctx := classifyAt(lines, p.line)

	line := lines[p.line]
	if ctx.InsideFence {
		rs := []rune(line)
		col := p.col
		if col > len(rs) {
			col = len(rs)
		}
		out := make([]string, 0, len(lines)+1)
		out = append(out, lines[:p.line]...)
		out = append(out, string(rs[:col]))
		out = append(out, string(rs[col:]))
		out = append(out, lines[p.line+1:]...)
		return joinAt(out, loc{p.line + 1, 0})
	}

	prefix := linePrefix(line, ctx)
	body := strings.TrimPrefix(line, prefix)

	if prefix != "" && strings.TrimSpace(body) == "" {
		lines[p.line] = ""
		return joinAt(lines, loc{p.line, 0})
	}

	rs := []rune(line)
	col := p.col
	if col > len(rs) {
		col = len(rs)
	}
	left := string(rs[:col])
	right := string(rs[col:])
	left = trimTrailingPreservingHardBreak(left)

	cont := continuationPrefix(prefix, ctx)

	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:p.line]...)
	out = append(out, left)
	out = append(out, cont+right)
	out = append(out, lines[p.line+1:]...)
	return joinAt(out, loc{p.line + 1, utf8.RuneCountInString(cont)})
}

func continuationPrefix(prefix string, ctx LineContext) string {
	if prefix == "" {
		return ""
	}
	if ctx.Kind == BlockListItem && strings.HasSuffix(ctx.ListMarker, ".") {
		return incrementOrdered(prefix, ctx.ListMarker)
	}
	return prefix
}

func incrementOrdered(prefix, marker string) string {
	idx := strings.LastIndex(prefix, marker)
	if idx < 0 {
		return prefix
	}
	n, err := strconv.Atoi(strings.TrimSuffix(marker, "."))
	if err != nil {
		return prefix
	}
	next := fmt.Sprintf("%d.", n+1)
	return prefix[:idx] + next + prefix[idx+len(marker):]
}

// trimTrailingPreservingHardBreak strips trailing ASCII spaces
// from s, but preserves a CommonMark hard-break (exactly two
// trailing spaces).
func trimTrailingPreservingHardBreak(s string) string {
	n := 0
	for i := len(s) - 1; i >= 0 && s[i] == ' '; i-- {
		n++
	}
	if n == 0 {
		return s
	}
	body := s[:len(s)-n]
	if n == 2 {
		return body + "  "
	}
	return body
}
