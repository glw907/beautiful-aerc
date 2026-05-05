package catkin

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// pairCloser maps the open delimiter of an auto-pair to its closer.
var pairCloser = map[rune]rune{
	'*': '*',
	'_': '_',
	'`': '`',
	'[': ']',
}

// handleAutoPair intercepts pair-trigger inserts and the
// backspace that deletes an empty pair. Pairing is suppressed
// inside code blocks and inline code spans.
func handleAutoPair(b Buffer, k tea.KeyMsg) (handled bool, _ Buffer, _ tea.Cmd) {
	if k.Paste {
		return false, b, nil
	}
	src := b.Value()
	cur := b.RuneOffset()

	if k.Type == tea.KeyBackspace {
		newSrc, newCur, ok := tryPairDelete(src, cur)
		if !ok {
			return false, b, nil
		}
		b.SetValue(newSrc)
		b.SetRuneOffset(newCur)
		return true, b, nil
	}

	if k.Type != tea.KeyRunes || len(k.Runes) != 1 {
		return false, b, nil
	}
	r := k.Runes[0]
	if _, isPair := pairCloser[r]; !isPair {
		return false, b, nil
	}
	if inCodeContext(src, cur) {
		return false, b, nil
	}
	newSrc, newCur := applyAutoPair(src, cur, r)
	b.SetValue(newSrc)
	b.SetRuneOffset(newCur)
	return true, b, nil
}

// applyAutoPair returns the buffer state after typing r at cur.
// Three behaviours: step over an existing closer, expand a
// single-char emphasis pair to double, or insert r and
// its closer with the cursor between.
func applyAutoPair(src string, cur int, r rune) (string, int) {
	runes := []rune(src)
	prev, next := neighbour(runes, cur)

	if next == r && (r == '*' || r == '_' || r == '`') {
		if prev == r && (r == '*' || r == '_') {
			left := string(runes[:cur-1])
			right := string(runes[cur+1:])
			pair := string([]rune{r, r})
			return left + pair + pair + right, cur + 1
		}
		return src, cur + 1
	}

	closer := pairCloser[r]
	out := make([]rune, 0, len(runes)+2)
	out = append(out, runes[:cur]...)
	out = append(out, r, closer)
	out = append(out, runes[cur:]...)
	return string(out), cur + 1
}

// tryPairDelete deletes a pair when cur sits between r and
// pairCloser[r]. ok=false on any other pattern.
func tryPairDelete(src string, cur int) (string, int, bool) {
	runes := []rune(src)
	if cur == 0 || cur >= len(runes) {
		return "", 0, false
	}
	prev, next := runes[cur-1], runes[cur]
	closer, ok := pairCloser[prev]
	if !ok || closer != next {
		return "", 0, false
	}
	out := append([]rune{}, runes[:cur-1]...)
	out = append(out, runes[cur+1:]...)
	return string(out), cur - 1, true
}

func neighbour(runes []rune, cur int) (prev, next rune) {
	if cur > 0 {
		prev = runes[cur-1]
	}
	if cur < len(runes) {
		next = runes[cur]
	}
	return prev, next
}

// inCodeContext reports whether cur sits inside a code block
// (fenced or indented) or inline code span on the current line.
func inCodeContext(src string, cur int) bool {
	lines := strings.Split(src, "\n")
	row, col := offsetToRowCol(src, cur)
	if row >= len(lines) {
		return false
	}
	ctxs := Classify(lines)
	switch ctxs[row].Kind {
	case BlockCodeFence, BlockCodeIndent:
		return true
	}
	return inInlineCode(lines[row], col)
}

// inInlineCode reports whether col sits inside a `…` span on
// line. Counts unescaped backticks up to col. An odd count means
// we're inside.
func inInlineCode(line string, col int) bool {
	n := 0
	i := 0
	for _, r := range line {
		if i >= col {
			break
		}
		if r == '`' {
			n++
		}
		i++
	}
	return n%2 == 1
}
