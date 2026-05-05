package catkin

import "unicode/utf8"

// wordBounds returns the rune-offset bounds of the word that
// contains or is adjacent to cur. When cur sits in whitespace,
// start == end == cur.
func wordBounds(runes []rune, cur int) (int, int) {
	if cur > len(runes) {
		cur = len(runes)
	}
	start, end := cur, cur
	for start > 0 && isWordRune(runes[start-1]) {
		start--
	}
	for end < len(runes) && isWordRune(runes[end]) {
		end++
	}
	return start, end
}

// wrapWord wraps the word at cursor with marker on both sides.
// On whitespace the markers are inserted with the cursor placed
// between them.
func wrapWord(src string, cur int, marker string) (string, int) {
	runes := []rune(src)
	start, end := wordBounds(runes, cur)
	mlen := utf8.RuneCountInString(marker)
	if start == end {
		out := string(runes[:cur]) + marker + marker + string(runes[cur:])
		return out, cur + mlen
	}
	word := string(runes[start:end])
	out := string(runes[:start]) + marker + word + marker + string(runes[end:])
	return out, end + 2*mlen
}

// insertLinkSkeleton inserts "[](url)" at cur and lands the
// cursor between the brackets.
func insertLinkSkeleton(src string, cur int) (string, int) {
	runes := []rune(src)
	if cur > len(runes) {
		cur = len(runes)
	}
	const skeleton = "[](url)"
	out := string(runes[:cur]) + skeleton + string(runes[cur:])
	return out, cur + 1
}
