package theme

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

// ansi16SlotDark and ansi16SlotLight name each role's ANSI-16 (4-bit)
// slot directly, one per named role (pass 2 review finding C1): a
// nearest-hex downsample collapses distinct roles onto the same
// slot (warn and error both land on bright red in the dark theme,
// for one measured case), which loses the role semantics the
// truecolor palette carries. Naming the slot keeps error, warn, and
// success on three distinct ANSI colors instead.
var ansi16SlotDark = map[Role]int{
	RoleFg:       7,
	RoleFgMuted:  8,
	RoleFgSubtle: 8,
	RoleAccent:   12,
	RoleUnread:   15,
	RoleError:    9,
	RoleWarn:     11,
	RoleSuccess:  10,
	RoleQuote:    7,
	RoleBorder:   8,
}

var ansi16SlotLight = map[Role]int{
	RoleFg:       0,
	RoleFgMuted:  8,
	RoleFgSubtle: 8,
	RoleAccent:   4,
	RoleUnread:   0,
	RoleError:    1,
	RoleWarn:     3,
	RoleSuccess:  2,
	RoleQuote:    8,
	RoleBorder:   8,
}

// roleClass groups a Role by the contrast floor it must clear
// (design decision 5): classText roles hold 4.5:1, classIndicator
// roles 3:1, and classStructural is border's contrast-exempt
// class.
type roleClass int

const (
	classText roleClass = iota
	classIndicator
	classStructural
)

// roleClassOf classifies every declared Role (design decision 5's
// partition), so a role that gains no palette entry or no class
// fails TestRoleCompleteness rather than rendering unstyled text
// silently (pass 2 review finding I2).
var roleClassOf = map[Role]roleClass{
	RoleFg:            classText,
	RoleFgMuted:       classText,
	RoleUnread:        classText,
	RoleError:         classText,
	RoleWarn:          classText,
	RoleSuccess:       classText,
	RoleLink:          classText,
	RoleQuote:         classText,
	RoleAccent:        classText,
	RoleFgSubtle:      classIndicator,
	RoleFocusedBorder: classIndicator,
	RoleFlag:          classIndicator,
	RoleDiffAdd:       classIndicator,
	RoleDiffDel:       classIndicator,
	RoleBorder:        classStructural,
}

// baseRole resolves role's five aliases (design decision 6: link is
// an accent alias, flag a warn alias, diffAdd/diffDel the
// success/error values, and focusedBorder repurposes accent as the
// degrade profiles' focused-divider color) to the role whose palette
// and ANSI-16 slot they borrow. A non-alias role resolves to itself.
func baseRole(role Role) Role {
	switch role {
	case RoleLink, RoleFocusedBorder:
		return RoleAccent
	case RoleFlag:
		return RoleWarn
	case RoleDiffAdd:
		return RoleSuccess
	case RoleDiffDel:
		return RoleError
	default:
		return role
	}
}

func roleHex(role Role, isDark bool) string {
	role = baseRole(role)
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

// GroundHex returns g's background hex color at isDark, with no
// leading "#": the same table paintGround resolves a Style's
// background from. It exists for a caller outside this package that
// needs to invert a rendered frame's SGR background back to the
// Ground that painted it (internal/ui's gallery ground-map sidecar),
// rather than a second, hand-copied palette.
func GroundHex(g Ground, isDark bool) string {
	return groundHex(g, isDark)
}

func ansi16Slot(role Role, isDark bool) int {
	role = baseRole(role)
	if isDark {
		return ansi16SlotDark[role]
	}
	return ansi16SlotLight[role]
}
