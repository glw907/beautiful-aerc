package theme

import "sort"

// DefaultThemeName is the CLI name of the default theme.
const DefaultThemeName = "one-dark"

var oneDarkPalette = Palette{
	BgBase:          "#282c34",
	BgElevated:      "#31353f",
	BgSelection:     "#393f4a",
	BgBorder:        "#5c6370",
	FgBase:          "#abb2bf",
	FgBright:        "#ced4de",
	FgBrightest:     "#e6e8ee",
	FgDim:           "#5c6370",
	AccentPrimary:   "#61afef",
	AccentSecondary: "#56b6c2",
	AccentTertiary:  "#98c379",
	ColorError:      "#e86671",
	ColorWarning:    "#d19a66",
	ColorSuccess:    "#98c379",
	ColorInfo:       "#e5c07b",
	ColorSpecial:    "#c678dd",
}

var nordPalette = Palette{
	BgBase:          "#2e3440",
	BgElevated:      "#3b4252",
	BgSelection:     "#434c5e",
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
	BgSelection:     "#134858",
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
	BgSelection:     "#504945",
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

var catppuccinMochaPalette = Palette{
	BgBase:          "#1e1e2e",
	BgElevated:      "#313244",
	BgSelection:     "#45475a",
	BgBorder:        "#585b70",
	FgBase:          "#cdd6f4",
	FgBright:        "#bac2de",
	FgBrightest:     "#f5e0dc",
	FgDim:           "#6c7086",
	AccentPrimary:   "#89b4fa",
	AccentSecondary: "#94e2d5",
	AccentTertiary:  "#a6e3a1",
	ColorError:      "#f38ba8",
	ColorWarning:    "#fab387",
	ColorSuccess:    "#a6e3a1",
	ColorInfo:       "#f9e2af",
	ColorSpecial:    "#cba6f7",
}

var catppuccinLattePalette = Palette{
	BgBase:          "#eff1f5",
	BgElevated:      "#ccd0da",
	BgSelection:     "#bcc0cc",
	BgBorder:        "#acb0be",
	FgBase:          "#4c4f69",
	FgBright:        "#5c5f77",
	FgBrightest:     "#11111b",
	FgDim:           "#9ca0b0",
	AccentPrimary:   "#1e66f5",
	AccentSecondary: "#179299",
	AccentTertiary:  "#40a02b",
	ColorError:      "#d20f39",
	ColorWarning:    "#fe640b",
	ColorSuccess:    "#40a02b",
	ColorInfo:       "#df8e1d",
	ColorSpecial:    "#8839ef",
}

var draculaPalette = Palette{
	BgBase:          "#282a36",
	BgElevated:      "#44475a",
	BgSelection:     "#44475a",
	BgBorder:        "#6272a4",
	FgBase:          "#f8f8f2",
	FgBright:        "#f8f8f2",
	FgBrightest:     "#ffffff",
	FgDim:           "#6272a4",
	AccentPrimary:   "#bd93f9",
	AccentSecondary: "#8be9fd",
	AccentTertiary:  "#50fa7b",
	ColorError:      "#ff5555",
	ColorWarning:    "#ffb86c",
	ColorSuccess:    "#50fa7b",
	ColorInfo:       "#f1fa8c",
	ColorSpecial:    "#ff79c6",
}

var tokyoNightPalette = Palette{
	BgBase:          "#1a1b26",
	BgElevated:      "#16161e",
	BgSelection:     "#283457",
	BgBorder:        "#3b4261",
	FgBase:          "#c0caf5",
	FgBright:        "#a9b1d6",
	FgBrightest:     "#c0caf5",
	FgDim:           "#565f89",
	AccentPrimary:   "#7aa2f7",
	AccentSecondary: "#7dcfff",
	AccentTertiary:  "#9ece6a",
	ColorError:      "#f7768e",
	ColorWarning:    "#ff9e64",
	ColorSuccess:    "#9ece6a",
	ColorInfo:       "#e0af68",
	ColorSpecial:    "#bb9af7",
}

var rosePinePalette = Palette{
	BgBase:          "#191724",
	BgElevated:      "#1f1d2e",
	BgSelection:     "#403d52",
	BgBorder:        "#524f67",
	FgBase:          "#e0def4",
	FgBright:        "#e0def4",
	FgBrightest:     "#e0def4",
	FgDim:           "#6e6a86",
	AccentPrimary:   "#c4a7e7",
	AccentSecondary: "#9ccfd8",
	AccentTertiary:  "#31748f",
	ColorError:      "#eb6f92",
	ColorWarning:    "#f6c177",
	ColorSuccess:    "#9ccfd8",
	ColorInfo:       "#f6c177",
	ColorSpecial:    "#c4a7e7",
}

var rosePineDawnPalette = Palette{
	BgBase:          "#faf4ed",
	BgElevated:      "#fffaf3",
	BgSelection:     "#dfdad9",
	BgBorder:        "#cecacd",
	FgBase:          "#575279",
	FgBright:        "#464261",
	FgBrightest:     "#26233a",
	FgDim:           "#9893a5",
	AccentPrimary:   "#907aa9",
	AccentSecondary: "#56949f",
	AccentTertiary:  "#286983",
	ColorError:      "#b4637a",
	ColorWarning:    "#ea9d34",
	ColorSuccess:    "#56949f",
	ColorInfo:       "#ea9d34",
	ColorSpecial:    "#907aa9",
}

var kanagawaPalette = Palette{
	BgBase:          "#1F1F28",
	BgElevated:      "#2A2A37",
	BgSelection:     "#223249",
	BgBorder:        "#54546D",
	FgBase:          "#DCD7BA",
	FgBright:        "#C8C093",
	FgBrightest:     "#DCD7BA",
	FgDim:           "#727169",
	AccentPrimary:   "#7E9CD8",
	AccentSecondary: "#7FB4CA",
	AccentTertiary:  "#6A9589",
	ColorError:      "#E82424",
	ColorWarning:    "#FF9E3B",
	ColorSuccess:    "#98BB6C",
	ColorInfo:       "#E6C384",
	ColorSpecial:    "#957FB8",
}

var everforestDarkPalette = Palette{
	BgBase:          "#2d353b",
	BgElevated:      "#343f44",
	BgSelection:     "#3d484d",
	BgBorder:        "#7a8478",
	FgBase:          "#d3c6aa",
	FgBright:        "#d3c6aa",
	FgBrightest:     "#e6ddc4",
	FgDim:           "#859289",
	AccentPrimary:   "#7fbbb3",
	AccentSecondary: "#83c092",
	AccentTertiary:  "#a7c080",
	ColorError:      "#e67e80",
	ColorWarning:    "#e69875",
	ColorSuccess:    "#a7c080",
	ColorInfo:       "#dbbc7f",
	ColorSpecial:    "#d699b6",
}

var everforestLightPalette = Palette{
	BgBase:          "#fdf6e3",
	BgElevated:      "#f4f0d9",
	BgSelection:     "#efebd4",
	BgBorder:        "#a6b0a0",
	FgBase:          "#5c6a72",
	FgBright:        "#5c6a72",
	FgBrightest:     "#3c474d",
	FgDim:           "#939f91",
	AccentPrimary:   "#3a94c5",
	AccentSecondary: "#35a77c",
	AccentTertiary:  "#8da101",
	ColorError:      "#f85552",
	ColorWarning:    "#f57d26",
	ColorSuccess:    "#8da101",
	ColorInfo:       "#dfa000",
	ColorSpecial:    "#df69ba",
}

var solarizedLightPalette = Palette{
	BgBase:          "#fdf6e3",
	BgElevated:      "#eee8d5",
	BgSelection:     "#eee8d5",
	BgBorder:        "#93a1a1",
	FgBase:          "#657b83",
	FgBright:        "#586e75",
	FgBrightest:     "#073642",
	FgDim:           "#93a1a1",
	AccentPrimary:   "#268bd2",
	AccentSecondary: "#2aa198",
	AccentTertiary:  "#2aa198",
	ColorError:      "#dc322f",
	ColorWarning:    "#cb4b16",
	ColorSuccess:    "#859900",
	ColorInfo:       "#b58900",
	ColorSpecial:    "#6c71c4",
}

var gruvboxLightPalette = Palette{
	BgBase:          "#fbf1c7",
	BgElevated:      "#ebdbb2",
	BgSelection:     "#d5c4a1",
	BgBorder:        "#a89984",
	FgBase:          "#3c3836",
	FgBright:        "#504945",
	FgBrightest:     "#1d2021",
	FgDim:           "#928374",
	AccentPrimary:   "#076678",
	AccentSecondary: "#427b58",
	AccentTertiary:  "#79740e",
	ColorError:      "#9d0006",
	ColorWarning:    "#af3a03",
	ColorSuccess:    "#79740e",
	ColorInfo:       "#b57614",
	ColorSpecial:    "#8f3f71",
}

// OneDark is the compiled One Dark theme (default).
var OneDark = NewCompiledTheme("One Dark", oneDarkPalette)

// Nord is the compiled Nord theme.
var Nord = NewCompiledTheme("Nord", nordPalette)

// SolarizedDark is the compiled Solarized Dark theme.
var SolarizedDark = NewCompiledTheme("Solarized Dark", solarizedDarkPalette)

// GruvboxDark is the compiled Gruvbox Dark theme.
var GruvboxDark = NewCompiledTheme("Gruvbox Dark", gruvboxDarkPalette)

// CatppuccinMocha is the compiled Catppuccin Mocha theme.
var CatppuccinMocha = NewCompiledTheme("Catppuccin Mocha", catppuccinMochaPalette)

// CatppuccinLatte is the compiled Catppuccin Latte theme.
var CatppuccinLatte = NewCompiledTheme("Catppuccin Latte", catppuccinLattePalette)

// Dracula is the compiled Dracula theme.
var Dracula = NewCompiledTheme("Dracula", draculaPalette)

// TokyoNight is the compiled Tokyo Night theme.
var TokyoNight = NewCompiledTheme("Tokyo Night", tokyoNightPalette)

// RosePine is the compiled Rose Pine theme.
var RosePine = NewCompiledTheme("Rose Pine", rosePinePalette)

// RosePineDawn is the compiled Rose Pine Dawn theme.
var RosePineDawn = NewCompiledTheme("Rose Pine Dawn", rosePineDawnPalette)

// Kanagawa is the compiled Kanagawa theme.
var Kanagawa = NewCompiledTheme("Kanagawa", kanagawaPalette)

// EverforestDark is the compiled Everforest Dark theme.
var EverforestDark = NewCompiledTheme("Everforest Dark", everforestDarkPalette)

// EverforestLight is the compiled Everforest Light theme.
var EverforestLight = NewCompiledTheme("Everforest Light", everforestLightPalette)

// SolarizedLight is the compiled Solarized Light theme.
var SolarizedLight = NewCompiledTheme("Solarized Light", solarizedLightPalette)

// GruvboxLight is the compiled Gruvbox Light theme.
var GruvboxLight = NewCompiledTheme("Gruvbox Light", gruvboxLightPalette)

// Themes maps lowercase CLI names to compiled themes.
var Themes = map[string]*CompiledTheme{
	"catppuccin-latte": CatppuccinLatte,
	"catppuccin-mocha": CatppuccinMocha,
	"dracula":          Dracula,
	"everforest-dark":  EverforestDark,
	"everforest-light": EverforestLight,
	"gruvbox-dark":     GruvboxDark,
	"gruvbox-light":    GruvboxLight,
	"kanagawa":         Kanagawa,
	"nord":             Nord,
	"one-dark":         OneDark,
	"rose-pine":        RosePine,
	"rose-pine-dawn":   RosePineDawn,
	"solarized-dark":   SolarizedDark,
	"solarized-light":  SolarizedLight,
	"tokyo-night":      TokyoNight,
}

// ThemeNames returns the available theme names in alphabetical order.
func ThemeNames() []string {
	names := make([]string, 0, len(Themes))
	for name := range Themes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
