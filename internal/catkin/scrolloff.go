package catkin

import "strings"

const scrollOff = 3

// ClampViewport returns a viewport top that keeps cursorLine within
// the scroll-off band. Returns 0 when total fits in height.
func ClampViewport(top, height, cursorLine, total int) int {
	if total <= height {
		return 0
	}
	rel := cursorLine - top
	switch {
	case rel < scrollOff:
		top = cursorLine - scrollOff
	case rel >= height-scrollOff:
		top = cursorLine - (height - scrollOff - 1)
	}
	if top < 0 {
		top = 0
	}
	if top > total-height {
		top = total - height
	}
	return top
}

func applyScrollOff(m Model) Model {
	src := m.buf.Value()
	cur := m.buf.RuneOffset()
	cursorLine, _ := offsetToRowCol(src, cur)
	total := lineCount(src)
	if m.mode.typewriter() && total > m.height && m.height > 0 {
		m.viewportTop = clampViewportTypewriter(m.height, cursorLine, total)
		return m
	}
	m.viewportTop = ClampViewport(m.viewportTop, m.height, cursorLine, total)
	return m
}

// clampViewportTypewriter holds the cursor at the vertical
// midpoint of height, clamped to the document range.
func clampViewportTypewriter(height, cursorLine, total int) int {
	top := cursorLine - height/2
	if top < 0 {
		top = 0
	}
	if top > total-height {
		top = total - height
	}
	return top
}

func lineCount(s string) int {
	return strings.Count(s, "\n") + 1
}
