# Theme System Redesign: Shell to TOML

## Goal

Replace the shell-based theme generation pipeline (`themes/generate`
shell script + `generated/palette.sh` intermediary) with direct TOML
theme file loading in Go, eliminating the generated palette file
entirely and moving styleset generation to a Go subcommand.

## Architecture

Go binaries read `.toml` theme files directly at runtime, resolving
color references and style modifiers at startup. The only generated
artifact is the aerc styleset, produced on demand by an explicit
`mailrender themes generate` command. The active theme name comes
from `styleset-name` in `aerc.conf`.

## Theme File Format

Theme files live in `.config/aerc/themes/` as TOML. Three ship with
the project: `nord.toml`, `solarized-dark.toml`, `gruvbox-dark.toml`.

```toml
name = "Nord"

[colors]
bg_base = "#2e3440"
bg_elevated = "#3b4252"
bg_selection = "#394353"
bg_border = "#49576b"
fg_base = "#d8dee9"
fg_bright = "#e5e9f0"
fg_brightest = "#eceff4"
fg_dim = "#616e88"
accent_primary = "#81a1c1"
accent_secondary = "#88c0d0"
accent_tertiary = "#8fbcbb"
color_error = "#bf616a"
color_warning = "#d08770"
color_success = "#a3be8c"
color_info = "#ebcb8b"
color_special = "#b48ead"

[tokens]
heading = { color = "color_success", bold = true }
bold = { bold = true }
italic = { italic = true }
link_text = { color = "accent_primary", underline = true }
link_url = { color = "fg_dim" }
rule = { color = "fg_dim" }
hdr_key = { color = "accent_primary", bold = true }
hdr_value = { color = "fg_base" }
hdr_dim = { color = "fg_dim" }
picker_num = { color = "accent_primary" }
picker_label = { color = "fg_base" }
picker_url = { color = "fg_dim" }
picker_sel_bg = { color = "bg_selection" }
picker_sel_fg = { color = "fg_bright" }
msg_marker = { color = "fg_dim", bold = true }
msg_title_success = { color = "color_success", bold = true }
msg_title_accent = { color = "accent_primary", bold = true }
msg_detail = { color = "fg_base" }
msg_dim = { color = "fg_dim" }
```

### Format rules

- All 16 color slots in `[colors]` are required. Missing slots cause
  a load error.
- Color values must be `#rrggbb` hex strings.
- Token `color` fields reference keys in `[colors]` by name. Unknown
  references cause a load error.
- Modifier fields (`bold`, `italic`, `underline`) are optional
  booleans, default false.
- Tokens without a `color` field are modifier-only (e.g., `bold`).
- No override markers, no inheritance, no shell variable expansion.

## `internal/theme` Package

Replaces `internal/palette/`. The package loads TOML theme files,
validates them, resolves tokens to ANSI escape sequences at load
time, and exposes a simple lookup API.

### Types

```go
type Theme struct {
    Name   string
    colors map[string]string // "accent_primary" → "#81a1c1"
    tokens map[string]string // "hdr_key" → "38;2;129;161;193;1"
}
```

### API

```go
// Load reads and validates a TOML theme file. Resolves all tokens
// to ANSI SGR parameter strings at load time.
func Load(path string) (*Theme, error)

// ANSI returns "\033[<params>m" for a token, or "" if not found.
func (t *Theme) ANSI(name string) string

// Color returns the hex value for a color slot, or "" if not found.
func (t *Theme) Color(name string) string

// Reset returns "\033[0m".
func (t *Theme) Reset() string

// FindPath locates the active theme file. Reads styleset-name from
// aerc.conf, then looks for themes/<name>.toml in $AERC_CONFIG or
// ~/.config/aerc/.
func FindPath() (string, error)
```

### Token resolution

At load time, each token is resolved to an ANSI SGR parameter string:

1. If `color` is set, look up the hex value in `[colors]`, convert
   to `38;2;R;G;B` using the existing `HexToANSI` logic.
2. Append modifier codes: bold=1, italic=3, underline=4.
3. Join with `;` and store in the `tokens` map.

Example: `hdr_key = { color = "accent_primary", bold = true }` with
`accent_primary = "#81a1c1"` resolves to `"38;2;129;161;193;1"`.

### Theme discovery

`FindPath` locates the active theme:

1. Find `aerc.conf`: check `$AERC_CONFIG/aerc.conf`, then
   `~/.config/aerc/aerc.conf`.
2. Parse `styleset-name=<name>` from `aerc.conf` (simple line scan,
   no full INI parser needed — the key appears once in `[ui]`).
3. Look for `themes/<name>.toml` relative to the `aerc.conf`
   location.

Error messages guide the user if any step fails (no aerc.conf, no
styleset-name, no matching .toml file).

## Styleset Generation

New cobra subcommand: `mailrender themes generate [theme-name]`.

- If `theme-name` is given, loads `themes/<name>.toml`.
- If omitted, reads `styleset-name` from `aerc.conf`.
- Validates the theme via `theme.Load()`.
- Writes `stylesets/<name>` with hex color values substituted into
  an embedded Go template.
- The template is a string constant in Go source reproducing the
  current styleset format (aerc INI-style with `[ui]` and `[viewer]`
  sections).

### Styleset template

The template references color slot names. During generation, each
`{{.ColorName}}` placeholder is replaced with the corresponding hex
value from `[colors]`. The template produces identical output to
what the shell script produces today.

Styleset sections:
- `[ui]` — status bar, title bar, completion, message list, tabs,
  directory list, part display, borders, spinner, selectors
- `[viewer]` — headers, quote levels, signature, URLs, diff colors

## Caller Migration

Four call sites across three files:

| File | Before | After |
|------|--------|-------|
| `cmd/mailrender/headers.go` | `palette.FindPath()` + `palette.Load()` | `theme.FindPath()` + `theme.Load()` |
| `cmd/mailrender/headers.go` | `p.ANSI("C_HDR_KEY")` | `t.ANSI("hdr_key")` |
| `cmd/pick-link/root.go` | `palette.FindPath()` + `palette.Load()` | `theme.FindPath()` + `theme.Load()` |
| `internal/filter/html.go` | `p.Get("C_LINK_TEXT")` | `t.ANSI("link_text")` |
| `internal/filter/html.go` | `palette.HexToANSI(p.Get("FG_DIM"))` | `t.ANSI("msg_dim")` (same color, avoids manual hex conversion) |
| `internal/picker/picker.go` | `p.ANSI("C_PICKER_NUM")` | `t.ANSI("picker_num")` |
| `internal/picker/picker.go` | `p.Get("C_PICKER_SEL_BG")` | `t.ANSI("picker_sel_bg")` (raw SGR, picker swaps 38→48 for BG) |

All `C_*` prefix references become lowercase without prefix. Every
runtime caller uses `ANSI()` — no caller needs raw hex values at
runtime. `Color()` exists for styleset generation, which substitutes
hex values into the aerc styleset template.

## Files Deleted

- `.config/aerc/themes/generate` — 285-line shell script
- `.config/aerc/generated/palette.sh` — generated intermediary
- `.config/aerc/generated/` — directory (no longer needed)
- `.config/aerc/themes/nord.sh` — replaced by `nord.toml`
- `.config/aerc/themes/solarized-dark.sh` — replaced by `.toml`
- `.config/aerc/themes/gruvbox-dark.sh` — replaced by `.toml`
- `internal/palette/palette.go` — replaced by `internal/theme/`
- `internal/palette/palette_test.go` — replaced by theme tests

## Files Created

- `internal/theme/theme.go` — theme loading, validation, resolution
- `internal/theme/theme_test.go` — unit tests
- `internal/theme/styleset.go` — styleset template and generation
- `internal/theme/styleset_test.go` — generation tests
- `cmd/mailrender/themes.go` — cobra `themes generate` subcommand
- `.config/aerc/themes/nord.toml`
- `.config/aerc/themes/solarized-dark.toml`
- `.config/aerc/themes/gruvbox-dark.toml`

## Files Modified

- `cmd/mailrender/headers.go` — import + API migration
- `cmd/pick-link/root.go` — import + API migration
- `internal/filter/html.go` — import + API migration
- `internal/picker/picker.go` — import + API migration
- `e2e/` test setup — provide `.toml` theme + minimal `aerc.conf`
  instead of `palette.sh`
- `Makefile` — no change (mailrender already built)
- `go.mod` — add TOML dependency
- `CLAUDE.md` — update theme system docs
- `README.md` — update theme section
- `docs/themes.md` — rewrite for TOML format
- `~/.claude/docs/aerc-setup.md` — update theme system section

## User-Facing Workflow

### Switching themes (after)

```sh
# Edit styleset-name in aerc.conf
mailrender themes generate
# Restart aerc
```

### Creating a custom theme

1. Copy `themes/nord.toml` to `themes/mytheme.toml`
2. Edit hex values in `[colors]`
3. Optionally adjust `[tokens]`
4. Set `styleset-name=mytheme` in `aerc.conf`
5. Run `mailrender themes generate`

### Daily use

No generation step. Go binaries read `themes/<name>.toml` directly
on every invocation. Change a hex value in the theme file, restart
aerc, see the change immediately in rendered messages.

## Testing

### `internal/theme/` unit tests

- Load valid TOML: all 16 colors parsed correctly
- Load valid TOML: all tokens resolved to correct ANSI strings
- Token with color + bold: verify combined SGR string
- Token with only modifiers (no color): verify modifier-only string
- Missing required color slot: returns error
- Invalid hex value: returns error
- Token referencing undefined color: returns error
- `ANSI()` wraps in escape sequences correctly
- `Color()` returns raw hex value
- `FindPath` with `AERC_CONFIG` env var
- `FindPath` reads `styleset-name` from `aerc.conf`
- `FindPath` with missing aerc.conf: clear error

### Styleset generation tests

- Generate from valid theme: output contains correct hex values
- Verify all color slots appear in output
- Output matches aerc styleset INI format

### E2E tests

Existing `e2e/` golden-file tests continue unchanged. Test setup
provides a `.toml` theme file and minimal `aerc.conf` instead of
`palette.sh`. Golden output files do not change — the ANSI output
is identical.

## Dependencies

- TOML parser: `github.com/BurntSushi/toml` — the standard Go TOML
  library, widely used, minimal dependency tree.
- No other new dependencies. `HexToANSI` logic already exists (moves
  from `internal/palette` to `internal/theme`).

## No Backward Compatibility

This is a clean cut. The shell theme files, palette.sh, and the
generate script are all deleted. Users must convert their theme
files to TOML (the three shipped themes are converted as part of
this work). Custom user themes follow the same conversion — the
format mapping is 1:1.
