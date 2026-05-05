package catkin

const scrollOff = 3

// ClampViewport returns a new viewport top so the cursor stays at
// least scrollOff lines from the top and bottom edges of the visible
// region. When total <= height the document fits entirely, so top
// clamps to 0.
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
	m.viewportTop = ClampViewport(m.viewportTop, m.height, cursorLine, total)
	return m
}

func lineCount(s string) int {
	if s == "" {
		return 1
	}
	n := 1
	for _, r := range s {
		if r == '\n' {
			n++
		}
	}
	return n
}
