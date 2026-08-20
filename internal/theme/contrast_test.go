package theme

import (
	"math"
	"strconv"
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
			got := contrastRatio(t, fgHex, groundHex(g, isDark))
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

// groundPairs names every unordered pair among the four grounds, for
// TestGroundsPairwiseDistinct's per-pair floor.
var groundPairs = []struct {
	a, b Ground
	name string
}{
	{GroundBase, GroundPanel, "base-panel"},
	{GroundBase, GroundSelected, "base-selected"},
	{GroundBase, GroundCode, "base-code"},
	{GroundPanel, GroundSelected, "panel-selected"},
	{GroundPanel, GroundCode, "panel-code"},
	{GroundSelected, GroundCode, "selected-code"},
}

// minGroundRatio is a low floor asserting only that two grounds are
// perceptibly different steps, not a text/indicator contrast floor;
// today's palette clears it with the worst pair (dark base-code) at
// 1.09.
const minGroundRatio = 1.08

// TestGroundsPairwiseDistinct asserts every pair of grounds clears
// minGroundRatio in both themes (pass 2 review finding I3: the prior
// guard checked only the dark theme and only that grounds were not
// bitwise identical, which a light theme collision would have
// passed).
func TestGroundsPairwiseDistinct(t *testing.T) {
	for _, isDark := range []bool{true, false} {
		for _, p := range groundPairs {
			got := contrastRatio(t, groundHex(p.a, isDark), groundHex(p.b, isDark))
			if got < minGroundRatio {
				t.Errorf("dark=%v %s: ratio %.3f, floor %.3f", isDark, p.name, got, minGroundRatio)
			}
		}
	}
}

// contrastRatio computes the WCAG contrast ratio between two hex
// colors (no leading '#'), failing the test via t on a malformed
// input rather than silently mis-parsing it.
func contrastRatio(t *testing.T, a, b string) float64 {
	t.Helper()
	la, lb := relativeLuminance(t, a), relativeLuminance(t, b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func relativeLuminance(t *testing.T, h string) float64 {
	t.Helper()
	r, g, b := hexRGB(t, h)
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

// hexRGB parses h, a 6-digit hex color with no leading '#', into its
// 8-bit channels. It fails the test via t.Fatalf on a wrong-length or
// non-hex input rather than the silent mis-parse the pass 2 review
// found (finding I4): the old digit-by-digit scanner treated an
// unrecognized byte as 0 instead of rejecting it, so a stray '#' or
// short string produced a wrong color rather than an error.
func hexRGB(t *testing.T, h string) (r, g, b float64) {
	t.Helper()
	if len(h) != 6 {
		t.Fatalf("hexRGB: %q is not 6 hex digits", h)
	}
	parse := func(s string) float64 {
		v, err := strconv.ParseUint(s, 16, 8)
		if err != nil {
			t.Fatalf("hexRGB: %q: %v", h, err)
		}
		return float64(v)
	}
	return parse(h[0:2]), parse(h[2:4]), parse(h[4:6])
}
