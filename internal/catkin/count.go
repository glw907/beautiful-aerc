package catkin

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// WordCount returns the number of whitespace-delimited tokens in
// the buffer.
func (m Model) WordCount() int { return wordCount(m.buf.Value()) }

// CharCount returns the number of runes in the buffer.
func (m Model) CharCount() int { return utf8.RuneCountInString(m.buf.Value()) }

func wordCount(s string) int {
	return len(strings.FieldsFunc(s, unicode.IsSpace))
}
