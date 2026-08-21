package theme

// GlyphSet is the theme's compiled glyph tokens (design language
// section 7; decision 3 adds Dismiss through Divider), resolved to
// the profile's fallback by Glyphs. Divider serves two uses that
// share one glyph and one RoleBorder styling: the structural
// pane-separator line ANSI-16/NO_COLOR draw in place of a ground
// step (DrawsDividers), and the reader's quote-bar prefix. WarnMarker
// is pass 2 task 8's addition (the banner's leading glyph), added
// under decision 3's allowance for a design-language token the
// build discovers it needs; its fallback is "^", not "!", since "!"
// collided with ErrorGutter's fallback exactly where color cannot
// tell the two apart (task-8-findings-r1.md F9).
type GlyphSet struct {
	Unread         string
	Flagged        string
	Attachment     string
	Collapsed      string
	Expanded       string
	Selected       string
	Ellipsis       string
	Dismiss        string
	EdgeBarFocused string
	EdgeBarBlurred string
	Separator      string
	ScrollPos      string
	TreeBranch     string
	TreeLast       string
	Divider        string
	ErrorGutter    string
	WarnMarker     string
}

// fullGlyphs is the Unicode glyph set, ProfileTrueColor's tier.
var fullGlyphs = GlyphSet{
	Unread:         "●",
	Flagged:        "⚑",
	Attachment:     "⊕",
	Collapsed:      "▸",
	Expanded:       "▾",
	Selected:       "✓",
	Ellipsis:       "…",
	Dismiss:        "✕",
	EdgeBarFocused: "▌",
	EdgeBarBlurred: "▏",
	Separator:      "·",
	ScrollPos:      "≡",
	TreeBranch:     "├─",
	TreeLast:       "└─",
	Divider:        "│",
	ErrorGutter:    "!",
	WarnMarker:     "▲",
}

// asciiGlyphs is the ASCII fallback set, ProfileANSI16 and
// ProfileNoColor's tier (design decision 3's substitution list).
// Every field is exactly as wide, in terminal cells, as its
// fullGlyphs counterpart (TestGlyphWidthParity), so a degrade
// substitution never shifts a column budget decision 12 fixed at
// full color.
var asciiGlyphs = GlyphSet{
	Unread:         "*",
	Flagged:        "!",
	Attachment:     "+",
	Collapsed:      ">",
	Expanded:       "v",
	Selected:       "X",
	Ellipsis:       "~",
	Dismiss:        "x",
	EdgeBarFocused: ">",
	EdgeBarBlurred: "|",
	Separator:      "-",
	ScrollPos:      "=",
	TreeBranch:     "|-",
	TreeLast:       "+-",
	Divider:        "|",
	ErrorGutter:    "!",
	WarnMarker:     "^",
}

// Glyphs returns t's glyph token set: fullGlyphs at
// ProfileTrueColor, asciiGlyphs at ProfileANSI16 and
// ProfileNoColor.
func (t Theme) Glyphs() GlyphSet {
	if t.profile == ProfileTrueColor {
		return fullGlyphs
	}
	return asciiGlyphs
}
