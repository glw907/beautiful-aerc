# Themes

beautiful-aerc's theme system drives colors across three layers from a single source file: the aerc UI (message list, sidebar, tabs), the message viewer (header rendering, markdown highlighting, link colors), and optionally the kitty terminal and nvim-mail editor.

## The 16 color slots

Each theme defines 16 semantic hex color slots. These slots are the only place hex values appear - everything else references them by name.

| Slot | Role |
|------|------|
| `BG_BASE` | Main background (message list, viewer pane) |
| `BG_ELEVATED` | Slightly lighter surface (completion dropdown background) |
| `BG_SELECTION` | Selected row or region highlight |
| `BG_BORDER` | UI borders and separators |
| `FG_BASE` | Default text color |
| `FG_BRIGHT` | Slightly brighter text (tab labels) |
| `FG_BRIGHTEST` | Brightest foreground (MIME part names) |
| `FG_DIM` | Dimmed text (read messages, secondary info) |
| `ACCENT_PRIMARY` | Primary highlight (title bar, focused selector, spinner) |
| `ACCENT_SECONDARY` | Secondary highlight (unread messages, active tab, link text) |
| `ACCENT_TERTIARY` | Tertiary highlight (unread in dirlist, quoted text level 1, link URLs) |
| `COLOR_ERROR` | Error state |
| `COLOR_WARNING` | Warning state, flagged messages, folded threads |
| `COLOR_SUCCESS` | Success state, heading color (Nord: green) |
| `COLOR_INFO` | Informational highlight, search results |
| `COLOR_SPECIAL` | Miscellaneous accent (answered/forwarded messages, special purpose) |

## Markdown tokens

Below the color slots, each theme defines markdown tokens that control message viewer syntax highlighting. Tokens reference slots by variable name and can include style modifiers.

| Token | Controls |
|-------|----------|
| `C_HEADING` | `#`, `##`, `###` heading lines |
| `C_BOLD` | `**bold**` text |
| `C_ITALIC` | `_italic_` text |
| `C_LINK_TEXT` | Link text in `[text](url)` |
| `C_LINK_URL` | URL in `[text](url)` (markdown links mode) |
| `C_RULE` | Horizontal rule (`---`) |

Token format: a slot reference (like `$ACCENT_SECONDARY`), an optional style modifier (`bold`, `italic`, `underline`), or both:

```sh
C_HEADING="$COLOR_SUCCESS bold"   # color + bold
C_BOLD="bold"                     # modifier only
C_LINK_TEXT="$ACCENT_SECONDARY"   # color only
```

The generator resolves slot references and converts them to ANSI escape parameters. At runtime, the Go binary reads these resolved values from `generated/palette.sh`.

## Built-in themes

| File | Theme | Style |
|------|-------|-------|
| `themes/nord.sh` | Nord | Cool dark (Arctic Ice Studio) |
| `themes/solarized-dark.sh` | Solarized Dark | Classic dark (Ethan Schoonover) |
| `themes/gruvbox-dark.sh` | Gruvbox Dark | Warm dark (morhetz) |

## Creating a custom theme

Copy an existing theme as a starting point:

```sh
cp .config/aerc/themes/nord.sh .config/aerc/themes/my-theme.sh
```

Edit the 16 hex values to match your color scheme. Keep the slot names exactly as-is - the generator and styleset both depend on them by name.

Adjust the markdown tokens to taste. You can reference any of the 16 slots, add modifiers, or combine them:

```sh
C_HEADING="$ACCENT_PRIMARY bold"
C_LINK_TEXT="$COLOR_INFO"
C_LINK_URL="$FG_DIM"
```

## Running the generator

Run from inside `.config/aerc/`:

```sh
cd .config/aerc
themes/generate themes/my-theme.sh
```

Or from anywhere, with a path:

```sh
~/.config/aerc/themes/generate ~/.config/aerc/themes/nord.sh
```

The generator produces two files:

**`generated/palette.sh`** - hex values for all 16 slots, plus ANSI-encoded token values for the Go binary. This file is read at runtime by `beautiful-aerc`.

**`stylesets/<theme-name>`** - an aerc styleset file with hex values for every UI element. After generating, set `styleset-name` in `aerc.conf` to match:

```ini
styleset-name=my-theme
```

The generator output looks like:

```
Theme:    themes/my-theme.sh
Palette:  generated/palette.sh
Styleset: stylesets/my-theme
```

## Override mechanism

Both generated files include a marker line near the bottom:

```sh
# --- overrides below this line are preserved across regeneration ---
```

Any content you add below this line survives re-running the generator. Use this to adjust individual values without touching the theme source file.

For example, to make flagged messages use a brighter red in the styleset:

```ini
# --- overrides below this line are preserved across regeneration ---
msglist_flagged.fg=#ff0000
msglist_flagged.bold=true
```

Or to override a single palette token:

```sh
# --- overrides below this line are preserved across regeneration ---
C_HEADING="1;38;2;255;200;100"   # custom heading color, ANSI format
```

Override values must use ANSI parameter format (e.g., `38;2;R;G;B` for RGB color, `1` for bold) since that is what the Go binary reads.

## Keeping kitty and nvim-mail in sync

The theme generator only covers the aerc layer. kitty and nvim-mail have their own color systems and do not read `palette.sh` at runtime.

**kitty:** Edit the `color0`-`color15` block in `.config/kitty/kitty-mail.conf` to match your theme's terminal palette. Most themes have published kitty color configs you can copy directly.

**nvim-mail:** The `aercmail` syntax colors are defined in `.config/nvim-mail/syntax/aercmail.vim`. Vim syntax files use hardcoded color values. Update the `guifg`/`guibg` values in that file to match your theme's hex colors.

Neither file needs to be regenerated - edit them once per theme switch.
