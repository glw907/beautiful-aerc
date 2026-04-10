package theme

import "github.com/charmbracelet/lipgloss"

// Palette holds the 16 semantic hex color values for a theme.
type Palette struct {
	BgBase      string
	BgElevated  string
	BgSelection string
	BgBorder    string

	FgBase      string
	FgBright    string
	FgBrightest string
	FgDim       string

	AccentPrimary   string
	AccentSecondary string
	AccentTertiary  string

	ColorError   string
	ColorWarning string
	ColorSuccess string
	ColorInfo    string
	ColorSpecial string
}

// CompiledTheme holds lipgloss colors and composed styles for rendering.
type CompiledTheme struct {
	// Name is the display name of the theme.
	Name string

	// Palette colors as lipgloss values
	BgBase, BgElevated, BgSelection, BgBorder         lipgloss.Color
	FgBase, FgBright, FgBrightest, FgDim               lipgloss.Color
	AccentPrimary, AccentSecondary, AccentTertiary      lipgloss.Color
	ColorError, ColorWarning, ColorSuccess              lipgloss.Color
	ColorInfo, ColorSpecial                             lipgloss.Color

	// Composed styles for content rendering
	HeaderKey      lipgloss.Style
	HeaderValue    lipgloss.Style
	HeaderDim      lipgloss.Style
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
		BgBorder:        lipgloss.Color(p.BgBorder),
		FgBase:          lipgloss.Color(p.FgBase),
		FgBright:        lipgloss.Color(p.FgBright),
		FgBrightest:     lipgloss.Color(p.FgBrightest),
		FgDim:           lipgloss.Color(p.FgDim),
		AccentPrimary:   lipgloss.Color(p.AccentPrimary),
		AccentSecondary: lipgloss.Color(p.AccentSecondary),
		AccentTertiary:  lipgloss.Color(p.AccentTertiary),
		ColorError:      lipgloss.Color(p.ColorError),
		ColorWarning:    lipgloss.Color(p.ColorWarning),
		ColorSuccess:    lipgloss.Color(p.ColorSuccess),
		ColorInfo:       lipgloss.Color(p.ColorInfo),
		ColorSpecial:    lipgloss.Color(p.ColorSpecial),
	}

	t.HeaderKey = lipgloss.NewStyle().
		Foreground(t.AccentPrimary).Bold(true)
	t.HeaderValue = lipgloss.NewStyle().
		Foreground(t.FgBase)
	t.HeaderDim = lipgloss.NewStyle().
		Foreground(t.FgDim)
	t.Paragraph = lipgloss.NewStyle().
		Foreground(t.FgBase)
	t.Heading = lipgloss.NewStyle().
		Foreground(t.ColorSuccess).Bold(true)
	t.Quote = lipgloss.NewStyle().
		Foreground(t.AccentTertiary)
	t.DeepQuote = lipgloss.NewStyle().
		Foreground(t.FgDim)
	t.Attribution = lipgloss.NewStyle().
		Foreground(t.FgDim).Italic(true)
	t.Signature = lipgloss.NewStyle().
		Foreground(t.FgDim)
	t.Bold = lipgloss.NewStyle().Bold(true)
	t.Italic = lipgloss.NewStyle().Italic(true)
	t.Link = lipgloss.NewStyle().
		Foreground(t.AccentPrimary).Underline(true)
	t.CodeInline = lipgloss.NewStyle().
		Foreground(t.FgBright)
	t.CodeBlock = lipgloss.NewStyle().
		Foreground(t.FgBright)
	t.HorizontalRule = lipgloss.NewStyle().
		Foreground(t.FgDim)

	return t
}

// PaletteHex returns the raw hex value for a color slot by name.
// Used by the styleset generator.
func (t *CompiledTheme) PaletteHex(name string) string {
	switch name {
	case "bg_base":
		return string(t.BgBase)
	case "bg_elevated":
		return string(t.BgElevated)
	case "bg_selection":
		return string(t.BgSelection)
	case "bg_border":
		return string(t.BgBorder)
	case "fg_base":
		return string(t.FgBase)
	case "fg_bright":
		return string(t.FgBright)
	case "fg_brightest":
		return string(t.FgBrightest)
	case "fg_dim":
		return string(t.FgDim)
	case "accent_primary":
		return string(t.AccentPrimary)
	case "accent_secondary":
		return string(t.AccentSecondary)
	case "accent_tertiary":
		return string(t.AccentTertiary)
	case "color_error":
		return string(t.ColorError)
	case "color_warning":
		return string(t.ColorWarning)
	case "color_success":
		return string(t.ColorSuccess)
	case "color_info":
		return string(t.ColorInfo)
	case "color_special":
		return string(t.ColorSpecial)
	default:
		return ""
	}
}
