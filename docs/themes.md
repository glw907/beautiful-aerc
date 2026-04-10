# Themes

This document covers the theme system in detail — color slots, styles, custom theme creation, and styleset generation. For a quick overview of switching themes, see the [Theme system](../README.md#theme-system) section of the README.

beautiful-aerc's theme system drives colors across three layers from compiled Go values: the aerc UI (message list, sidebar, tabs via generated stylesets), the message viewer (headers, body, links via lipgloss styles), and optionally the kitty terminal and nvim-mail editor.

## Compiled themes

Themes are defined as Go struct literals in `internal/theme/themes.go`. Each theme is built from a `Palette` (16 hex color slots) via `NewCompiledTheme()`, which constructs all lipgloss styles at init time.

```go
var nordPalette = Palette{
    BgBase:          "#2e3440",
    BgElevated:      "#3b4252",
    // ... 14 more slots
}

var Nord = NewCompiledTheme("Nord", nordPalette)
```

There are no TOML files to load at runtime. The binary ships with compiled themes.

## The 16 color slots

Each `Palette` must define exactly these 16 slots. All values are 7-character hex strings (`#rrggbb`).

| Slot | Role |
|------|------|
| `BgBase` | Main background (message list, viewer pane) |
| `BgElevated` | Slightly lighter surface (completion dropdown background) |
| `BgSelection` | Selected row or region highlight |
| `BgBorder` | UI borders and separators |
| `FgBase` | Default text color |
| `FgBright` | Slightly brighter text (tab labels) |
| `FgBrightest` | Brightest foreground (MIME part names) |
| `FgDim` | Dimmed text (read messages, secondary info) |
| `AccentPrimary` | Primary highlight (title bar, focused selector, spinner) |
| `AccentSecondary` | Secondary highlight (unread messages, active tab) |
| `AccentTertiary` | Tertiary highlight (unread in dirlist, quoted text level 1, link URLs) |
| `ColorError` | Error state |
| `ColorWarning` | Warning state, flagged messages, folded threads |
| `ColorSuccess` | Success state, heading color |
| `ColorInfo` | Informational highlight, search results |
| `ColorSpecial` | Miscellaneous accent (answered/forwarded messages) |

## Lipgloss styles

`NewCompiledTheme()` builds the following lipgloss.Style fields on `CompiledTheme`:

| Style | Used for |
|-------|----------|
| `Paragraph` | Body paragraph text |
| `Heading` | Markdown headings |
| `Bold` | Bold inline spans |
| `Italic` | Italic inline spans |
| `CodeInline` | Inline code spans |
| `CodeBlock` | Fenced code blocks |
| `Link` | Link text (underlined) |
| `Quote` | Level-1 blockquotes |
| `DeepQuote` | Level-2+ blockquotes |
| `Attribution` | Quote attribution lines |
| `Signature` | Email signature blocks |
| `HorizontalRule` | Horizontal rules |
| `HeaderKey` | Header field names (From:, Subject:, etc.) |
| `HeaderValue` | Header field values |
| `HeaderDim` | Secondary header text (angle brackets, separators) |

## Built-in themes

| Theme | Style |
|-------|-------|
| `theme.Nord` | Cool dark (Arctic Ice Studio) |
| `theme.SolarizedDark` | Classic dark (Ethan Schoonover) |
| `theme.GruvboxDark` | Warm dark (morhetz) |

## Generating a styleset

The Go binaries use compiled themes directly, but aerc needs a static styleset file for its UI colors. Generate with:

```sh
mailrender themes generate nord
```

Or generate all themes at once:

```sh
mailrender themes generate
```

After generating, set `styleset-name` in `aerc.conf` to match the theme name.

## Creating a custom theme

1. Add a new `Palette` var in `internal/theme/themes.go` with 16 hex colors
2. Add a compiled theme: `var MyTheme = NewCompiledTheme("My Theme", myPalette)`
3. Register it in `cmd/mailrender/themes.go` and `cmd/mailrender/preview.go`
4. Generate the aerc styleset: `mailrender themes generate my-theme`
5. Preview with: `mailrender preview --theme my-theme corpus/some-email.html`

## Keeping kitty and nvim-mail in sync

The theme system covers the aerc layer only. kitty and nvim-mail have their own color systems.

**kitty:** Edit the `color0`–`color15` block in `.config/kitty/kitty-mail.conf` to match your theme's terminal palette. Most themes have published kitty color configs you can copy directly.

**nvim-mail:** The `aercmail` syntax colors are defined in `.config/nvim-mail/syntax/aercmail.vim`. Update the `guifg`/`guibg` values to match your theme's hex colors.

Neither file needs to be regenerated — edit them once per theme switch.
