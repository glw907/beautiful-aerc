package catkin

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handlePaste turns a bracketed-paste KeyMsg whose payload is a
// bare URL into a markdown link wrapping the word at the cursor:
// `[word](url)`. Falls back to ordinary paste behaviour when the
// payload isn't a URL, the cursor isn't on a word, or the cursor
// sits in code context.
func handlePaste(b Buffer, k tea.KeyMsg) (handled bool, _ Buffer, _ tea.Cmd) {
	if !k.Paste || len(k.Runes) == 0 {
		return false, b, nil
	}
	pasted := string(k.Runes)
	if !looksLikeURL(pasted) {
		return false, b, nil
	}
	src := b.Value()
	cur := b.RuneOffset()
	if inCodeContext(src, cur) {
		return false, b, nil
	}
	start, end, ok := wordAt(src, cur)
	if !ok {
		return false, b, nil
	}
	runes := []rune(src)
	word := string(runes[start:end])
	link := "[" + word + "](" + pasted + ")"
	out := string(runes[:start]) + link + string(runes[end:])
	b.SetValue(out)
	b.SetRuneOffset(start + len([]rune(link)))
	return true, b, nil
}

// looksLikeURL applies VS Code's pragmatic test: a single
// whitespace-free token starting with http://, https://, or
// mailto:.
func looksLikeURL(s string) bool {
	s = strings.TrimSpace(s)
	if strings.ContainsAny(s, " \t\n\r") {
		return false
	}
	switch {
	case strings.HasPrefix(s, "http://"),
		strings.HasPrefix(s, "https://"),
		strings.HasPrefix(s, "mailto:"):
		return true
	}
	return false
}

// wordAt returns the rune span of the word containing or
// adjacent to cur. Returns ok=false when the cursor isn't on or
// touching word runes. The "current word" semantics differ from
// nextWordBoundary / prevWordBoundary, which jump past the
// surrounding whitespace — here the bounds are the word itself.
func wordAt(src string, cur int) (start, end int, ok bool) {
	runes := []rune(src)
	on := cur < len(runes) && isWordRune(runes[cur])
	leftOf := cur > 0 && isWordRune(runes[cur-1])
	if !on && !leftOf {
		return 0, 0, false
	}
	start = cur
	if !on {
		start = cur - 1
	}
	for start > 0 && isWordRune(runes[start-1]) {
		start--
	}
	end = cur
	for end < len(runes) && isWordRune(runes[end]) {
		end++
	}
	return start, end, end > start
}
