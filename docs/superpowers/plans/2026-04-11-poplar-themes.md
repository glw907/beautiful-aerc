# Poplar Theme Selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship poplar with 15 compiled themes (10 dark, 5 light) selectable via `--theme`/`-t` flag, defaulting to One Dark, with a `poplar themes` subcommand.

**Architecture:** Add 12 new `Palette` vars and compiled theme vars to `internal/theme/themes.go`. Update the `Themes` map and `ThemeNames()` to cover all 15. Add a `-t` short flag and change the default in `cmd/poplar/root.go`. Add a `poplar themes` subcommand. Update `cmd/mailrender/themes.go` and `preview.go` to use the shared registry instead of hardcoded maps.

**Tech Stack:** Go, lipgloss, cobra

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/theme/themes.go` | Modify | Add 12 palettes + compiled theme vars, update `Themes` map and `ThemeNames()` |
| `internal/theme/palette_test.go` | Modify | Extend `TestAllThemesBuild` and distinctness test to all 15 themes |
| `cmd/poplar/root.go` | Modify | Add `-t` short flag, change default to `one-dark` |
| `cmd/poplar/themes.go` | Create | `poplar themes` subcommand |
| `cmd/poplar/main.go` | Modify | Register themes subcommand |
| `cmd/mailrender/themes.go` | Modify | Use `theme.Themes` registry instead of hardcoded map |
| `cmd/mailrender/preview.go` | Modify | Use `theme.Themes` registry instead of switch statement |
| `docs/themes.md` | Modify | Add all 15 themes to reference |
| `docs/styling.md` | Modify | Remove TOML references, update to compiled theme model |
| `docs/poplar/architecture.md` | Modify | Add theme lineup decision record |
| `docs/poplar/STATUS.md` | Modify | Add theme spec link to plans list |

---

### Task 1: Add One Dark palette and compiled theme

**Files:**
- Modify: `internal/theme/themes.go`

- [ ] **Step 1: Write the failing test**

Add One Dark to the `TestAllThemesBuild` test in `internal/theme/palette_test.go`:

```go
func TestAllThemesBuild(t *testing.T) {
	themes := map[string]*CompiledTheme{
		"Nord":          Nord,
		"SolarizedDark": SolarizedDark,
		"GruvboxDark":   GruvboxDark,
		"OneDark":       OneDark,
	}
	for name, th := range themes {
		t.Run(name, func(t *testing.T) {
			if th == nil {
				t.Fatal("theme is nil")
			}
			rendered := th.Heading.Render("Test")
			if rendered == "Test" {
				t.Error("Heading style is unstyled")
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/theme/ -run TestAllThemesBuild -v`
Expected: compile error — `OneDark` undefined

- [ ] **Step 3: Add One Dark palette and compiled theme**

Add to `internal/theme/themes.go` before the existing `nordPalette`:

```go
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
```

Add the compiled theme var after the palette:

```go
// OneDark is the compiled One Dark theme (default).
var OneDark = NewCompiledTheme("One Dark", oneDarkPalette)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/theme/ -run TestAllThemesBuild -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/theme/themes.go internal/theme/palette_test.go
git commit -m "Add One Dark palette and compiled theme"
```

---

### Task 2: Add Catppuccin Mocha and Catppuccin Latte

**Files:**
- Modify: `internal/theme/themes.go`
- Modify: `internal/theme/palette_test.go`

- [ ] **Step 1: Add palettes and compiled themes**

Add to `internal/theme/themes.go`:

```go
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

// CatppuccinMocha is the compiled Catppuccin Mocha theme.
var CatppuccinMocha = NewCompiledTheme("Catppuccin Mocha", catppuccinMochaPalette)

// CatppuccinLatte is the compiled Catppuccin Latte theme.
var CatppuccinLatte = NewCompiledTheme("Catppuccin Latte", catppuccinLattePalette)
```

- [ ] **Step 2: Add to test**

Add `"CatppuccinMocha": CatppuccinMocha` and `"CatppuccinLatte": CatppuccinLatte` to the `TestAllThemesBuild` map.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/theme/ -run TestAllThemesBuild -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/theme/themes.go internal/theme/palette_test.go
git commit -m "Add Catppuccin Mocha and Latte palettes"
```

---

### Task 3: Add Dracula and Tokyo Night

**Files:**
- Modify: `internal/theme/themes.go`
- Modify: `internal/theme/palette_test.go`

- [ ] **Step 1: Add palettes and compiled themes**

Add to `internal/theme/themes.go`:

```go
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

// Dracula is the compiled Dracula theme.
var Dracula = NewCompiledTheme("Dracula", draculaPalette)

// TokyoNight is the compiled Tokyo Night theme.
var TokyoNight = NewCompiledTheme("Tokyo Night", tokyoNightPalette)
```

- [ ] **Step 2: Add to test**

Add `"Dracula": Dracula` and `"TokyoNight": TokyoNight` to the `TestAllThemesBuild` map.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/theme/ -run TestAllThemesBuild -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/theme/themes.go internal/theme/palette_test.go
git commit -m "Add Dracula and Tokyo Night palettes"
```

---

### Task 4: Add Rose Pine and Rose Pine Dawn

**Files:**
- Modify: `internal/theme/themes.go`
- Modify: `internal/theme/palette_test.go`

- [ ] **Step 1: Add palettes and compiled themes**

Add to `internal/theme/themes.go`:

```go
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

// RosePine is the compiled Rose Pine theme.
var RosePine = NewCompiledTheme("Rose Pine", rosePinePalette)

// RosePineDawn is the compiled Rose Pine Dawn theme.
var RosePineDawn = NewCompiledTheme("Rose Pine Dawn", rosePineDawnPalette)
```

- [ ] **Step 2: Add to test**

Add `"RosePine": RosePine` and `"RosePineDawn": RosePineDawn` to the `TestAllThemesBuild` map.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/theme/ -run TestAllThemesBuild -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/theme/themes.go internal/theme/palette_test.go
git commit -m "Add Rose Pine and Rose Pine Dawn palettes"
```

---

### Task 5: Add Kanagawa, Everforest Dark, and Everforest Light

**Files:**
- Modify: `internal/theme/themes.go`
- Modify: `internal/theme/palette_test.go`

- [ ] **Step 1: Add palettes and compiled themes**

Add to `internal/theme/themes.go`:

```go
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

// Kanagawa is the compiled Kanagawa theme.
var Kanagawa = NewCompiledTheme("Kanagawa", kanagawaPalette)

// EverforestDark is the compiled Everforest Dark theme.
var EverforestDark = NewCompiledTheme("Everforest Dark", everforestDarkPalette)

// EverforestLight is the compiled Everforest Light theme.
var EverforestLight = NewCompiledTheme("Everforest Light", everforestLightPalette)
```

- [ ] **Step 2: Add to test**

Add `"Kanagawa": Kanagawa`, `"EverforestDark": EverforestDark`, and `"EverforestLight": EverforestLight` to the `TestAllThemesBuild` map.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/theme/ -run TestAllThemesBuild -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/theme/themes.go internal/theme/palette_test.go
git commit -m "Add Kanagawa, Everforest Dark, and Everforest Light palettes"
```

---

### Task 6: Add Solarized Light and Gruvbox Light

**Files:**
- Modify: `internal/theme/themes.go`
- Modify: `internal/theme/palette_test.go`

- [ ] **Step 1: Add palettes and compiled themes**

Add to `internal/theme/themes.go`:

```go
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

// SolarizedLight is the compiled Solarized Light theme.
var SolarizedLight = NewCompiledTheme("Solarized Light", solarizedLightPalette)

// GruvboxLight is the compiled Gruvbox Light theme.
var GruvboxLight = NewCompiledTheme("Gruvbox Light", gruvboxLightPalette)
```

- [ ] **Step 2: Add to test**

Add `"SolarizedLight": SolarizedLight` and `"GruvboxLight": GruvboxLight` to the `TestAllThemesBuild` map.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/theme/ -run TestAllThemesBuild -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/theme/themes.go internal/theme/palette_test.go
git commit -m "Add Solarized Light and Gruvbox Light palettes"
```

---

### Task 7: Update theme registry and ThemeNames

**Files:**
- Modify: `internal/theme/themes.go`

- [ ] **Step 1: Write the failing test**

Replace the `TestAllThemesHaveDistinctColors` test in `internal/theme/theme_test.go` with a comprehensive registry test:

```go
func TestThemeRegistryComplete(t *testing.T) {
	expected := []string{
		"catppuccin-latte", "catppuccin-mocha", "dracula",
		"everforest-dark", "everforest-light", "gruvbox-dark",
		"gruvbox-light", "kanagawa", "nord", "one-dark",
		"rose-pine", "rose-pine-dawn", "solarized-dark",
		"solarized-light", "tokyo-night",
	}

	if len(Themes) != 15 {
		t.Fatalf("Themes map has %d entries, want 15", len(Themes))
	}

	for _, name := range expected {
		if _, ok := Themes[name]; !ok {
			t.Errorf("Themes map missing %q", name)
		}
	}

	names := ThemeNames()
	if len(names) != 15 {
		t.Fatalf("ThemeNames() returned %d names, want 15", len(names))
	}

	// Verify alphabetical order.
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("ThemeNames() not sorted: %q before %q", names[i-1], names[i])
		}
	}

	// Verify all themes have distinct bg_base (no copy-paste errors).
	seen := map[string]string{}
	for name, th := range Themes {
		bg := th.PaletteHex("bg_base")
		if bg == "" {
			t.Errorf("theme %q has empty bg_base", name)
		}
		if other, ok := seen[bg]; ok {
			// Solarized Light and Everforest Light share #fdf6e3 — skip that known pair.
			if !((name == "solarized-light" && other == "everforest-light") ||
				(name == "everforest-light" && other == "solarized-light")) {
				t.Errorf("themes %q and %q share bg_base %s", name, other, bg)
			}
		}
		seen[bg] = name
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/theme/ -run TestThemeRegistryComplete -v`
Expected: FAIL — `Themes map has 3 entries, want 15`

- [ ] **Step 3: Update Themes map and ThemeNames**

Replace the `Themes` map and `ThemeNames` function in `internal/theme/themes.go`:

```go
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
	return []string{
		"catppuccin-latte", "catppuccin-mocha", "dracula",
		"everforest-dark", "everforest-light", "gruvbox-dark",
		"gruvbox-light", "kanagawa", "nord", "one-dark",
		"rose-pine", "rose-pine-dawn", "solarized-dark",
		"solarized-light", "tokyo-night",
	}
}
```

- [ ] **Step 4: Run all theme tests**

Run: `go test ./internal/theme/ -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/theme/themes.go internal/theme/theme_test.go
git commit -m "Update theme registry to 15 themes, alphabetical order"
```

---

### Task 8: Update poplar CLI (default, short flag)

**Files:**
- Modify: `cmd/poplar/root.go`

- [ ] **Step 1: Add `-t` short flag and change default to `one-dark`**

In `cmd/poplar/root.go`, replace the `StringVar` call:

```go
cmd.Flags().StringVarP(&f.theme, "theme", "t", "one-dark",
    "color theme ("+strings.Join(theme.ThemeNames(), ", ")+")")
```

`StringVarP` adds the `-t` short alias. The default changes from `"nord"` to `"one-dark"`.

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/poplar/`
Expected: success

- [ ] **Step 3: Verify flag works**

Run: `go run ./cmd/poplar/ --help`
Expected: output shows `-t, --theme string   color theme (catppuccin-latte, ...)`

- [ ] **Step 4: Commit**

```bash
git add cmd/poplar/root.go
git commit -m "Change poplar default theme to one-dark, add -t short flag"
```

---

### Task 9: Add `poplar themes` subcommand

**Files:**
- Create: `cmd/poplar/themes.go`
- Modify: `cmd/poplar/main.go`

- [ ] **Step 1: Create the themes subcommand**

Create `cmd/poplar/themes.go`:

```go
package main

import (
	"fmt"

	"github.com/glw907/beautiful-aerc/internal/theme"
	"github.com/spf13/cobra"
)

func newThemesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "themes",
		Short: "List available color themes",
		Run: func(cmd *cobra.Command, args []string) {
			for _, name := range theme.ThemeNames() {
				if name == "one-dark" {
					fmt.Printf("* %s (default)\n", name)
				} else {
					fmt.Printf("  %s\n", name)
				}
			}
		},
	}
}
```

- [ ] **Step 2: Register the subcommand**

In `cmd/poplar/main.go`, add the subcommand after creating the root command:

```go
func main() {
	cmd := newRootCmd()
	cmd.AddCommand(newThemesCmd())
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Verify output**

Run: `go run ./cmd/poplar/ themes`
Expected output:

```
  catppuccin-latte
  catppuccin-mocha
  dracula
  everforest-dark
  everforest-light
  gruvbox-dark
  gruvbox-light
  kanagawa
  nord
* one-dark (default)
  rose-pine
  rose-pine-dawn
  solarized-dark
  solarized-light
  tokyo-night
```

- [ ] **Step 4: Commit**

```bash
git add cmd/poplar/themes.go cmd/poplar/main.go
git commit -m "Add poplar themes subcommand"
```

---

### Task 10: Update mailrender to use shared registry

**Files:**
- Modify: `cmd/mailrender/themes.go`
- Modify: `cmd/mailrender/preview.go`

- [ ] **Step 1: Update themes.go generate command**

Replace the hardcoded map and list in `cmd/mailrender/themes.go` with the shared registry. Replace the `RunE` body:

```go
func newThemesGenerateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "generate [theme-name]",
		Short: "Generate aerc styleset from a compiled theme",
		Long:  "Available themes: " + strings.Join(theme.ThemeNames(), ", ") + ". Generates all if no name given.",
		RunE: func(cmd *cobra.Command, args []string) error {
			configDir, err := findConfigDir()
			if err != nil {
				return err
			}
			stylesetsDir := filepath.Join(configDir, "stylesets")
			if err := os.MkdirAll(stylesetsDir, 0755); err != nil {
				return fmt.Errorf("create stylesets dir: %w", err)
			}

			if len(args) > 0 {
				name := strings.ToLower(args[0])
				t, ok := theme.Themes[name]
				if !ok {
					return fmt.Errorf("unknown theme %q (available: %s)",
						args[0], strings.Join(theme.ThemeNames(), ", "))
				}
				return generateOne(t, stylesetsDir)
			}

			for _, name := range theme.ThemeNames() {
				if err := generateOne(theme.Themes[name], stylesetsDir); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
```

Add `"strings"` to the import block.

- [ ] **Step 2: Update preview.go resolveTheme**

Replace the `resolveTheme` function in `cmd/mailrender/preview.go`:

```go
func resolveTheme(name string) *theme.CompiledTheme {
	if t, ok := theme.Themes[strings.ToLower(name)]; ok {
		return t
	}
	return theme.OneDark
}
```

Add `"strings"` to the import block.

- [ ] **Step 3: Verify build**

Run: `go build ./cmd/mailrender/`
Expected: success

- [ ] **Step 4: Commit**

```bash
git add cmd/mailrender/themes.go cmd/mailrender/preview.go
git commit -m "Use shared theme registry in mailrender"
```

---

### Task 11: Update docs/themes.md

**Files:**
- Modify: `docs/themes.md`

- [ ] **Step 1: Update the built-in themes table**

Replace the "Built-in themes" section (lines 68-75) with all 15 themes:

```markdown
## Built-in themes

### Dark themes

| Theme | CLI name | Style |
|-------|----------|-------|
| `theme.OneDark` | `one-dark` | Neutral, muted (default) |
| `theme.Nord` | `nord` | Cool, arctic blue |
| `theme.SolarizedDark` | `solarized-dark` | Warm-neutral, classic |
| `theme.GruvboxDark` | `gruvbox-dark` | Warm, retro |
| `theme.CatppuccinMocha` | `catppuccin-mocha` | Pastel, warm |
| `theme.Dracula` | `dracula` | Purple, vibrant |
| `theme.TokyoNight` | `tokyo-night` | Cool, modern |
| `theme.RosePine` | `rose-pine` | Muted, warm |
| `theme.Kanagawa` | `kanagawa` | Cool, muted Japanese aesthetic |
| `theme.EverforestDark` | `everforest-dark` | Green, earthy |

### Light themes

| Theme | CLI name | Style |
|-------|----------|-------|
| `theme.CatppuccinLatte` | `catppuccin-latte` | Pastel, warm |
| `theme.SolarizedLight` | `solarized-light` | Warm-neutral, classic |
| `theme.GruvboxLight` | `gruvbox-light` | Warm, creamy |
| `theme.RosePineDawn` | `rose-pine-dawn` | Soft, warm |
| `theme.EverforestLight` | `everforest-light` | Green, soft |
```

- [ ] **Step 2: Update the custom theme instructions**

In the "Creating a custom theme" section, update step 3 to reference the shared registry:

```markdown
3. Add it to the `Themes` map and `ThemeNames()` in `internal/theme/themes.go`
```

(This replaces the reference to `cmd/mailrender/themes.go` and `cmd/mailrender/preview.go`, which now use the shared registry.)

- [ ] **Step 3: Commit**

```bash
git add docs/themes.md
git commit -m "Update theme reference with all 15 themes"
```

---

### Task 12: Update docs/styling.md

**Files:**
- Modify: `docs/styling.md`

- [ ] **Step 1: Update the principle section**

Replace the opening "Principle" section (lines 6-17) to remove TOML and Glamour references:

```markdown
## Principle

All styling flows from compiled themes through two rendering paths:

1. **Content pipeline** — `internal/filter/` converts raw email to
   normalized markdown. `internal/content/` parses it into blocks and
   renders with lipgloss styles from `CompiledTheme`.
2. **aerc styleset** — `mailrender themes generate` writes an aerc
   styleset file from the compiled theme's palette hex values.

Go code never assembles ANSI codes manually or hardcodes color values.

See [themes.md](themes.md) for the compiled theme reference.
```

- [ ] **Step 2: Commit**

```bash
git add docs/styling.md
git commit -m "Update styling docs to reflect compiled theme model"
```

---

### Task 13: Update architecture.md and STATUS.md

**Files:**
- Modify: `docs/poplar/architecture.md`
- Modify: `docs/poplar/STATUS.md`

- [ ] **Step 1: Add theme lineup decision to architecture.md**

Add to the "Key Decisions" section in `docs/poplar/architecture.md`:

```markdown
### 15 compiled themes with One Dark default
**Decision:** Ship 15 compiled themes (10 dark, 5 light). Default is
One Dark. Selection criteria: terminal ecosystem presence (kitty/alacritty
ports minimum) with popularity as tiebreaker.
**Rationale:** One Dark is neutral and familiar to millions of VS Code
users. The 15-theme lineup covers every major terminal color scheme
family. All themes are compiled Go values — no runtime config files.
`poplar themes` subcommand lists available themes.
**Date:** 2026-04-11
```

Also update the theme line in the Overview table from:

```
| Themes | `internal/theme/` | Compiled lipgloss themes (Nord, SolarizedDark, GruvboxDark) |
```

to:

```
| Themes | `internal/theme/` | Compiled lipgloss themes (15 themes, One Dark default) |
```

- [ ] **Step 2: Add theme spec link to STATUS.md**

Add to the Plans list in `docs/poplar/STATUS.md`:

```markdown
- [Theme selection spec](../superpowers/specs/2026-04-11-poplar-themes-design.md)
- [Theme selection plan](../superpowers/plans/2026-04-11-poplar-themes.md)
```

- [ ] **Step 3: Commit**

```bash
git add docs/poplar/architecture.md docs/poplar/STATUS.md
git commit -m "Add theme lineup decision and spec links to docs"
```

---

### Task 14: Final verification

- [ ] **Step 1: Run all tests**

Run: `make check`
Expected: all pass (vet + test)

- [ ] **Step 2: Build and install**

Run: `make install`
Expected: all four binaries install to `~/.local/bin/`

- [ ] **Step 3: Verify poplar themes**

Run: `poplar themes`
Expected: 15 themes listed alphabetically, `* one-dark (default)` marked

- [ ] **Step 4: Verify poplar launches with each theme flag variant**

Run: `poplar -t dracula` — verify launches, Ctrl-C to exit
Run: `poplar --theme catppuccin-mocha` — verify launches
Run: `poplar` — verify launches with One Dark (default)
Run: `poplar -t NORD` — verify case-insensitive lookup works

- [ ] **Step 5: Verify mailrender themes generate**

Run: `mailrender themes generate one-dark`
Expected: generates `~/.config/aerc/stylesets/One Dark`

- [ ] **Step 6: Commit any remaining changes**

If any files were missed, add and commit them.
