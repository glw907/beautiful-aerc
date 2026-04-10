package theme

var nordPalette = Palette{
	BgBase:          "#2e3440",
	BgElevated:      "#3b4252",
	BgSelection:     "#394353",
	BgBorder:        "#49576b",
	FgBase:          "#d8dee9",
	FgBright:        "#e5e9f0",
	FgBrightest:     "#eceff4",
	FgDim:           "#616e88",
	AccentPrimary:   "#81a1c1",
	AccentSecondary: "#88c0d0",
	AccentTertiary:  "#8fbcbb",
	ColorError:      "#bf616a",
	ColorWarning:    "#d08770",
	ColorSuccess:    "#a3be8c",
	ColorInfo:       "#ebcb8b",
	ColorSpecial:    "#b48ead",
}

var solarizedDarkPalette = Palette{
	BgBase:          "#002b36",
	BgElevated:      "#073642",
	BgSelection:     "#073642",
	BgBorder:        "#586e75",
	FgBase:          "#839496",
	FgBright:        "#93a1a1",
	FgBrightest:     "#eee8d5",
	FgDim:           "#657b83",
	AccentPrimary:   "#268bd2",
	AccentSecondary: "#2aa198",
	AccentTertiary:  "#2aa198",
	ColorError:      "#dc322f",
	ColorWarning:    "#cb4b16",
	ColorSuccess:    "#859900",
	ColorInfo:       "#b58900",
	ColorSpecial:    "#6c71c4",
}

var gruvboxDarkPalette = Palette{
	BgBase:          "#282828",
	BgElevated:      "#3c3836",
	BgSelection:     "#3c3836",
	BgBorder:        "#665c54",
	FgBase:          "#ebdbb2",
	FgBright:        "#fbf1c7",
	FgBrightest:     "#fbf1c7",
	FgDim:           "#928374",
	AccentPrimary:   "#83a598",
	AccentSecondary: "#8ec07c",
	AccentTertiary:  "#8ec07c",
	ColorError:      "#fb4934",
	ColorWarning:    "#fe8019",
	ColorSuccess:    "#b8bb26",
	ColorInfo:       "#fabd2f",
	ColorSpecial:    "#d3869b",
}

// Nord is the compiled Nord theme.
var Nord = NewCompiledTheme("Nord", nordPalette)

// SolarizedDark is the compiled Solarized Dark theme.
var SolarizedDark = NewCompiledTheme("Solarized Dark", solarizedDarkPalette)

// GruvboxDark is the compiled Gruvbox Dark theme.
var GruvboxDark = NewCompiledTheme("Gruvbox Dark", gruvboxDarkPalette)
