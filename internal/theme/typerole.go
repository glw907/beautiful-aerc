package theme

import "charm.land/lipgloss/v2"

// TypeRole is a type role from the design language's component
// vocabulary (section 7): an emphasis choice layered over a color
// role, not a per-screen decision.
type TypeRole int

// The four type roles.
const (
	TypeTitle TypeRole = iota // emTitle: bold
	TypeLabel                 // emLabel: dim
	TypeValue                 // emValue: normal
	TypeHint                  // emHint: dim italic (decision 12's mail-row snippet)
)

// TypeStyle returns role's style resolved against ground.
func (t Theme) TypeStyle(role TypeRole, ground Ground) lipgloss.Style {
	switch role {
	case TypeTitle:
		return t.Style(RoleFg, ground).Bold(true)
	case TypeLabel:
		return t.Style(RoleFgMuted, ground)
	case TypeHint:
		return t.Style(RoleFgSubtle, ground).Italic(true)
	default:
		return t.Style(RoleFg, ground)
	}
}
