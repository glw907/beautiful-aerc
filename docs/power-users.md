# beautiful-aerc for Power Users

Deep technical reference for the filter pipeline, theme system, and architectural decisions behind beautiful-aerc. If you want to understand how things work under the hood — or if something isn't rendering the way you expect — this is the place.

For getting started, see the [README](../README.md). For the compose editor, see [nvim-mail.md](nvim-mail.md). For creating custom themes, see [themes.md](themes.md).

## Table of contents

- [aerc filter protocol](#aerc-filter-protocol)
- [HTML filter pipeline](#html-filter-pipeline)
- [Header filter](#header-filter)
- [Plain text filter](#plain-text-filter)
- [Footnote system](#footnote-system)
- [Link picker architecture](#link-picker-architecture)
- [Theme token resolution](#theme-token-resolution)
- [Known edge cases](#known-edge-cases)
- [Troubleshooting](#troubleshooting)

## aerc filter protocol

aerc invokes filter commands as shell commands. The protocol is simple:

- Email content arrives on **stdin**
- Styled ANSI text goes to **stdout**
- The `AERC_COLUMNS` environment variable carries the terminal width as a string
- A non-zero exit code causes aerc to show an error

mailrender reads `AERC_COLUMNS` in each subcommand and falls back to 80 if not set or not parseable.

The three filter hooks in `aerc.conf`:

| Subcommand | aerc hook | Input | Calls pandoc? |
|------------|-----------|-------|---------------|
| `mailrender headers` | `.headers` | Raw RFC 2822 headers | No |
| `mailrender html` | `text/html` | Raw HTML body | Yes |
| `mailrender plain` | `text/plain` | Raw plain text body | No (unless HTML detected) |

For the `.headers` filter, aerc sends the full RFC 2822 headers (key: value lines, with continuation lines for folded headers). The blank line separating headers from body is included in stdin.

For `text/html`, aerc sends the raw HTML body. For `text/plain`, it sends the raw plain text body.

## HTML filter pipeline

When aerc opens an HTML message, it pipes the raw HTML body to `mailrender html`. The pipeline runs in sequential stages, each transforming the content:

### 1. Pre-pandoc cleanup

Before passing HTML to pandoc, the binary strips known junk that produces bad markdown:

- **Mozilla attributes** — `class="moz-..."` and `data-moz-do-not-send` attributes that cause pandoc to emit escaped spans
- **Hidden elements** — `display:none` divs are removed using a nesting-aware approach (see [Known edge cases](#known-edge-cases)). Responsive emails often embed a hidden duplicate of the entire body.
- **Tracking pixels** — `<img>` tags with zero width or height are stripped. Some senders embed these *inside* hyperlink text, causing pandoc to split URLs across paragraphs.

### 2. Pandoc conversion

The binary calls pandoc as a subprocess:

```sh
pandoc -f html -t markdown --reference-links --wrap=none -L unwrap-tables.lua
```

`unwrap-tables.lua` is a pandoc Lua filter embedded in the mailrender binary (written to a temp file for pandoc to use). It flattens nested HTML layout tables into plain text instead of letting pandoc render them as markdown grid tables. Marketing emails use `<table>` for layout, not data — without this filter, you'd see garbled grid tables everywhere.

`--reference-links` produces reference-style markdown links, which the footnote converter (stage 6) transforms into numbered footnotes.

### 3. Pandoc artifact cleanup

pandoc's markdown output contains artifacts that don't render cleanly in a terminal:

- Trailing backslashes at line ends (pandoc's hard line-break marker)
- Backslash-escaped punctuation (`\.`, `\-`, `\[`, `\'`, `\|`, `\"`, `\--`)
- Superscript caret markers (`^text^`) from HTML `<sup>` elements
- Nested heading markers (`## ##` from nested `<hN>` tags)
- Empty heading lines (from empty `<hN>` tags)
- Consecutive bold markers (`****`) from adjacent `<strong>` blocks separated by `<br>`
- Stray bold markers (`**` on a line by themselves)

### 4. Bold normalization

pandoc sometimes emits `**` bold markers that open in one paragraph and close in another (from consecutive `<strong>` blocks separated by `<br>`). `normalizeBoldMarkers` scans each paragraph for unpaired `**` and strips the dangling marker so bold doesn't bleed across paragraph boundaries.

### 5. List normalization

- Unicode bullet characters (`●`, `•`, `◦`, `◆`, `▪`, `▸`, `‣`, `⁃`) are converted to standard `-` markers
- Over-indented list items (4+ spaces) are reduced to standard 2-space indent
- Loose lists (blank lines between items) are compacted

### 6. Whitespace normalization

- Non-breaking spaces (NBSP `\u00a0`) and typographic spaces (`\u2000`–`\u200a`) are replaced with regular spaces
- Zero-width characters (`\u200b`–`\u200d`, `\u2060`–`\u2064`, `\ufeff`, `\u00ad`, `\u034f`, `\u180e`) are removed entirely
- Lines containing only spaces are stripped to blank
- Three or more consecutive blank lines are collapsed to two
- Leading blank lines are removed

### 7. Footnote conversion

`convertToFootnotes` transforms pandoc's reference-style links into numbered footnote syntax:

- Numbers references sequentially (`[^1]`, `[^2]`, etc.)
- Replaces body references with colored link text and dimmed footnote markers
- Strips emphasis markers from link display text (pandoc wraps linked `<em>` text in `*...*`)
- Renders images with alt text as `[image: alt text]` labels
- Removes images without alt text
- Strips brackets from unresolved references
- Deduplicates adjacent identical footnote anchors (caused by tracking link wrappers)
- Self-referencing links (where the display text matches the URL) render as plain URLs with no footnote

### 8. Footnote styling

`styleFootnotes` applies ANSI colors to the footnote reference section:

- Separator line in dim color
- Reference labels (`[^N]:`) in dim color
- Reference URLs in link color
- OSC 8 hyperlink escape sequences wrap each URL so terminals can make them clickable even when the display text is truncated

### 9. Markdown highlighting

The final pass walks lines and applies ANSI color to markdown syntax:

- `#`, `##`, `###` heading lines get heading color
- `**text**` spans get bold style
- `*text*` spans get italic style
- Horizontal rules (`---`, `___`) get rule color

All colors come from the active TOML theme file via resolved tokens.

## Header filter

The `.headers` filter runs for every message before the body. It replaces aerc's built-in header rendering with a custom display.

**Why replace the built-in headers?** aerc's built-in header area has limited styling control — you get `header.fg` and `header.bold` for all headers equally. The filter gives us per-field coloring, address wrapping, and a consistent layout.

**What the filter does:**

1. **Parses** raw RFC 2822 headers, joining folded continuation lines and stripping `\r`
2. **Reorders** headers in a fixed display order: From, To, Cc, Date, Subject. All other headers are suppressed (aerc's raw headers are available via `H` / `:toggle-headers` if needed)
3. **Colorizes** header field names with the `hdr_key` token (blue bold in Nord), field values with `hdr_value`, and angle brackets around email addresses with `hdr_dim`
4. **Strips bare brackets** — `<email@dom>` without a preceding name becomes `email@dom` in foreground color
5. **Wraps address headers** at recipient boundaries, filling to terminal width with continuation lines indented to align under the first address
6. **Prints a separator line** in `bg_border` color below the headers

**The X-Collapse trick:** `aerc.conf` sets `header-layout=X-Collapse`, which tells aerc to display only the `X-Collapse` header in its built-in header area. No email has that header, so the area collapses to nothing. The built-in border line still renders and serves as the top separator. Only the filter output is shown.

## Plain text filter

The `text/plain` filter checks the first 50 lines of the message body for HTML tags (`<div>`, `<html>`, `<body>`, `<table>`, `<span>`, `<br>`, `<p>`). If found, it treats the message as HTML and routes it through the full HTML pipeline.

This handles a common case where some mail clients send plain text MIME parts that contain full HTML markup.

If no HTML is detected, the filter pipes the text through aerc's built-in `wrap | colorize` for standard plain text reflow and color rendering.

## Footnote system

Links are the trickiest part of rendering HTML email in a terminal. Inline URLs clutter the text and break the reading flow. The footnote system solves this by separating link text from URLs:

**In the body:** Link text is colored and followed by a dimmed footnote marker (`[^1]`). Self-referencing links (where the text is the URL) render as plain colored URLs with no footnote.

**At the bottom:** A dimmed separator line followed by numbered URL references. Each reference shows the full URL.

**OSC 8 hyperlinks:** Long URLs are visually truncated with `...` to fit within `AERC_COLUMNS`. The full URL is embedded in an [OSC 8 hyperlink escape sequence](https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda) so supporting terminals (kitty, iTerm2, WezTerm, etc.) can make the truncated text clickable. The link picker also reads OSC 8 hrefs, so truncation never affects link opening.

**Colors used:**

| Element | Token |
|---------|-------|
| Link text in body | `link_text` |
| Footnote markers `[^N]` | `msg_dim` |
| Separator line | `msg_dim` |
| Reference labels `[^N]:` | `msg_dim` |
| Reference URLs | `link_url` |

## Link picker architecture

`pick-link` is a standalone binary that provides keyboard-driven URL selection from the message viewer.

**How it's invoked:** The keybinding `<Tab> = :pipe pick-link<Enter>` in `binds.conf` tells aerc to pipe the raw message to pick-link's stdin.

**Pipeline:**

1. Reads the raw message from stdin
2. Runs the HTML filter internally (`filter.HTML`) to extract clean footnoted URLs — the same filter the viewer uses
3. Parses the footnote reference section to extract URL list
4. Opens a full-screen picker UI on `/dev/tty` (not stdin, since stdin is the piped message)
5. User selects a URL
6. Opens the URL via `xdg-open` (or hands `mailto:` links back to aerc)

**Why `/dev/tty`?** stdin is the piped message content, so the picker reads keyboard input directly from the terminal device (`/dev/tty`). This is a common pattern for interactive filters that receive data on stdin.

**Why an alternate screen buffer?** The picker uses the terminal's alternate screen buffer (the same mechanism `vim`, `less`, and other full-screen programs use) so the picker UI doesn't pollute the terminal scrollback. When you close the picker, the original screen is restored.

**Picker controls:**

- Keys `1`-`9` instantly select that link (no Enter needed)
- Key `0` selects the 10th link
- `j`/`k` or arrow keys to navigate
- `Enter` to select the highlighted link
- `q` or `Escape` to cancel

**Colors:** The picker reads the same TOML theme file as mailrender. Tokens: `picker_num` (digits), `picker_label` (link text), `picker_url` (URL text), `picker_sel_bg` and `picker_sel_fg` (selected row).

## Theme token resolution

The Go binaries load the active TOML theme file at startup. Here's how that works:

**Theme discovery:**

1. Find the aerc config directory — checks `$AERC_CONFIG` first, then `~/.config/aerc/`
2. Read `styleset-name` from `aerc.conf` in that directory
3. Load `themes/<styleset-name>.toml` from the same config directory

The lookup is case-sensitive and must match the filename exactly (without `.toml`).

**Token resolution:**

Each token in the `[tokens]` section references a color slot by name and can include style modifiers:

```toml
heading = { color = "color_success", bold = true }
```

At load time, the `color_success` slot is looked up in `[colors]` (e.g., `#a3be8c`), converted to an RGB ANSI SGR parameter (`38;2;163;190;140`), and combined with the bold modifier (`1`). The result is a complete SGR string: `38;2;163;190;140;1`.

At render time, styled text is wrapped as `\033[<sgr>m<text>\033[0m` — the SGR escape applies the style, and `\033[0m` resets after each span to prevent color bleed.

**Styleset generation:**

The Go binaries read the theme TOML directly at runtime, but aerc needs a static styleset file for its own UI colors (message list, sidebar, tabs, etc.). `mailrender themes generate` reads the TOML and writes a complete aerc styleset using the theme's semantic color slots.

## Known edge cases

These issues were encountered and solved during pipeline development on real email. Documented here for debugging and regression testing.

### Solved issues

- **Nesting-aware hidden div removal** — Responsive HTML emails (Apple receipts, etc.) embed a hidden duplicate of the body in a `display:none` div with many nested inner `<div>` tags. A simple regex closes at the first inner `</div>`. Fixed with a depth-tracking loop that counts `<div>` opens and `</div>` closes.

- **Tracking pixels inside URLs** — Bank of America embeds 1x1 tracking `<img>` tags inside hyperlink text, causing pandoc to split a single URL across paragraphs. Fixed with pre-pandoc stripping of zero-size `<img>` tags.

- **Multi-line `![](url)` images** — pandoc splits long image URLs across lines. Handled with multi-line regex matching.

- **Pandoc backslash escapes** — All punctuation escaping (`\[`, `\]`, `\*`, `\'`, `\|`, `\"`, `\--`, etc.) is removed in artifact cleanup.

- **Image-link fragments** — `[![alt](img) text](url)` patterns (GitHub annotations) leave broken fragments after image stripping. Fixed with a dedicated regex that extracts alt text and preserves the outer link URL.

- **Empty-URL links** — `[text]()` from tracking redirects that pandoc can't resolve are reduced to plain text.

- **Grid tables from layout HTML** — Marketing emails use `<table>` for layout, which pandoc renders as grid tables. Fixed with the `unwrap-tables.lua` Lua filter.

- **Hidden divs and tracking images** — Pre-clean stage removes `display:none` elements (nesting-aware) and zero-size `<img>` tags before pandoc sees them.

- **Bold bleed across paragraphs** — pandoc emits `**` markers that open in one paragraph and close in another. Fixed with per-paragraph bold marker balancing.

- **Duplicate footnote anchors** — Tracking link wrappers cause pandoc to emit the same `[^N]` anchor twice in a row. Fixed with position-aware deduplication.

### Problem sender patterns

Sender types that stress the pipeline — useful for regression testing:

| Sender type | What makes it hard |
|-------------|-------------------|
| Marketing emails | Layout tables, tracking images, NBSP padding, zero-width characters |
| Remind.com | Bare `rmd.me/` URLs with no `https://` scheme |
| GitHub notifications | Image-links, multi-line link text, tracking redirects → empty URLs |
| Google Calendar invites | Image buttons for RSVP, empty-URL links |
| Thunderbird senders | `class="moz-*"` attributes pollute pandoc output |
| Apple receipts | Complex nested tables, heavy inline styles, hidden duplicate body |
| Newsletters | HTML with no paragraph breaks in their text/plain part |
| Callcentric | Angle-bracket autolink URLs (`<https://...>`) |
| Bank of America | Tracking pixels embedded inside hyperlink text |

## Troubleshooting

### All output is unstyled / no colors

The binary couldn't find the theme file. Check:

1. `styleset-name=nord` is set in `aerc.conf`
2. `themes/nord.toml` exists in the same directory as `aerc.conf`
3. If using `$AERC_CONFIG`, verify it points to the directory containing `aerc.conf`

### HTML messages show raw HTML or markdown source

pandoc is not installed or not on `$PATH`. Install it:

```sh
sudo apt install pandoc    # Debian/Ubuntu
brew install pandoc        # macOS
```

Verify: `pandoc --version`

### Headers appear twice

aerc's built-in header rendering is active alongside the filter. Check that `aerc.conf` has:

```ini
show-headers=true
header-layout=X-Collapse
```

### Marketing emails have garbled table content

The `unwrap-tables.lua` pandoc filter is embedded in the `mailrender` binary and written to a temp file at runtime. Verify pandoc is installed and on `$PATH`:

```sh
echo "<html><body><p>test</p></body></html>" | mailrender html
```

### Colors look wrong after switching themes

Regenerate the styleset and restart aerc:

```sh
mailrender themes generate
# Then restart aerc
```

The Go binaries read the theme file at startup, not on every message. aerc must be restarted after changing themes.
