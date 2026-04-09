# Filters

beautiful-aerc replaces aerc's default filter pipeline with a single Go binary that handles header rendering, HTML messages, and plain text. Each subcommand reads from stdin and writes ANSI-styled text to stdout, which is aerc's filter protocol.

## The three subcommands

| Subcommand | aerc hook | Calls pandoc? |
|------------|-----------|---------------|
| `beautiful-aerc headers` | `.headers` | No |
| `beautiful-aerc html` | `text/html` | Yes |
| `beautiful-aerc plain` | `text/plain` | No (unless HTML detected) |

These are configured in `aerc.conf`:

```ini
[filters]
.headers=beautiful-aerc headers
text/html=beautiful-aerc html
text/plain=beautiful-aerc plain
```

## HTML pipeline

When aerc opens an HTML message, it pipes the raw HTML body to `beautiful-aerc html`. The pipeline runs in stages:

**1. Artifact cleanup (pre-pandoc)**

Before passing HTML to pandoc, the binary strips known junk that produces bad markdown output:

- Moz-specific HTML attributes (`class="moz-..."`, `data-moz-do-not-send`)
- These attributes cause pandoc to emit escaped spans in the markdown

**2. pandoc conversion**

The binary calls pandoc as a subprocess:

```sh
pandoc -f html -t markdown --wrap=none -L unwrap-tables.lua
```

`unwrap-tables.lua` is a pandoc Lua filter that flattens nested HTML tables into plain text instead of letting pandoc render them as markdown tables. Marketing emails often use layout tables, not data tables - this produces much cleaner output.

**3. Artifact cleanup (post-pandoc)**

pandoc's markdown output contains artifacts that don't render cleanly in a terminal:

- Trailing backslashes at line ends (pandoc's line-break marker)
- Backslash-escaped punctuation (e.g., `\.`, `\-`, `\[`)
- Non-breaking spaces (replaced with regular spaces)
- Zero-width characters (`U+200B`, `U+200C`, `U+FEFF`) - removed
- Lines containing only spaces - stripped to blank
- Three or more consecutive blank lines - collapsed to two

Image and empty-link cleanup happens inside `convertToFootnotes` rather than as a separate regex pass, because pandoc's `--reference-links` output is reference-style markdown, not inline-style.

**4. Markdown syntax highlighting**

The cleaned markdown is scanned line by line. Elements matching markdown syntax get ANSI color applied:

- Lines starting with `#`, `##`, `###` get `C_HEADING` color
- `**text**` spans get `C_BOLD` style
- `_text_` spans get `C_ITALIC` style
- Horizontal rules (`---`, `***`, `___`) get `C_RULE` color

Colors come from `generated/palette.sh`. See [docs/themes.md](themes.md) for the token reference.

**5. Footnote-style links**

Links are rendered as footnote references. Body text stays clean with colored link text and dimmed footnote markers. A numbered reference section at the bottom lists all URLs.

Self-referencing links (where the display text is the URL itself) render as plain URLs with no footnote number.

pandoc is called with `--reference-links` to produce reference-style output. `convertToFootnotes` handles the full conversion: numbering refs, replacing body references, stripping emphasis markers from link display text (pandoc wraps linked `<em>` text in `*...*`), rendering images with alt text as `[image: alt text]` labels, removing images without alt text, and stripping brackets from unresolved references. `styleFootnotes` then applies ANSI colors.

The full pipeline is:
```
pandoc (--reference-links) -> cleanMozAttributes -> cleanPandocArtifacts -> normalizeListIndent -> normalizeWhitespace -> convertToFootnotes -> styleFootnotes -> highlightMarkdown
```

## Footnote link rendering

Body text shows colored link text followed by a dimmed footnote marker:

```
If you don't recognize this account, remove[^1] it.

Check activity[^2]

You can also see security activity at
https://myaccount.google.com/notifications
```

A dimmed separator and numbered reference section follow the body:

```
----------------------------------------
[^1]: https://accounts.google.com/AccountDisavow?adt=...
[^2]: https://accounts.google.com/AccountChooser?Email=...
```

Colors used:
- Link text: `C_LINK_TEXT`
- Footnote markers `[^N]`: `FG_DIM` (converted from hex to ANSI)
- Separator: `FG_DIM`
- Reference labels `[^N]:`: `FG_DIM`
- Reference URLs: `C_LINK_URL`

## Link picker

The `beautiful-aerc pick-link` subcommand provides keyboard-driven URL selection. It reads text from stdin, extracts all URLs, and presents a numbered list.

Interaction:
- Keys 1-9 instantly select that link
- Key 0 selects the 10th link
- j/k or arrow keys to navigate
- Enter to select the highlighted link
- q or Escape to cancel

The selected URL is printed to stdout.

Keybinding in `binds.conf`:

```ini
[view]
<Tab> = :menu -dc 'beautiful-aerc pick-link' :open-link
```

aerc's `:menu` pipes the current message through the command and uses the output as the argument to `:open-link`.

Picker colors come from palette.sh:
- Number: `C_PICKER_NUM`
- Label: `C_PICKER_LABEL`
- URL text: `C_PICKER_URL`
- Selected line: `C_PICKER_SEL_BG` + `C_PICKER_SEL_FG`

## Header formatting

The `.headers` filter runs for every message before the body is shown. It receives RFC 2822 headers from aerc and writes a styled header block.

**Header reordering**

Headers are displayed in a fixed order regardless of how they appear in the raw message:

1. From
2. To
3. Cc (omitted if empty)
4. Date
5. Subject

All other headers are suppressed. This is a deliberate design choice - aerc's raw headers are available via `:toggle-headers` if needed.

**Colorization**

Header field names (From, To, Date, Subject) are styled with `C_HDR_KEY`. Field values use `C_HDR_VALUE`. Angle brackets around email addresses use `C_HDR_DIM`.

**Address wrapping**

Long address lists (To, Cc) wrap to a continuation indent that aligns with the start of the value, not the field name. The wrapping width respects `AERC_COLUMNS`.

**Separator**

A horizontal separator line is printed after the headers, using `BG_BORDER` color, before the message body appears.

**aerc.conf note**

The `aerc.conf` in this repo sets:

```ini
show-headers=true
header-layout=X-Collapse
```

`X-Collapse` is a nonexistent header name. This tricks aerc into hiding its built-in header row, leaving only the output from the headers filter. Without this, you would see both aerc's header rendering and the filter output.

## Plain text handling

The `text/plain` filter checks the first 50 lines of the message body for HTML tags (`<div>`, `<html>`, `<body>`, `<table>`, `<span>`, `<br>`, `<p>`). If found, it treats the message as HTML and routes it through the same pipeline as `beautiful-aerc html`.

This handles a common case where some mail clients send plain text MIME parts that contain full HTML markup.

If no HTML is detected, the filter pipes the text through aerc's built-in `wrap | colorize` for standard plain text reflow and color rendering.

## How palette.sh tokens map to visual output

The Go binary loads the active TOML theme file at startup. Theme
discovery reads `styleset-name` from `aerc.conf`, then looks for
`themes/<name>.toml` relative to the config directory.

Each token in the theme file is resolved to an ANSI SGR parameter
string at load time. For example, a token `heading = { color =
"color_success", bold = true }` with `color_success = "#a3be8c"`
resolves to `38;2;163;190;140;1`. These are wrapped as
`\033[<value>m` and inserted around the relevant text. The binary
always resets with `\033[0m` after each styled span to avoid color
bleed.

The theme lookup path:

1. `$AERC_CONFIG/aerc.conf` → read `styleset-name` → `themes/<name>.toml`
2. `~/.config/aerc/aerc.conf` → same

If `aerc.conf` is not found, `styleset-name` is missing, or the
theme file does not exist, the binary exits with a clear error.

## Troubleshooting

**All output is unstyled / no colors**

The binary could not find the theme file. Verify:

1. `styleset-name=nord` is set in `aerc.conf`
2. `themes/nord.toml` exists in the same directory as `aerc.conf`

If using `$AERC_CONFIG`, verify it points to the directory containing
`aerc.conf`.

**HTML messages show raw HTML or markdown source**

pandoc is not installed or not on `$PATH`. Install it:

```sh
sudo apt install pandoc   # Debian/Ubuntu
brew install pandoc       # macOS
```

Verify: `pandoc --version`

**Headers appear twice**

aerc's built-in header rendering is active alongside the filter. Check that `aerc.conf` has:

```ini
show-headers=true
header-layout=X-Collapse
```

**Marketing emails have garbled table content**

The `unwrap-tables.lua` filter is missing or not being found by pandoc. Check that `.config/aerc/filters/unwrap-tables.lua` exists and that the `html` filter command in `aerc.conf` passes it via `-L`:

The binary passes `unwrap-tables.lua` by resolving a path relative to its own location. If the binary is installed via `go install` to `$GOBIN`, the Lua filter path may not resolve. The binary looks for the Lua filter alongside the stow-installed config files. Verify with:

```sh
beautiful-aerc html < /dev/null
```

If it errors about the Lua filter, check that stow has linked the `.config/aerc/filters/` directory correctly.

**Colors look wrong after switching themes**

Regenerate the styleset and restart aerc:

```sh
mailrender themes generate
# Then reopen aerc
```

The binary reads the theme file at startup, not on every message. Aerc must be restarted after changing themes.
