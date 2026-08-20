package theme

// GlyphSet is the theme's compiled glyph tokens (design language
// section 7; decision 3 adds Dismiss through ErrorGutter), resolved
// to the profile's fallback by Glyphs.
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
	QuoteBar       string
	ErrorGutter    string
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
	QuoteBar:       "│",
	ErrorGutter:    "!",
}

// asciiGlyphs is the ASCII fallback set, ProfileANSI16 and
// ProfileNoColor's tier (design decision 3's substitution list).
var asciiGlyphs = GlyphSet{
	Unread:         "*",
	Flagged:        "!",
	Attachment:     "+",
	Collapsed:      ">",
	Expanded:       "v",
	Selected:       "X",
	Ellipsis:       "...",
	Dismiss:        "x",
	EdgeBarFocused: ">",
	EdgeBarBlurred: "|",
	Separator:      "-",
	ScrollPos:      "=",
	TreeBranch:     "|-",
	TreeLast:       "+-",
	QuoteBar:       "|",
	ErrorGutter:    "!",
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
