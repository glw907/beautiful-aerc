package theme

import (
	"math"
	"testing"
)

// TestContrastFloors asserts UX-7: every text role clears 4.5:1 and
// every indicator role clears 3:1 against all four grounds, both
// themes, over the compiled palette (design decisions 5 and 6).
// RoleBorder is deliberately excluded: decision 6 names it
// structural and sub-contrast on purpose.
func TestContrastFloors(t *testing.T) {
	textRoles := []Role{RoleFg, RoleFgMuted, RoleUnread, RoleError, RoleWarn, RoleSuccess, RoleLink, RoleQuote, RoleAccent}
	indicatorRoles := []Role{RoleFgSubtle, RoleFocusedBorder, RoleFlag, RoleDiffAdd, RoleDiffDel}
	grounds := []Ground{GroundBase, GroundPanel, GroundSelected, GroundCode}

	check := func(t *testing.T, label, fgHex string, isDark bool, need float64) {
		for _, g := range grounds {
			got := contrastRatio(fgHex, groundHex(g, isDark))
			if got < need {
				t.Errorf("%s dark=%v ground=%v: ratio %.2f, need %.2f", label, isDark, g, got, need)
			}
		}
	}

	for _, isDark := range []bool{true, false} {
		for _, role := range textRoles {
			check(t, "text role", roleHex(role, isDark), isDark, 4.5)
		}
		for _, role := range indicatorRoles {
			check(t, "indicator role", roleHex(role, isDark), isDark, 3.0)
		}
		for slot := range 8 {
			row := 0
			if !isDark {
				row = 1
			}
			check(t, "calendarSlot", calendarSlotHex[row][slot], isDark, 3.0)
		}
	}
}

// contrastRatio computes the WCAG contrast ratio between two hex
// colors (no leading '#').
func contrastRatio(a, b string) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func relativeLuminance(h string) float64 {
	r, g, b := hexRGB(h)
	return 0.2126*linearize(r) + 0.7152*linearize(g) + 0.0722*linearize(b)
}

// linearize converts an 8-bit sRGB channel to its linear-light value
// (WCAG 2.x's relative luminance formula).
func linearize(c float64) float64 {
	c /= 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func hexRGB(h string) (r, g, b float64) {
	v := func(i int) float64 {
		n := 0
		for _, c := range h[i : i+2] {
			n *= 16
			switch {
			case c >= '0' && c <= '9':
				n += int(c - '0')
			case c >= 'A' && c <= 'F':
				n += int(c-'A') + 10
			case c >= 'a' && c <= 'f':
				n += int(c-'a') + 10
			}
		}
		return float64(n)
	}
	return v(0), v(2), v(4)
}
