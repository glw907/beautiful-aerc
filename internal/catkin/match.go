package catkin

import (
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// bracketMatchAt returns the rune offset on line of the delimiter
// matching the one under col. ok=false when col isn't on a known
// delimiter or no match is found on the same line.
func bracketMatchAt(line string, col int) (int, bool) {
	if col < 0 || col >= utf8.RuneCountInString(line) {
		return 0, false
	}
	for _, sp := range scanSpans(line) {
		if col >= sp.open && col < sp.open+sp.delim {
			return sp.close + (col - sp.open), true
		}
		if col >= sp.close && col < sp.close+sp.delim {
			return sp.open + (col - sp.close), true
		}
	}
	return 0, false
}

type spanPos struct {
	open, close int // rune offsets of opening / closing delim
	delim       int // delim length in runes (1 or 2)
}

// scanSpans walks line in tokenize order and records the rune
// positions of the open / close delimiters of each inline span:
// `…`, **…**, __…__, *…*, _…_, [text](url) (open=[, close=]).
func scanSpans(line string) []spanPos {
	var out []spanPos
	pos := 0
	walkSpans(line, func(kind spanKind, text string, sub []string) {
		n := utf8.RuneCountInString(text)
		switch kind {
		case spanCode, spanItalic:
			out = append(out, spanPos{pos, pos + n - 1, 1})
		case spanBold:
			out = append(out, spanPos{pos, pos + n - 2, 2})
		case spanBoldItalic:
			out = append(out, spanPos{pos, pos + n - 3, 3})
		case spanLink:
			textLen := utf8.RuneCountInString(sub[1])
			out = append(out, spanPos{pos, pos + textLen + 1, 1})
		}
		pos += n
	})
	return out
}

// overlayMatch re-renders displayCol in styled with style. The
// rune drawn is ch (the original delimiter). ANSI styling on the
// underlying line is replaced for that single cell.
func overlayMatch(styled string, displayCol int, ch rune, style lipgloss.Style) string {
	width := ansi.StringWidth(styled)
	if displayCol < 0 || displayCol >= width {
		return styled
	}
	left := ansi.Cut(styled, 0, displayCol)
	right := ansi.Cut(styled, displayCol+1, width)
	return left + style.Render(string(ch)) + right
}
