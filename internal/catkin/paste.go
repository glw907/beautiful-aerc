package catkin

import "strings"

// urlWrapTarget reports whether payload is a URL-like token and the
// cursor sits inside a word, returning the word's rune boundaries.
// When ok is false the caller inserts payload literally.
func urlWrapTarget(runes []rune, cur int, payload string) (start, end int, ok bool) {
	if strings.ContainsAny(payload, " \t\n\r") {
		return 0, 0, false
	}
	if !strings.HasPrefix(payload, "http://") &&
		!strings.HasPrefix(payload, "https://") &&
		!strings.HasPrefix(payload, "mailto:") {
		return 0, 0, false
	}
	atRight := cur > 0 && isWordRune(runes[cur-1])
	atLeft := cur < len(runes) && isWordRune(runes[cur])
	if !atRight && !atLeft {
		return 0, 0, false
	}
	start = cur
	for start > 0 && isWordRune(runes[start-1]) {
		start--
	}
	end = cur
	for end < len(runes) && isWordRune(runes[end]) {
		end++
	}
	if start == end {
		return 0, 0, false
	}
	return start, end, true
}
