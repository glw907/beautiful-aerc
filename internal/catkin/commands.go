package catkin

import (
	"strings"
	"unicode/utf8"
)

// loc identifies a rune position by line index and rune column
// within that line.
type loc struct {
	line int
	col  int
}

// splitAtCursor returns the source split into lines and the cursor
// position as (line, col). col is a rune offset within the line.
func splitAtCursor(src string, cur int) ([]string, loc) {
	lines := strings.Split(src, "\n")
	pos := 0
	for i, l := range lines {
		n := utf8.RuneCountInString(l)
		if cur <= pos+n {
			return lines, loc{i, cur - pos}
		}
		pos += n + 1
	}
	last := len(lines) - 1
	return lines, loc{last, utf8.RuneCountInString(lines[last])}
}

// joinAt rebuilds source from lines and returns the rune offset
// corresponding to the (line, col) position.
func joinAt(lines []string, p loc) (string, int) {
	off := 0
	for i := range min(p.line, len(lines)) {
		off += utf8.RuneCountInString(lines[i]) + 1
	}
	if p.line < len(lines) {
		c := p.col
		ln := utf8.RuneCountInString(lines[p.line])
		if c > ln {
			c = ln
		}
		off += c
	}
	return strings.Join(lines, "\n"), off
}

// linePrefix returns the literal prefix string (quote runs +
// optional list/task marker with trailing space) for line under
// the given context. The classifier already counted the prefix in
// runes via PrefixWidth and stripped it into PostPrefix.
func linePrefix(line string, ctx LineContext) string {
	postRunes := utf8.RuneCountInString(ctx.PostPrefix)
	totalRunes := utf8.RuneCountInString(line)
	prefixRunes := totalRunes - postRunes
	if prefixRunes <= 0 {
		return ""
	}
	bytes := 0
	for range prefixRunes {
		_, size := utf8.DecodeRuneInString(line[bytes:])
		if size == 0 {
			break
		}
		bytes += size
	}
	return line[:bytes]
}

func classifyAt(lines []string, idx int) LineContext {
	ctxs := Classify(lines)
	if idx < 0 || idx >= len(ctxs) {
		return LineContext{}
	}
	return ctxs[idx]
}
