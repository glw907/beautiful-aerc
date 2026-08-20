package theme

import (
	"image/color"

	"github.com/charmbracelet/colorprofile"
)

// paletteDark and paletteLight hold the hex values for the roles
// with a direct palette entry (design decision 6, the "cool slate,
// v3 values" table). RoleLink, RoleFlag, RoleDiffAdd, RoleDiffDel,
// and RoleFocusedBorder have no entry here; roleHex resolves them to
// another role's value.
var paletteDark = map[Role]string{
	RoleFg:       "D4D8DF",
	RoleFgMuted:  "969DA9",
	RoleFgSubtle: "7A8290",
	RoleAccent:   "85B3D1",
	RoleUnread:   "ECEEF2",
	RoleError:    "DF8484",
	RoleWarn:     "D4B36A",
	RoleSuccess:  "97BE8C",
	RoleQuote:    "A0A5AE",
	RoleBorder:   "3A414D",
}

var paletteLight = map[Role]string{
	RoleFg:       "262B33",
	RoleFgMuted:  "4C545F",
	RoleFgSubtle: "646D79",
	RoleAccent:   "285370",
	RoleUnread:   "12161C",
	RoleError:    "90342E",
	RoleWarn:     "6A4E0A",
	RoleSuccess:  "3A5A32",
	RoleQuote:    "4A525D",
	RoleBorder:   "B4BCC9",
}

var groundDark = map[Ground]string{
	GroundBase:     "16181D",
	GroundPanel:    "262B36",
	GroundSelected: "2A3441",
	GroundCode:     "1D2026",
}

var groundLight = map[Ground]string{
	GroundBase:     "E4E7EC",
	GroundPanel:    "FFFFFF",
	GroundSelected: "C4CDDB",
	GroundCode:     "D9DDE4",
}

// roleHex returns role's hex color for isDark, resolving the five
// alias roles to the entry they borrow (design decision 6: link is
// an accent alias, flag a warn alias, diffAdd/diffDel the
// success/error values, and focusedBorder repurposes accent as the
// degrade profiles' focused-divider color).
func roleHex(role Role, isDark bool) string {
	switch role {
	case RoleLink, RoleFocusedBorder:
		role = RoleAccent
	case RoleFlag:
		role = RoleWarn
	case RoleDiffAdd:
		role = RoleSuccess
	case RoleDiffDel:
		role = RoleError
	}
	if isDark {
		return paletteDark[role]
	}
	return paletteLight[role]
}

func groundHex(g Ground, isDark bool) string {
	if isDark {
		return groundDark[g]
	}
	return groundLight[g]
}

// ansi16Color downsamples h to its nearest 4-bit ANSI approximation,
// the ProfileANSI16 half of Style's profile switch.
func ansi16Color(h string) color.Color {
	return colorprofile.ANSI.Convert(hexColor(h))
}
