package theme

import (
	"github.com/charmbracelet/glamour/ansi"
)

func ptr[T any](v T) *T { return &v }

// GlamourStyle builds a Glamour ansi.StyleConfig from the theme's tokens
// and color slots. Tokens map to Glamour style elements:
//
//	heading   → H1-H6
//	bold      → Strong
//	italic    → Emph
//	link_text → LinkText
//	rule      → HorizontalRule
func (t *Theme) GlamourStyle() ansi.StyleConfig {
	style := ansi.StyleConfig{
		Document: ansi.StyleBlock{
			Margin: ptr(uint(0)),
			Indent: ptr(uint(0)),
		},
	}

	if hdr := t.glamourPrimitive("heading"); hdr != nil {
		hdr.BlockSuffix = "\n"
		block := ansi.StyleBlock{StylePrimitive: *hdr}
		style.H1 = block
		style.H2 = block
		style.H3 = block
		style.H4 = block
		style.H5 = block
		style.H6 = block
	}

	if s := t.glamourPrimitive("bold"); s != nil {
		style.Strong = *s
	}

	if s := t.glamourPrimitive("italic"); s != nil {
		style.Emph = *s
	}

	if s := t.glamourPrimitive("link_text"); s != nil {
		style.LinkText = *s
	}

	if s := t.glamourPrimitive("link_url"); s != nil {
		s.BlockPrefix = "("
		s.BlockSuffix = ")"
		style.Link = *s
	}

	if s := t.glamourPrimitive("rule"); s != nil {
		style.HorizontalRule = *s
	}

	// Blockquote: use a "│ " indent token colored with accent_tertiary,
	// matching the quote styling in the aerc styleset.
	style.BlockQuote = ansi.StyleBlock{
		Indent:      ptr(uint(1)),
		IndentToken: ptr("│ "),
	}
	if color, ok := t.colors["accent_tertiary"]; ok {
		style.BlockQuote.StylePrimitive = ansi.StylePrimitive{
			Color: ptr(color),
		}
	}

	// List styling. Glamour doesn't support hanging indent for
	// wrapped list items (issue #56, #314), so continuation lines
	// align with the bullet rather than the text. Good enough.
	style.List = ansi.StyleList{
		LevelIndent: 2,
	}
	style.Item = ansi.StylePrimitive{
		BlockPrefix: "- ",
	}
	style.Enumeration = ansi.StylePrimitive{
		BlockPrefix: ". ",
	}

	return style
}

// glamourPrimitive converts a theme token to a Glamour StylePrimitive.
// Returns nil if the token is not defined.
func (t *Theme) glamourPrimitive(tokenName string) *ansi.StylePrimitive {
	def, ok := t.tokenDefs[tokenName]
	if !ok {
		return nil
	}

	p := &ansi.StylePrimitive{}

	// Load() validates all color references, so def.Color is always a valid slot.
	if def.Color != "" {
		p.Color = ptr(t.colors[def.Color])
	}
	if def.Bold {
		p.Bold = ptr(true)
	}
	if def.Italic {
		p.Italic = ptr(true)
	}
	if def.Underline {
		p.Underline = ptr(true)
	}

	return p
}
