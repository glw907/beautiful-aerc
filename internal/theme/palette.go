// SPDX-License-Identifier: MIT

package theme

import "github.com/charmbracelet/lipgloss"

// Palette holds the 16 semantic hex color values for a theme.
type Palette struct {
	BgBase      string
	BgElevated  string
	BgSelection string
	BgSubtle    string
	BgBorder    string

	FgBase   string
	FgBright string
	FgDim    string

	AccentPrimary   string
	AccentSecondary string
	AccentTertiary  string

	ColorError   string
	ColorWarning string
	ColorSuccess string
}

// CompiledTheme holds lipgloss colors and composed styles for rendering.
type CompiledTheme struct {
	// Name is the display name of the theme.
	Name string

	// Palette colors as lipgloss values
	BgBase, BgElevated, BgSelection, BgSubtle, BgBorder lipgloss.Color
	FgBase, FgBright, FgDim                             lipgloss.Color
	AccentPrimary, AccentSecondary, AccentTertiary      lipgloss.Color
	ColorError, ColorWarning, ColorSuccess              lipgloss.Color

	// Composed styles for content rendering
	HeaderKey    lipgloss.Style
	HeaderValue  lipgloss.Style
	HeaderDim    lipgloss.Style
	SubjectTitle lipgloss.Style
	Paragraph      lipgloss.Style
	Heading        lipgloss.Style
	Quote          lipgloss.Style
	DeepQuote      lipgloss.Style
	Attribution    lipgloss.Style
	Signature      lipgloss.Style
	Bold           lipgloss.Style
	Italic         lipgloss.Style
	Link           lipgloss.Style
	CodeInline     lipgloss.Style
	CodeBlock      lipgloss.Style
	HorizontalRule lipgloss.Style
}

// NewCompiledTheme creates a CompiledTheme from a Palette, building all composed styles.
func NewCompiledTheme(name string, p Palette) *CompiledTheme {
	t := &CompiledTheme{
		Name: name,

		BgBase:          lipgloss.Color(p.BgBase),
		BgElevated:      lipgloss.Color(p.BgElevated),
		BgSelection:     lipgloss.Color(p.BgSelection),
		BgSubtle:        lipgloss.Color(p.BgSubtle),
		BgBorder:        lipgloss.Color(p.BgBorder),
		FgBase:          lipgloss.Color(p.FgBase),
		FgBright:        lipgloss.Color(p.FgBright),
		FgDim:           lipgloss.Color(p.FgDim),
		AccentPrimary:   lipgloss.Color(p.AccentPrimary),
		AccentSecondary: lipgloss.Color(p.AccentSecondary),
		AccentTertiary:  lipgloss.Color(p.AccentTertiary),
		ColorError:      lipgloss.Color(p.ColorError),
		ColorWarning:    lipgloss.Color(p.ColorWarning),
		ColorSuccess:    lipgloss.Color(p.ColorSuccess),
	}

	// Header-pane styles render on BgSubtle (a small elevation off
	// BgBase, signaling the header belongs to the message context, not
	// the sidebar's BgElevated chrome track); body-pane styles render
	// on BgBase. Baking the pane bg into every leaf eliminates the
	// terminal-default-bg gaps lipgloss leaves between pre-styled
	// segments (issue #209).
	t.HeaderKey = lipgloss.NewStyle().
		Foreground(t.AccentPrimary).Bold(true).Background(t.BgSubtle)
	t.HeaderValue = lipgloss.NewStyle().
		Foreground(t.FgBase).Background(t.BgSubtle)
	t.HeaderDim = lipgloss.NewStyle().
		Foreground(t.FgDim).Background(t.BgSubtle)
	t.SubjectTitle = lipgloss.NewStyle().
		Foreground(t.FgBright).Bold(true).Background(t.BgSubtle)
	t.Paragraph = lipgloss.NewStyle().
		Foreground(t.FgBase).Background(t.BgBase)
	t.Heading = lipgloss.NewStyle().
		Foreground(t.ColorSuccess).Bold(true).Background(t.BgBase)
	t.Quote = lipgloss.NewStyle().
		Foreground(t.AccentTertiary).Background(t.BgBase)
	t.DeepQuote = lipgloss.NewStyle().
		Foreground(t.FgDim).Background(t.BgBase)
	t.Attribution = lipgloss.NewStyle().
		Foreground(t.FgDim).Italic(true).Background(t.BgBase)
	t.Signature = lipgloss.NewStyle().
		Foreground(t.FgDim).Background(t.BgBase)
	t.Bold = lipgloss.NewStyle().Bold(true).Background(t.BgBase)
	t.Italic = lipgloss.NewStyle().Italic(true).Background(t.BgBase)
	t.Link = lipgloss.NewStyle().
		Foreground(t.AccentPrimary).Underline(true).Background(t.BgBase)
	t.CodeInline = lipgloss.NewStyle().
		Foreground(t.FgBright).Background(t.BgBase)
	t.CodeBlock = lipgloss.NewStyle().
		Foreground(t.FgBright).Background(t.BgBase)
	t.HorizontalRule = lipgloss.NewStyle().
		Foreground(t.FgDim).Background(t.BgBase)

	return t
}

