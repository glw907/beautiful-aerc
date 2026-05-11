package uicore

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/glw907/poplar/internal/ansix"
)

// ApplyBg layers the background of bgStyle onto base so a foreground style
// can pick up the row's background without clobbering already-rendered ANSI
// segments.
func ApplyBg(base, bgStyle lipgloss.Style) lipgloss.Style {
	if bg := bgStyle.GetBackground(); bg != (lipgloss.NoColor{}) {
		return base.Background(bg)
	}
	return base
}

// FillRowToWidth fits a fully-rendered row of ANSI segments to exactly
// width display cells. Short rows are right-padded with bgStyle so the row
// background extends to the panel edge. Over-wide rows are truncated.
func FillRowToWidth(m ansix.Measurer, row string, width int, bgStyle lipgloss.Style) string {
	rw := m.Width(row)
	if rw < width {
		return row + bgStyle.Render(strings.Repeat(" ", width-rw))
	}
	if rw > width {
		return m.Truncate(row, width)
	}
	return row
}
