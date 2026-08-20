package theme

import "testing"

// declaredRoles enumerates every Role constant, so the completeness
// tests below can assert every one carries both a palette entry and
// a class (pass 2 review finding I2: nothing previously linked the
// Role enum to the palette or the contrast guard's lists, so an
// unlisted role would render unstyled text with no test failing).
var declaredRoles = []Role{
	RoleFg, RoleFgMuted, RoleFgSubtle, RoleAccent, RoleUnread,
	RoleError, RoleWarn, RoleSuccess, RoleLink, RoleQuote, RoleFlag,
	RoleDiffAdd, RoleDiffDel, RoleBorder, RoleFocusedBorder,
}

func TestRoleCompleteness(t *testing.T) {
	for _, role := range declaredRoles {
		if _, ok := roleClassOf[role]; !ok {
			t.Errorf("role %v: no roleClassOf entry", role)
		}
		if roleHex(role, true) == "" {
			t.Errorf("role %v: no dark palette entry", role)
		}
		if roleHex(role, false) == "" {
			t.Errorf("role %v: no light palette entry", role)
		}
	}
}

func TestGroundCompleteness(t *testing.T) {
	for _, g := range []Ground{GroundBase, GroundPanel, GroundSelected, GroundCode} {
		if groundHex(g, true) == "" {
			t.Errorf("ground %v: no dark palette entry", g)
		}
		if groundHex(g, false) == "" {
			t.Errorf("ground %v: no light palette entry", g)
		}
	}
}

// TestANSI16RoleSemantics asserts pass 2 review finding C1: named
// ANSI-16 slots, not a nearest-hex downsample, so error, warn, and
// success stay on three distinct ANSI colors and fg stays distinct
// from fgMuted, in both themes.
func TestANSI16RoleSemantics(t *testing.T) {
	for _, isDark := range []bool{true, false} {
		slots := map[Role]int{
			RoleError:   ansi16Slot(RoleError, isDark),
			RoleWarn:    ansi16Slot(RoleWarn, isDark),
			RoleSuccess: ansi16Slot(RoleSuccess, isDark),
		}
		seen := map[int]Role{}
		for role, slot := range slots {
			if other, ok := seen[slot]; ok {
				t.Errorf("dark=%v: role %v and %v share ANSI-16 slot %d", isDark, role, other, slot)
			}
			seen[slot] = role
		}
		if fg, muted := ansi16Slot(RoleFg, isDark), ansi16Slot(RoleFgMuted, isDark); fg == muted {
			t.Errorf("dark=%v: fg and fgMuted share ANSI-16 slot %d", isDark, fg)
		}
	}
}
