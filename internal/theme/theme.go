// Package theme compiles the poplar design language's tokens
// (docs/superpowers/specs/2026-07-27-poplar-design-language.md) as
// Go values: color roles, glyphs, spacing, type roles, border sets,
// and spinner frames. The UX-3 styling analyzer exempts this
// package and internal/catkin; every other package styles only
// through the accessors here, never a raw hex value or a direct
// lipgloss call.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Profile is the terminal's rendering capability: color depth, and,
// since poplar's two degrade profiles both assume a matching glyph
// constraint (design decision 3), Unicode glyph support. The
// runtime capability resolver (technical design section 12) selects
// one from NO_COLOR, TERM, COLORTERM, and the background-color
// query, and passes it to New.
type Profile int

// The three profiles the theme ships.
const (
	ProfileTrueColor Profile = iota
	ProfileANSI16
	ProfileNoColor
)

// Ground is one of the four backgrounds decision 6 verifies every
// role against: base content, elevated panel chrome, the
// accent-tinted selection, and the code inset. Style takes a Ground
// rather than a raw color so a caller can never pair a role with a
// background the contrast test never checked.
type Ground int

// The four verified grounds (design decision 6, decision 11's
// ground grammar).
const (
	GroundBase Ground = iota
	GroundPanel
	GroundSelected
	GroundCode
)

// Role is a color role in the design language's token vocabulary
// (design language section 7; decision 6's palette). RoleLink,
// RoleFlag, RoleDiffAdd, RoleDiffDel, and RoleFocusedBorder are
// role aliases: each resolves to another role's palette entry
// (decision 6) but keeps its own name so a caller states intent
// rather than borrowing an unrelated role.
type Role int

// The color roles, both plain palette entries and aliases.
const (
	RoleFg Role = iota
	RoleFgMuted
	RoleFgSubtle
	RoleAccent
	RoleUnread
	RoleError
	RoleWarn
	RoleSuccess
	RoleLink
	RoleQuote
	RoleFlag
	RoleDiffAdd
	RoleDiffDel
	RoleBorder
	RoleFocusedBorder
)

// Theme is poplar's compiled token set for one isDark/Profile pair.
type Theme struct {
	isDark  bool
	profile Profile
}

// New returns the theme for isDark and profile.
func New(isDark bool, profile Profile) Theme {
	return Theme{isDark: isDark, profile: profile}
}

// Style returns role's style resolved against ground. Role-intrinsic
// channels apply regardless of profile: RoleUnread is bold, RoleQuote
// is italic, RoleLink is underlined (decision 6's table). Under
// ProfileTrueColor, ground supplies a real background color; under
// ProfileANSI16 and ProfileNoColor the ground color drops in favor
// of the caller's own structural divider (decision 3's degrade
// substitution) except that GroundSelected and RoleError switch to
// reverse video instead, the design language's degrade-table
// channels for the selected and error states.
func (t Theme) Style(role Role, ground Ground) lipgloss.Style {
	s := lipgloss.NewStyle()
	switch role {
	case RoleUnread:
		s = s.Bold(true)
	case RoleQuote:
		s = s.Italic(true)
	case RoleLink:
		s = s.Underline(true)
	}
	return t.paint(s, roleHex(role, t.isDark), ground, role == RoleError)
}

// paint applies fgHex and ground to s per t's profile. forceReverse
// asks for reverse video regardless of ground, RoleError's half of
// the degrade table.
func (t Theme) paint(s lipgloss.Style, fgHex string, ground Ground, forceReverse bool) lipgloss.Style {
	switch t.profile {
	case ProfileTrueColor:
		return s.Foreground(hexColor(fgHex)).Background(hexColor(groundHex(ground, t.isDark)))
	case ProfileANSI16:
		s = s.Foreground(ansi16Color(fgHex))
	}
	if ground == GroundSelected || forceReverse {
		s = s.Reverse(true)
	}
	return s
}

func hexColor(h string) color.Color {
	return lipgloss.Color("#" + h)
}
