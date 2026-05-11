package catkin

import (
	"unicode"

	tea "charm.land/bubbletea/v2"
)

// nextWordBoundary returns the rune offset of the start of the next
// word at or after off. Returns the rune count of src when off is
// past the last word.
func nextWordBoundary(src string, off int) int {
	runes := []rune(src)
	i := off
	for i < len(runes) && isWordRune(runes[i]) {
		i++
	}
	for i < len(runes) && !isWordRune(runes[i]) {
		i++
	}
	return i
}

// prevWordBoundary returns the rune offset of the start of the
// previous word strictly before off.
func prevWordBoundary(src string, off int) int {
	runes := []rune(src)
	if off <= 0 {
		return 0
	}
	i := off - 1
	for i > 0 && !isWordRune(runes[i]) {
		i--
	}
	for i > 0 && isWordRune(runes[i-1]) {
		i--
	}
	return i
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// handleWordNav intercepts Ctrl+Left/Right/Backspace/Delete.
// Returns (handled, updated buffer, cmd). Caller passes through to
// the buffer's normal Update when handled is false.
func handleWordNav(b Buffer, msg tea.KeyPressMsg) (bool, Buffer, tea.Cmd) {
	if msg.Mod&tea.ModAlt != 0 {
		return false, b, nil
	}
	switch msg.String() {
	case "ctrl+right":
		next := nextWordBoundary(b.Value(), b.RuneOffset())
		b = b.WithRuneOffset(next)
		return true, b, nil
	case "ctrl+left":
		prev := prevWordBoundary(b.Value(), b.RuneOffset())
		b = b.WithRuneOffset(prev)
		return true, b, nil
	case "ctrl+backspace":
		off := b.RuneOffset()
		prev := prevWordBoundary(b.Value(), off)
		runes := []rune(b.Value())
		b = b.WithValue(string(runes[:prev]) + string(runes[off:])).WithRuneOffset(prev)
		return true, b, nil
	case "ctrl+delete":
		off := b.RuneOffset()
		next := nextWordBoundary(b.Value(), off)
		runes := []rune(b.Value())
		b = b.WithValue(string(runes[:off]) + string(runes[next:])).WithRuneOffset(off)
		return true, b, nil
	}
	return false, b, nil
}
