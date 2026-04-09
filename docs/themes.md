# Themes

beautiful-aerc's theme system drives colors across three layers from a single source file: the aerc UI (message list, sidebar, tabs), the message viewer (header rendering, markdown highlighting, link colors), and optionally the kitty terminal and nvim-mail editor.

## Theme file format

Theme files live in `.config/aerc/themes/` and use TOML. Each file has three parts: a `name` field, a `[colors]` section with 16 required slots, and a `[tokens]` section with style definitions.

```toml
name = "Nord"

[colors]
bg_base       = "#2e3440"
bg_elevated   = "#3b4252"
bg_selection  = "#394353"
bg_border     = "#49576b"
fg_base       = "#d8dee9"
fg_bright     = "#e5e9f0"
fg_brightest  = "#eceff4"
fg_dim        = "#616e88"
accent_primary   = "#81a1c1"
accent_secondary = "#88c0d0"
accent_tertiary  = "#8fbcbb"
color_error   = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info    = "#ebcb8b"
color_special = "#b48ead"

[tokens]
heading  = { color = "color_success", bold = true }
bold     = { bold = true }
italic   = { italic = true }
link_text = { color = "accent_primary", underline = true }
link_url  = { color = "fg_dim" }
rule      = { color = "fg_dim" }
hdr_key   = { color = "accent_primary", bold = true }
hdr_value = { color = "fg_base" }
hdr_dim   = { color = "fg_dim" }
# ... etc
```

At startup, `mailrender` reads the active `.toml` file directly — there is no intermediate palette file. Tokens are resolved to ANSI escape sequences in memory.

## The 16 color slots

The `[colors]` section must define exactly these 16 slots. All values must be 7-character hex strings (`#rrggbb`).

| Slot | Role |
|------|------|
| `bg_base` | Main background (message list, viewer pane) |
| `bg_elevated` | Slightly lighter surface (completion dropdown background) |
| `bg_selection` | Selected row or region highlight |
| `bg_border` | UI borders and separators |
| `fg_base` | Default text color |
| `fg_bright` | Slightly brighter text (tab labels) |
| `fg_brightest` | Brightest foreground (MIME part names) |
| `fg_dim` | Dimmed text (read messages, secondary info) |
| `accent_primary` | Primary highlight (title bar, focused selector, spinner) |
| `accent_secondary` | Secondary highlight (unread messages, active tab) |
| `accent_tertiary` | Tertiary highlight (unread in dirlist, quoted text level 1, link URLs) |
| `color_error` | Error state |
| `color_warning` | Warning state, flagged messages, folded threads |
| `color_success` | Success state, heading color |
| `color_info` | Informational highlight, search results |
| `color_special` | Miscellaneous accent (answered/forwarded messages) |

## Token definitions

The `[tokens]` section maps token names to style definitions. Each definition can specify a color slot reference and style modifiers.

Token format:

```toml
token_name = { color = "slot_name", bold = true, italic = true, underline = true }
```

- **`color`** — must be one of the 16 color slot names from `[colors]`. Omit for modifier-only tokens.
- **`bold`**, **`italic`**, **`underline`** — boolean modifiers, all optional, all default to `false`.

Examples:

```toml
heading   = { color = "color_success", bold = true }   # color + bold
bold      = { bold = true }                             # modifier only
link_text = { color = "accent_primary", underline = true }
hdr_dim   = { color = "fg_dim" }                       # color only
```

### Markdown tokens

| Token | Controls |
|-------|----------|
| `heading` | `#`, `##`, `###` heading lines |
| `bold` | `**bold**` text |
| `italic` | `_italic_` text |
| `link_text` | Link text in footnote references |
| `link_url` | URL in footnote reference section |
| `rule` | Horizontal rule (`---`) |

### Header tokens

| Token | Controls |
|-------|----------|
| `hdr_key` | Header field names (From, Subject, etc.) |
| `hdr_value` | Header field values (names, text) |
| `hdr_dim` | Secondary header text (angle brackets, separators) |

### Link picker tokens

| Token | Controls |
|-------|----------|
| `picker_num` | Picker shortcut digits (1–9, 0) |
| `picker_label` | Picker link label text |
| `picker_url` | Picker URL text |
| `picker_sel_bg` | Picker selected row background |
| `picker_sel_fg` | Picker selected row foreground |

### Message UI tokens

| Token | Controls |
|-------|----------|
| `msg_marker` | Message heading `#` marker |
| `msg_title_success` | Success heading (confirmations) |
| `msg_title_accent` | Interactive heading (picker, prompts) |
| `msg_detail` | Message detail text (filenames, labels) |
| `msg_dim` | Message secondary text (counts, hints) |

## Built-in themes

| File | Theme name | Style |
|------|------------|-------|
| `themes/nord.toml` | Nord | Cool dark (Arctic Ice Studio) |
| `themes/solarized-dark.toml` | Solarized Dark | Classic dark (Ethan Schoonover) |
| `themes/gruvbox-dark.toml` | Gruvbox Dark | Warm dark (morhetz) |

## Generating a styleset

The Go binaries read theme colors directly at runtime, but aerc needs a static styleset file for its UI colors. Generate one with:

```sh
mailrender themes generate nord
```

This writes `stylesets/Nord` to your aerc config directory and prints:

```
Theme:    nord.toml
Styleset: stylesets/Nord
```

To generate the active theme (determined by `styleset-name` in `aerc.conf`):

```sh
mailrender themes generate
```

After generating, set `styleset-name` in `aerc.conf` to the theme's `name` value (case-sensitive, matching the `name =` field in the `.toml` file):

```ini
styleset-name=Nord
```

## Creating a custom theme

Copy an existing theme as a starting point:

```sh
cp .config/aerc/themes/nord.toml .config/aerc/themes/my-theme.toml
```

Edit the `name` field and the 16 hex values in `[colors]`. Keep the slot names exactly as-is — they are required by name. Adjust the token definitions in `[tokens]` as needed.

Generate the aerc styleset:

```sh
mailrender themes generate my-theme
```

Then set `styleset-name` in `aerc.conf` to the `name` value from your `.toml` file.

## How themes are discovered

At runtime, `mailrender` locates the active theme by:

1. Finding the aerc config directory — checks `$AERC_CONFIG` first, then `~/.config/aerc/`
2. Reading `styleset-name` from `aerc.conf` in that directory
3. Loading `themes/<styleset-name>.toml` from the same config directory

The lookup is case-sensitive and must match the filename exactly (without the `.toml` extension).

## Keeping kitty and nvim-mail in sync

The theme system covers the aerc layer only. kitty and nvim-mail have their own color systems and do not read the TOML theme at runtime.

**kitty:** Edit the `color0`–`color15` block in `.config/kitty/kitty-mail.conf` to match your theme's terminal palette. Most themes have published kitty color configs you can copy directly.

**nvim-mail:** The `aercmail` syntax colors are defined in `.config/nvim-mail/syntax/aercmail.vim`. Vim syntax files use hardcoded color values. Update the `guifg`/`guibg` values in that file to match your theme's hex colors.

Neither file needs to be regenerated — edit them once per theme switch.
