package catkin

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const scrollOff = 3

// ClampViewport returns a viewport top (in visual rows) that keeps
// cursorVisual within the scroll-off band of a height-tall viewport
// when the document totals totalVisual visual rows. Returns 0 when the
// document fits in height.
func ClampViewport(top, height, cursorVisual, totalVisual int) int {
	if totalVisual <= height {
		return 0
	}
	rel := cursorVisual - top
	switch {
	case rel < scrollOff:
		top = cursorVisual - scrollOff
	case rel >= height-scrollOff:
		top = cursorVisual - (height - scrollOff - 1)
	}
	if top < 0 {
		top = 0
	}
	if top > totalVisual-height {
		top = totalVisual - height
	}
	return top
}

// cursorVisualRow returns the cursor's visual-row index and the total
// visual rows for the document at width. Quoted lines use the same
// budget arithmetic as visualWrap.
func cursorVisualRow(lines []string, ctxs []LineContext, width, cursorRow, cursorCol int) (int, int) {
	if width <= 0 {
		return cursorRow, len(lines)
	}
	total := 0
	cursorVR := 0
	for i, line := range lines {
		ctx := LineContext{}
		if i < len(ctxs) {
			ctx = ctxs[i]
		}
		h := visualHeight(line, width, ctx)
		if i == cursorRow {
			cursorVR = total + visualOffsetInLine(line, width, ctx, cursorCol)
		}
		total += h
	}
	return cursorVR, total
}

// visualHeight returns the number of visual rows a source line occupies
// at width, mirroring visualWrap's budget arithmetic.
func visualHeight(line string, width int, ctx LineContext) int {
	if width <= 0 {
		return 1
	}
	if ctx.QuoteDepth > 0 && !ctx.InsideFence {
		budget := width - 2*ctx.QuoteDepth
		if budget < 1 {
			budget = 1
		}
		prefix := strings.Repeat("> ", ctx.QuoteDepth)
		body := strings.TrimPrefix(line, prefix)
		if lipgloss.Width(body) <= budget {
			return 1
		}
		return strings.Count(ansi.Wrap(body, budget, ""), "\n") + 1
	}
	if lipgloss.Width(line) <= width {
		return 1
	}
	return strings.Count(ansi.Wrap(line, width, ""), "\n") + 1
}

// visualOffsetInLine returns the cursor's visual-row offset within its
// source line: 0 for the first visual row, 1 for the second, etc.
func visualOffsetInLine(line string, width int, ctx LineContext, col int) int {
	if width <= 0 || col <= 0 {
		return 0
	}
	budget := width
	skip := 0
	if ctx.QuoteDepth > 0 && !ctx.InsideFence {
		budget = width - 2*ctx.QuoteDepth
		if budget < 1 {
			budget = 1
		}
		skip = 2 * ctx.QuoteDepth
	}
	if col <= skip {
		return 0
	}
	body := []rune(strings.TrimPrefix(line, strings.Repeat("> ", ctx.QuoteDepth)))
	consumed := col - skip
	if consumed > len(body) {
		consumed = len(body)
	}
	if consumed <= 0 {
		return 0
	}
	prefix := string(body[:consumed])
	if lipgloss.Width(prefix) <= budget {
		return 0
	}
	return strings.Count(ansi.Wrap(prefix, budget, ""), "\n")
}

func applyScrollOff(m Model) Model {
	src := m.buf.Value()
	cur := m.buf.RuneOffset()
	lines := strings.Split(src, "\n")
	ctxs := Classify(lines)
	cursorRow, cursorCol := offsetToRowCol(src, cur)
	cursorVR, totalVR := cursorVisualRow(lines, ctxs, m.width, cursorRow, cursorCol)
	if m.mode.typewriter() && totalVR > m.height && m.height > 0 {
		m.viewportTop = clampViewportTypewriter(m.height, cursorVR, totalVR)
		return m
	}
	m.viewportTop = ClampViewport(m.viewportTop, m.height, cursorVR, totalVR)
	return m
}

// clampViewportTypewriter centers the cursor row in height, clamped to
// the document range.
func clampViewportTypewriter(height, cursorVR, totalVR int) int {
	top := cursorVR - height/2
	if top < 0 {
		top = 0
	}
	if top > totalVR-height {
		top = totalVR - height
	}
	return top
}
