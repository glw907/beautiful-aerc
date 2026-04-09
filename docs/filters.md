# Filters

mailrender replaces aerc's default filter pipeline with a single Go binary that handles header rendering, HTML messages, and plain text. Each subcommand reads from stdin and writes ANSI-styled text to stdout, following aerc's filter protocol.

## The three subcommands

| Subcommand | aerc hook | Calls pandoc? |
|------------|-----------|---------------|
| `mailrender headers` | `.headers` | No |
| `mailrender html` | `text/html` | Yes |
| `mailrender plain` | `text/plain` | No (unless HTML detected) |

These are configured in `aerc.conf`:

```ini
[filters]
.headers=mailrender headers
text/html=mailrender html
text/plain=mailrender plain
```

## HTML pipeline

When aerc opens an HTML message, it pipes the raw HTML body to `mailrender html`. The pipeline runs in stages:

**1. Artifact cleanup (pre-pandoc)**

Before passing HTML to pandoc, the binary strips known junk that produces bad markdown output:

- Moz-specific HTML attributes (`class="moz-..."`, `data-moz-do-not-send`)
- These attributes cause pandoc to emit escaped spans in the markdown

**2. pandoc conversion**

The binary calls pandoc as a subprocess:

```sh
pandoc -f html -t markdown --wrap=none -L unwrap-tables.lua
```

`unwrap-tables.lua` is a pandoc Lua filter (embedded in the binary) that flattens nested HTML tables into plain text instead of letting pandoc render them as markdown tables. Marketing emails often use layout tables, not data tables - this produces much cleaner output.

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

- Lines starting with `#`, `##`, `###` get heading color
- `**text**` spans get bold style
- `_text_` spans get italic style
- Horizontal rules (`---`, `***`, `___`) get rule color

Colors come from the active TOML theme file. See [docs/themes.md](themes.md) for the token reference.

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
- Link text: `link_text` token
- Footnote markers `[^N]`: `msg_dim` token
- Separator: `msg_dim` token
- Reference labels `[^N]:`: `msg_dim` token
- Reference URLs: `link_url` token

## Link picker

`pick-link` is a standalone binary that provides keyboard-driven URL selection. It is invoked via `:pipe` so aerc feeds the raw message on stdin. The binary runs the HTML filter internally to extract footnoted URLs, then opens a full-screen picker UI on `/dev/tty` (alternate screen buffer, raw terminal mode).

Interaction:
- Keys 1-9 instantly select that link
- Key 0 selects the 10th link
- j/k or arrow keys to navigate
- Enter to select the highlighted link
- q or Escape to cancel

The selected URL is opened via `xdg-open`.

Keybinding in `binds.conf`:

```ini
[view]
<Tab> = :pipe pick-link<Enter>
```

Picker colors come from the active theme file:
- Number: `picker_num` token
- Label: `picker_label` token
- URL text: `picker_url` token
- Selected line: `picker_sel_bg` + `picker_sel_fg` tokens

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

Header field names (From, To, Date, Subject) are styled with the `hdr_key` token. Field values use the `hdr_value` token. Angle brackets around email addresses use the `hdr_dim` token.

**Address wrapping**

Long address lists (To, Cc) wrap to a continuation indent that aligns with the start of the value, not the field name. The wrapping width respects `AERC_COLUMNS`.

**Separator**

A horizontal separator line is printed after the headers, using `bg_border` color, before the message body appears.

**aerc.conf note**

The `aerc.conf` in this repo sets:

```ini
show-headers=true
header-layout=X-Collapse
```

`X-Collapse` is a nonexistent header name. This tricks aerc into hiding its built-in header row, leaving only the output from the headers filter. Without this, you would see both aerc's header rendering and the filter output.

## Plain text handling

The `text/plain` filter checks the first 50 lines of the message body for HTML tags (`<div>`, `<html>`, `<body>`, `<table>`, `<span>`, `<br>`, `<p>`). If found, it treats the message as HTML and routes it through the same pipeline as `mailrender html`.

This handles a common case where some mail clients send plain text MIME parts that contain full HTML markup.

If no HTML is detected, the filter pipes the text through aerc's built-in `wrap | colorize` for standard plain text reflow and color rendering.

## How theme tokens map to visual output

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

1. `$AERC_CONFIG/aerc.conf` -> read `styleset-name` -> `themes/<name>.toml`
2. `~/.config/aerc/aerc.conf` -> same

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

The `unwrap-tables.lua` pandoc filter is embedded in the `mailrender` binary. If you see garbled tables, verify that pandoc is installed and on `$PATH`:

```sh
mailrender html < /dev/null
```

**Colors look wrong after switching themes**

Regenerate the styleset and restart aerc:

```sh
mailrender themes generate
# Then reopen aerc
```

The binary reads the theme file at startup, not on every message. Aerc must be restarted after changing themes.
