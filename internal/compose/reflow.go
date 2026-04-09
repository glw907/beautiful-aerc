package compose

import (
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
)

// quotePrefix extracts the leading "> > " style prefix from a line.
// Returns empty string if the line is not quoted.
func quotePrefix(line string) string {
	if len(line) == 0 || line[0] != '>' {
		return ""
	}
	i := 1
	for i < len(line) && (line[i] == '>' || line[i] == ' ') {
		i++
	}
	return line[:i]
}

// quoteDepth counts the number of '>' characters in a prefix.
func quoteDepth(prefix string) int {
	n := 0
	for _, c := range prefix {
		if c == '>' {
			n++
		}
	}
	return n
}

// canonicalPrefix builds a normalized prefix for a given depth:
// depth 1 = "> ", depth 2 = "> > ", etc.
func canonicalPrefix(depth int) string {
	if depth <= 0 {
		return ""
	}
	return strings.Repeat("> ", depth)
}

// hasAlphanumeric returns true if s contains at least one letter or digit.
func hasAlphanumeric(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// wrapText wraps text to fit within width columns when prepended with
// prefix. Breaks only at spaces. Returns a slice of lines, each
// starting with prefix.
func wrapText(text, prefix string, width int) []string {
	avail := width - runewidth.StringWidth(prefix)
	if avail <= 0 {
		return []string{prefix + text}
	}

	var result []string
	for runewidth.StringWidth(text) > avail {
		breakIdx := -1
		w := 0
		for i, r := range text {
			w += runewidth.RuneWidth(r)
			if w > avail {
				break
			}
			if r == ' ' {
				breakIdx = i
			}
		}
		if breakIdx < 0 {
			break // word exceeds available width, emit as-is
		}
		result = append(result, prefix+strings.TrimRight(text[:breakIdx+1], " "))
		text = strings.TrimLeft(text[breakIdx+1:], " ")
	}
	if len(text) > 0 {
		result = append(result, prefix+text)
	}
	return result
}

// reflowQuoted joins consecutive quoted lines at the same depth into
// paragraphs and re-wraps them at 72 columns. Blank quoted lines and
// decorative lines (no alphanumeric content) are preserved as breaks.
// Unquoted lines pass through unchanged.
func reflowQuoted(body []string) []string {
	var result []string
	i := 0
	for i < len(body) {
		prefix := quotePrefix(body[i])
		if prefix == "" {
			result = append(result, body[i])
			i++
			continue
		}

		depth := quoteDepth(prefix)
		canon := canonicalPrefix(depth)
		text := strings.TrimSpace(body[i][len(prefix):])

		// Blank quoted line — preserve as paragraph break
		if text == "" {
			result = append(result, strings.TrimRight(canon, " "))
			i++
			continue
		}

		// Decorative line (no letters or digits) — preserve as-is
		if !hasAlphanumeric(text) {
			avail := maxWidth - runewidth.StringWidth(canon)
			if runewidth.StringWidth(text) > avail {
				text = runewidth.Truncate(text, avail, "")
			}
			result = append(result, canon+text)
			i++
			continue
		}

		// Join consecutive lines at the same quote depth
		j := i + 1
		for j < len(body) {
			np := quotePrefix(body[j])
			if quoteDepth(np) != depth {
				break
			}
			nt := strings.TrimSpace(body[j][len(np):])
			if nt == "" || !hasAlphanumeric(nt) {
				break
			}
			text = strings.TrimRight(text, " ") + " " + strings.TrimLeft(nt, " ")
			j++
		}

		text = strings.TrimSpace(text)
		wrapped := wrapText(text, canon, maxWidth)
		result = append(result, wrapped...)
		i = j
	}
	return result
}
