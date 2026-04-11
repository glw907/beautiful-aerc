# beautiful-aerc for Power Users

Deep technical reference for the filter pipeline, theme system, and architectural decisions behind beautiful-aerc. If you want to understand how things work under the hood — or if something isn't rendering the way you expect — this is the place.

For getting started, see the [README](../README.md). For the compose editor, see [nvim-mail.md](nvim-mail.md). For creating custom themes, see [themes.md](themes.md).

## Table of contents

- [aerc filter protocol](#aerc-filter-protocol)
- [HTML filter pipeline](#html-filter-pipeline)
- [Header filter](#header-filter)
- [Plain text filter](#plain-text-filter)
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

| Subcommand | aerc hook | Input |
|------------|-----------|-------|
| `mailrender headers` | `.headers` | Raw RFC 2822 headers |
| `mailrender html` | `text/html` | Raw HTML body |
| `mailrender plain` | `text/plain` | Raw plain text body |

For the `.headers` filter, aerc sends the full RFC 2822 headers (key: value lines, with continuation lines for folded headers). The blank line separating headers from body is included in stdin.

For `text/html`, aerc sends the raw HTML body. For `text/plain`, it sends the raw plain text body.

## HTML filter pipeline

When aerc opens an HTML message, it pipes the raw HTML body to `mailrender html`. The pipeline runs in sequential stages, each transforming the content:

### 1. prepareHTML

Before conversion, the binary strips known junk:

- **Mozilla attributes** — `class="moz-..."` and `data-moz-do-not-send` attributes that would pollute the markdown output
- **Hidden elements** — `display:none` divs are removed using a nesting-aware depth-tracking loop (see [Known edge cases](#known-edge-cases)). Responsive emails often embed a hidden duplicate of the entire body.
- **Zero-size images** — `<img>` tags with zero width or height are stripped. Some senders embed these *inside* hyperlink text.

### 2. convertHTML

The cleaned HTML is converted to markdown using the [`github.com/JohannesKaufmann/html-to-markdown`](https://github.com/JohannesKaufmann/html-to-markdown) library with four plugins:

- **commonmark plugin** — standard CommonMark markdown output
- **table plugin** — data tables (those with `<th>` headers) become GFM pipe tables
- **layoutTablePlugin** — detects layout tables (no `<th>`) and flattens each `<td>` cell into a separate paragraph. Marketing emails use `<table>` for layout, not data — without this, you'd see garbled grid tables everywhere.
- **imageStripPlugin** — `<img>` tags emit only their alt text (if any), or nothing. Images with explicit width/height at or below 24px are suppressed entirely (decorative icons).

### 3. normalizeWhitespace

- Non-breaking spaces (NBSP `\u00a0`) and typographic spaces (`\u2000`–`\u200a`) are replaced with regular spaces
- Zero-width characters (`\u200b`–`\u200d`, `\u2060`–`\u2064`, `\ufeff`, `\u00ad`, `\u034f`, `\u180e`) are removed entirely
- Lines containing only spaces are stripped to blank
- Three or more consecutive blank lines are collapsed to two
- Leading blank lines are removed

### 4. deduplicateBlocks

Consecutive paragraph blocks with the same visible text are collapsed to one. Comparison ignores markdown link URLs, so image-links with different tracking parameters but the same alt text are treated as duplicates.

### 5. stripEmptyLinks

Markdown links with empty text — `[](url)` — are removed entirely.

### 6. collapseShortBlocks

Runs of three or more consecutive short plain-text blocks (under 25 characters each, no markdown syntax) are joined onto a single line separated by ` · `. This handles navigation bars, step trackers, and tag lists that come from flattened table cells and were never meant to be read vertically.

### 7. unflattenQuotes

Detects Outlook-style flattened quoted replies: paragraphs containing an attribution line (`Person wrote:`) followed by inline `&gt;` markers where line breaks originally were. Reconstructs them as proper markdown blockquotes with `> ` prefixes.

### 8. compactLineRuns

Runs of three or more consecutive short single-line blocks (under 80 visible characters, not markdown block elements or sentence-ending lines) are joined with trailing spaces (`  \n`) into compact runs. This handles scattered short lines from flattened table cells that aren't short enough for `collapseShortBlocks` but are clearly not intended as separate paragraphs.

### 9. reflowMarkdown

Plain paragraphs and blockquotes are rewrapped to 78 columns using minimum-raggedness dynamic programming. Headings, tables, lists, and code fences are left untouched.

### 10. Glamour rendering

The markdown is rendered to styled ANSI output by [Glamour](https://github.com/charmbracelet/glamour) using a style derived from the active TOML theme. Headings, bold, italic, links, and blockquotes are all styled via Glamour's style document, which is built from the theme's color slots at startup.

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

- **Tracking pixels inside URLs** — Bank of America embeds 1x1 tracking `<img>` tags inside hyperlink text. Fixed with pre-conversion stripping of zero-size `<img>` tags in `prepareHTML`.

- **Image-link fragments** — `[![alt](img) text](url)` patterns (GitHub annotations) leave broken fragments after image stripping. The `imageStripPlugin` emits only the alt text, so the surrounding link is handled normally by the commonmark plugin.

- **Empty-URL links** — `[](url)` links with no text are removed by `stripEmptyLinks`.

- **Grid tables from layout HTML** — Marketing emails use `<table>` for layout. The `layoutTablePlugin` detects tables without `<th>` headers and flattens each cell into a paragraph instead of rendering a grid table.

- **Flattened Outlook quotes** — Outlook mobile flattens quoted reply text into a single `<p>` with literal `&gt;` markers where line breaks were. `unflattenQuotes` detects the `wrote: &gt;` pattern and reconstructs proper markdown blockquotes.

- **Duplicate content blocks** — Some email templates render the same content twice (e.g. for responsive display). `deduplicateBlocks` removes consecutive blocks with identical visible text.

- **Navigation bar noise** — Flattened table cells for nav bars, step trackers, and tag lists produce a stream of tiny one-word blocks. `collapseShortBlocks` joins runs of three or more into a single ` · `-separated line.

### Problem sender patterns

Sender types that stress the pipeline — useful for regression testing:

| Sender type | What makes it hard |
|-------------|-------------------|
| Marketing emails | Layout tables, tracking images, NBSP padding, zero-width characters |
| Remind.com | Bare `rmd.me/` URLs with no `https://` scheme |
| GitHub notifications | Image-links, multi-line link text, tracking redirects → empty URLs |
| Google Calendar invites | Image buttons for RSVP, empty-URL links |
| Thunderbird senders | `class="moz-*"` attributes pollute converter output; stripped in `prepareHTML` |
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

### HTML messages show raw HTML or an error

`mailrender html` failed. Run the filter manually to see the error:

```sh
echo "<html><body><p>test</p></body></html>" | mailrender html
```

Check that the `mailrender` binary is installed (`make install`) and on `$PATH`.

### Headers appear twice

aerc's built-in header rendering is active alongside the filter. Check that `aerc.conf` has:

```ini
show-headers=true
header-layout=X-Collapse
```

### Marketing emails have garbled table content

The `layoutTablePlugin` should flatten layout tables automatically. If you're seeing grid tables, the table in question likely has `<th>` header cells — the plugin treats those as data tables and passes them to the standard table renderer. Inspect the raw HTML to confirm.

Verify the filter is running correctly:

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
