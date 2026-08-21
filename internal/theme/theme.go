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
	"strconv"
	"strings"

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
// (decision 6) but keeps its name so a caller states intent
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

// DrawsDividers reports whether ground steps stop carrying pane
// separation and a caller must draw the structural divider glyph
// instead (design decision 3): true at ProfileANSI16 and
// ProfileNoColor, false at ProfileTrueColor.
func (t Theme) DrawsDividers() bool {
	return t.profile != ProfileTrueColor
}

// Style returns role's style resolved against ground. Role-intrinsic
// channels apply regardless of profile: RoleUnread is bold, RoleQuote
// is italic, RoleLink is underlined (decision 6's table). At
// ProfileANSI16 and ProfileNoColor, RoleFgMuted and RoleFgSubtle also
// gain Faint, the non-color expression of "dim" the design language's
// type roles name. Foreground resolves per profile: a true hex color
// at ProfileTrueColor, an explicit ANSI-16 slot at ProfileANSI16 (a
// nearest-match downsample collapses distinct roles onto the same
// slot, so the palette names one directly per role), and no color at
// all at ProfileNoColor. Background and the selection/reverse channel
// come from paintGround.
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
	if t.profile != ProfileTrueColor {
		switch role {
		case RoleFgMuted, RoleFgSubtle:
			s = s.Faint(true)
		}
	}
	switch t.profile {
	case ProfileTrueColor:
		s = s.Foreground(hexColor(roleHex(role, t.isDark)))
	case ProfileANSI16:
		s = s.Foreground(ansi16(ansi16Slot(role, t.isDark)))
	}
	return t.paintGround(s, ground)
}

// EmphasizeRole returns role's style resolved against ground with
// Bold added, the seam a caller reaches for instead of calling
// lipgloss's Bold on a Style Style already returned (the UX-3
// styling analyzer's blind spot on a method call chained off a
// theme-returned value; this method is the honest route around it
// rather than a caller taking the shortcut itself): the status line's
// active-surface cluster label is its first caller
// (task-6-findings-r1.md F5).
func (t Theme) EmphasizeRole(role Role, ground Ground) lipgloss.Style {
	return t.Style(role, ground).Bold(true)
}

// paintGround applies ground's background at ProfileTrueColor. At
// ProfileANSI16 and ProfileNoColor, where the ground steps that
// carry pane and selection separation at full color are not
// reliably visible, GroundSelected switches to reverse video
// instead: selection's exclusive non-color channel (design language
// degrade table, ruling I1 of the pass 2 review). No other role or
// state adds reverse, so a role's Style never collides with
// selection's cue, and ProfileTrueColor never uses reverse at all
// since the background color already carries the cue.
func (t Theme) paintGround(s lipgloss.Style, ground Ground) lipgloss.Style {
	if t.profile == ProfileTrueColor {
		return s.Background(hexColor(groundHex(ground, t.isDark)))
	}
	if ground == GroundSelected {
		s = s.Reverse(true)
	}
	return s
}

func hexColor(h string) color.Color {
	return lipgloss.Color("#" + h)
}

// Blank returns ground's background painted across width×height
// blank cells: a chrome band or pane the render seam has no content
// for yet (decision 11's grounds-tile-first rule: Main and its bands
// paint before a pane's content lands on top). It carries no
// foreground role, since a ground alone never implies a text color.
func (t Theme) Blank(ground Ground, width, height int) string {
	return t.paintGround(lipgloss.NewStyle(), ground).Render(blankBlock(width, height))
}

func blankBlock(width, height int) string {
	row := strings.Repeat(" ", width)
	rows := make([]string, height)
	for i := range rows {
		rows[i] = row
	}
	return strings.Join(rows, "\n")
}

// Center centers s's content both horizontally and vertically within
// whatever Width and Height s already carries: the one place
// lipgloss's Center alignment constant is named, so a caller
// composing a centered block (the placeholder composition, the
// floor state's notice) never imports lipgloss itself.
func (t Theme) Center(s lipgloss.Style) lipgloss.Style {
	return s.Align(lipgloss.Center, lipgloss.Center)
}

// ansi16 returns the lipgloss ANSI-16 color for slot (0-15).
func ansi16(slot int) color.Color {
	return lipgloss.Color(strconv.Itoa(slot))
}
