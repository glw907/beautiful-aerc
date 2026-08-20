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

// calendarSlotANSI16 names each slot's ANSI-16 approximation
// directly, the same reasoning as ansi16SlotDark/Light: a
// nearest-hex downsample cannot be trusted to keep 8 slots
// distinguishable at 4-bit depth.
var calendarSlotANSI16 = [2][8]int{
	{12, 10, 11, 13, 14, 9, 5, 3}, // dark
	{4, 2, 3, 5, 6, 1, 5, 3},      // light
}

// CalendarSlot returns the style for the i'th slot in the calendar's
// theme-assigned color cycle (CA-8), resolved against ground. The
// cycle has 8 hues and i wraps past it.
func (t Theme) CalendarSlot(i int, ground Ground) lipgloss.Style {
	row := 0
	if !t.isDark {
		row = 1
	}
	slot := i % 8
	if slot < 0 {
		slot += 8
	}

	s := lipgloss.NewStyle()
	switch t.profile {
	case ProfileTrueColor:
		s = s.Foreground(hexColor(calendarSlotHex[row][slot]))
	case ProfileANSI16:
		s = s.Foreground(ansi16(calendarSlotANSI16[row][slot]))
	}
	return t.paintGround(s, ground)
}
