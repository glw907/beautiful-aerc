package theme

import "charm.land/lipgloss/v2"

// calendarSlotHex holds the muted 8-hue cycle (design decision 6):
// accent, success, warn, and error reuse their named role's value;
// the remaining four hues have no other role and are given directly.
// The light row nudges each dark hue's lightness only, holding hue
// and saturation, to clear the 3:1 indicator floor against
// bgPanel's white (the theme task's sanctioned nudge, decision 6).
var calendarSlotHex = [2][8]string{
	{ // dark
		"85B3D1", "97BE8C", "D4B36A", "C99BC0",
		"7FBFB2", "DF8484", "B0A1E0", "C0A98F",
	},
	{ // light
		"285370", "3A5A32", "6A4E0A", "99538C",
		"3B756A", "90342E", "7358C8", "816749",
	},
}

// CalendarSlot returns the style for the i'th slot in the calendar's
// theme-assigned color cycle (CA-8), resolved against ground. The
// cycle has 8 hues and i wraps past it.
func (t Theme) CalendarSlot(i int, ground Ground) lipgloss.Style {
	row := 0
	if !t.isDark {
		row = 1
	}
	slot := i % len(calendarSlotHex[row])
	if slot < 0 {
		slot += len(calendarSlotHex[row])
	}
	return t.paint(lipgloss.NewStyle(), calendarSlotHex[row][slot], ground, false)
}
