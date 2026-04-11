# Poplar Theme Selection Design

Ship poplar with a curated set of compiled themes selectable via
`--theme` flag. One binary, many looks.

## Decisions

- **Default**: One Dark (neutral, inoffensive, familiar to millions)
- **Selection criteria**: terminal ecosystem presence (kitty/alacritty
  ports minimum), popularity as tiebreaker
- **Dark + light**: both supported
- **Count**: 15 themes (10 dark, 5 light)
- **No runtime theme files**: all themes compiled as Go values

## CLI Interface

```
poplar                          # launches with one-dark (default)
poplar -t dracula               # short flag
poplar --theme catppuccin-mocha # long flag
poplar themes                   # list available themes (subcommand)
```

The `themes` subcommand prints available theme names, one per line,
with the default marked. This is the idiomatic Go/cobra pattern
for listing resources (`docker images`, `git remote`).

Flag names are kebab-case, lowercase, no spaces. Case-insensitive
lookup on input.

## Theme Lineup

### Dark (10)

| CLI name | Display name | Character |
|----------|-------------|-----------|
| one-dark | One Dark | Neutral, muted. Default. |
| nord | Nord | Cool, arctic blue |
| solarized-dark | Solarized Dark | Warm-neutral, classic |
| gruvbox-dark | Gruvbox Dark | Warm, retro |
| catppuccin-mocha | Catppuccin Mocha | Pastel, warm |
| dracula | Dracula | Purple, vibrant |
| tokyo-night | Tokyo Night | Cool, modern |
| rose-pine | Rose Pine | Muted, warm |
| kanagawa | Kanagawa | Cool, muted Japanese aesthetic |
| everforest-dark | Everforest Dark | Green, earthy |

### Light (5)

| CLI name | Display name | Character |
|----------|-------------|-----------|
| catppuccin-latte | Catppuccin Latte | Pastel, warm |
| solarized-light | Solarized Light | Warm-neutral, classic |
| gruvbox-light | Gruvbox Light | Warm, creamy |
| rose-pine-dawn | Rose Pine Dawn | Soft, warm |
| everforest-light | Everforest Light | Green, soft |

## Palette Mappings

Each theme maps to the 16-slot `Palette` struct. The mapping
requires judgment — source themes have 20-60 colors and our
semantic slots compress that into 16. The principles:

- **bg_base**: primary background
- **bg_elevated**: raised surfaces (popups, completion)
- **bg_selection**: selected row highlight
- **bg_border**: borders, inactive elements
- **fg_base**: primary text
- **fg_bright**: emphasized text
- **fg_brightest**: maximum contrast text
- **fg_dim**: de-emphasized text (comments, read messages)
- **accent_primary**: primary brand color (header keys, titles)
- **accent_secondary**: secondary accent (selected items, unread)
- **accent_tertiary**: tertiary accent (quotes, urls)
- **color_error**: errors, deleted
- **color_warning**: warnings, flagged
- **color_success**: success, additions
- **color_info**: search results, highlights
- **color_special**: answered/forwarded, special states

### One Dark (default)

```
bg_base          = #282c34
bg_elevated      = #31353f
bg_selection     = #393f4a
bg_border        = #5c6370
fg_base          = #abb2bf
fg_bright        = #ced4de
fg_brightest     = #e6e8ee
fg_dim           = #5c6370
accent_primary   = #61afef
accent_secondary = #56b6c2
accent_tertiary  = #98c379
color_error      = #e86671
color_warning    = #d19a66
color_success    = #98c379
color_info       = #e5c07b
color_special    = #c678dd
```

### Catppuccin Mocha

```
bg_base          = #1e1e2e
bg_elevated      = #313244
bg_selection     = #45475a
bg_border        = #585b70
fg_base          = #cdd6f4
fg_bright        = #bac2de
fg_brightest     = #f5e0dc
fg_dim           = #6c7086
accent_primary   = #89b4fa
accent_secondary = #94e2d5
accent_tertiary  = #a6e3a1
color_error      = #f38ba8
color_warning    = #fab387
color_success    = #a6e3a1
color_info       = #f9e2af
color_special    = #cba6f7
```

### Catppuccin Latte

```
bg_base          = #eff1f5
bg_elevated      = #ccd0da
bg_selection     = #bcc0cc
bg_border        = #acb0be
fg_base          = #4c4f69
fg_bright        = #5c5f77
fg_brightest     = #11111b
fg_dim           = #9ca0b0
accent_primary   = #1e66f5
accent_secondary = #179299
accent_tertiary  = #40a02b
color_error      = #d20f39
color_warning    = #fe640b
color_success    = #40a02b
color_info       = #df8e1d
color_special    = #8839ef
```

### Dracula

```
bg_base          = #282a36
bg_elevated      = #44475a
bg_selection     = #44475a
bg_border        = #6272a4
fg_base          = #f8f8f2
fg_bright        = #f8f8f2
fg_brightest     = #ffffff
fg_dim           = #6272a4
accent_primary   = #bd93f9
accent_secondary = #8be9fd
accent_tertiary  = #50fa7b
color_error      = #ff5555
color_warning    = #ffb86c
color_success    = #50fa7b
color_info       = #f1fa8c
color_special    = #ff79c6
```

### Tokyo Night

```
bg_base          = #1a1b26
bg_elevated      = #16161e
bg_selection     = #283457
bg_border        = #3b4261
fg_base          = #c0caf5
fg_bright        = #a9b1d6
fg_brightest     = #c0caf5
fg_dim           = #565f89
accent_primary   = #7aa2f7
accent_secondary = #7dcfff
accent_tertiary  = #9ece6a
color_error      = #f7768e
color_warning    = #ff9e64
color_success    = #9ece6a
color_info       = #e0af68
color_special    = #bb9af7
```

### Rose Pine

```
bg_base          = #191724
bg_elevated      = #1f1d2e
bg_selection     = #403d52
bg_border        = #524f67
fg_base          = #e0def4
fg_bright        = #e0def4
fg_brightest     = #e0def4
fg_dim           = #6e6a86
accent_primary   = #c4a7e7
accent_secondary = #9ccfd8
accent_tertiary  = #31748f
color_error      = #eb6f92
color_warning    = #f6c177
color_success    = #9ccfd8
color_info       = #f6c177
color_special    = #c4a7e7
```

### Rose Pine Dawn

```
bg_base          = #faf4ed
bg_elevated      = #fffaf3
bg_selection     = #dfdad9
bg_border        = #cecacd
fg_base          = #575279
fg_bright        = #464261
fg_brightest     = #26233a
fg_dim           = #9893a5
accent_primary   = #907aa9
accent_secondary = #56949f
accent_tertiary  = #286983
color_error      = #b4637a
color_warning    = #ea9d34
color_success    = #56949f
color_info       = #ea9d34
color_special    = #907aa9
```

### Kanagawa

```
bg_base          = #1F1F28
bg_elevated      = #2A2A37
bg_selection     = #223249
bg_border        = #54546D
fg_base          = #DCD7BA
fg_bright        = #C8C093
fg_brightest     = #DCD7BA
fg_dim           = #727169
accent_primary   = #7E9CD8
accent_secondary = #7FB4CA
accent_tertiary  = #6A9589
color_error      = #E82424
color_warning    = #FF9E3B
color_success    = #98BB6C
color_info       = #E6C384
color_special    = #957FB8
```

### Everforest Dark

```
bg_base          = #2d353b
bg_elevated      = #343f44
bg_selection     = #3d484d
bg_border        = #7a8478
fg_base          = #d3c6aa
fg_bright        = #d3c6aa
fg_brightest     = #e6ddc4
fg_dim           = #859289
accent_primary   = #7fbbb3
accent_secondary = #83c092
accent_tertiary  = #a7c080
color_error      = #e67e80
color_warning    = #e69875
color_success    = #a7c080
color_info       = #dbbc7f
color_special    = #d699b6
```

### Everforest Light

```
bg_base          = #fdf6e3
bg_elevated      = #f4f0d9
bg_selection     = #efebd4
bg_border        = #a6b0a0
fg_base          = #5c6a72
fg_bright        = #5c6a72
fg_brightest     = #3c474d
fg_dim           = #939f91
accent_primary   = #3a94c5
accent_secondary = #35a77c
accent_tertiary  = #8da101
color_error      = #f85552
color_warning    = #f57d26
color_success    = #8da101
color_info       = #dfa000
color_special    = #df69ba
```

### Solarized Light

```
bg_base          = #fdf6e3
bg_elevated      = #eee8d5
bg_selection     = #eee8d5
bg_border        = #93a1a1
fg_base          = #657b83
fg_bright        = #586e75
fg_brightest     = #073642
fg_dim           = #93a1a1
accent_primary   = #268bd2
accent_secondary = #2aa198
accent_tertiary  = #2aa198
color_error      = #dc322f
color_warning    = #cb4b16
color_success    = #859900
color_info       = #b58900
color_special    = #6c71c4
```

### Gruvbox Light

```
bg_base          = #fbf1c7
bg_elevated      = #ebdbb2
bg_selection     = #d5c4a1
bg_border        = #a89984
fg_base          = #3c3836
fg_bright        = #504945
fg_brightest     = #1d2021
fg_dim           = #928374
accent_primary   = #076678
accent_secondary = #427b58
accent_tertiary  = #79740e
color_error      = #9d0006
color_warning    = #af3a03
color_success    = #79740e
color_info       = #b57614
color_special    = #8f3f71
```

## Implementation

### Theme registry (`internal/theme/themes.go`)

Add 12 new palette vars and compiled theme vars. Update the
`Themes` map and `ThemeNames()` to include all 15. Change the
default from `nord` to `one-dark` in the cobra flag definition.

### Subcommand (`cmd/poplar/themes.go`)

New `poplar themes` subcommand that lists available themes:

```
$ poplar themes
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

Alphabetical, `*` marks the default.

### CLI changes (`cmd/poplar/root.go`)

- Add `-t` short flag alias for `--theme`
- Change default from `"nord"` to `"one-dark"`
- Case-insensitive theme lookup (already implemented)

### Styleset generation

`mailrender themes generate` already works with any
`*CompiledTheme`. The new themes are automatically available
for aerc styleset generation by name.

## Documentation Updates

- **`docs/poplar/architecture.md`**: Add decision record for theme
  lineup (15 compiled themes, One Dark default, selection criteria,
  `poplar themes` subcommand).
- **`docs/poplar/STATUS.md`**: Update plans list with this spec.
- **`docs/themes.md`**: Add all 15 themes to the theme reference.
  Currently documents Nord, Solarized Dark, Gruvbox Dark only.
- **`docs/styling.md`**: Update if it references the three-theme
  limitation or hardcoded Nord default.

## Testing

- Existing `palette_test.go` validates palette → compiled theme
  construction. Extend to cover all 15 themes.
- Verify each theme compiles without panic (non-empty palette
  values, valid hex).
- Manual: launch `poplar -t <name>` for each theme, confirm
  colors render correctly on a dark and light terminal background.
