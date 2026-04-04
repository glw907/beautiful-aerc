# Footnote-Style Link Rendering

## Goal

Replace inline markdown links `[text](url)` with footnote-style
references in the HTML filter. Body text stays clean and readable;
URLs are collected in a numbered reference section at the bottom.
A `pick-link` subcommand provides a keyboard-driven link picker
for opening URLs without scrolling.

## Current behavior

The HTML filter pipeline produces inline markdown links:

```
If you don't recognize this account,
[remove](https://accounts.google.com/AccountDisavow?adt=AOX8kiq4...)
it.
```

Long tracking URLs break paragraph flow. The `--clean-links` flag
hides URLs entirely but removes the ability to see or click them.

## New behavior

### Body text

Link text is colored (ACCENT_SECONDARY). A dimmed footnote marker
(FG_DIM) follows the link text. Self-referencing links (where the
display text equals the URL) render as plain URLs with no footnote.

```
If you don't recognize this account, remove[^1] it.

Check activity[^2]

You can also see security activity at
https://myaccount.google.com/notifications
```

### Reference section

A dimmed separator line followed by numbered footnote definitions.
Labels are dimmed, URLs are in link URL color and remain plain text
so kitty Ctrl+click works.

```
────────────────────────────────────────────────
[^1]: https://accounts.google.com/AccountDisavow?adt=...
[^2]: https://accounts.google.com/AccountChooser?Email=...
```

### Link picker

A new `beautiful-aerc pick-link` subcommand reads URLs from stdin,
displays a numbered list, and prints the selected URL to stdout.

The picker UI reads colors from palette.sh so it matches the active
theme. Palette tokens used:

- Number/label: ACCENT_PRIMARY
- URL text: FG_DIM
- Selected line highlight: BG_SELECTION + FG_BRIGHT
- Prompt/chrome: FG_BASE

Interaction:
- Keys 1-9 instantly select that link
- Key 0 selects the 10th link
- Any other key enters vim-style navigation (j/k to move, Enter to
  select, q/Esc to cancel)

Keybinding in `binds.conf`:

```ini
[viewer]
<Tab> = :menu -dc 'pick-link' :open-link
```

aerc's `:menu` pipes the current message text through the command
and uses the output as the argument to `:open-link`.

## Pipeline changes

Current:
```
pandoc (inline) -> cleanup -> styleLinks -> highlightMarkdown -> output
```

New:
```
pandoc (--reference-links) -> cleanup -> convertToFootnotes -> styleFootnotes -> highlightMarkdown -> output
```

### Pandoc

Add `--reference-links` to the pandoc arguments. Pandoc outputs
shortcut reference links: `[text]` in body, `[text]: url` at the
bottom. Duplicate link texts get numeric fallback labels.

### convertToFootnotes

Post-processes pandoc's reference output:

1. Split text into body and reference definitions (indented
   `[label]: url` lines at the end).
2. Number each reference sequentially starting from 1.
3. In the body, replace `[text]` with `text[^N]` and `[text][K]`
   with `text[^N]`.
4. Self-referencing links (where text = URL) become plain URLs.
   Autolinks `<url>` become plain URLs. Neither gets a footnote
   number.
5. Build the reference section: `[^N]: url` for each numbered entry.

### styleFootnotes

Applies ANSI colors:

- Body link text: link text color (from palette C_LINK_TEXT)
- Footnote markers `[^N]`: dim color (FG_DIM)
- Separator line: dim color, full terminal width in "─" characters
- Reference labels `[^N]:`: dim color
- Reference URLs: link URL color (from palette C_LINK_URL)

### highlightMarkdown

Runs after footnote styling, same as before. Headings, bold,
italic, and rules are unaffected because footnote markers don't
contain `#`, `*`, or `_` characters.

## Removals

- `--clean-links` flag: removed from `html` and `plain` subcommands.
  Footnotes replace both link display modes.
- `styleLinks` function: replaced by `convertToFootnotes` and
  `styleFootnotes`.
- `reLink` regex: no longer needed in the HTML path.

## Scope

- HTML filter only. The plain text filter is unchanged.
- Plain text emails that contain HTML are already delegated to the
  HTML filter and will get footnote rendering automatically.

## Testing

- Unit tests for `convertToFootnotes`: single link, multiple links,
  duplicate link text, self-referencing links, autolinks, no links.
- Unit tests for `styleFootnotes`: verify ANSI codes on link text,
  markers, separator, and reference entries.
- E2E golden file updates for all HTML fixtures.
- Unit tests for `pick-link` subcommand: numbered selection, edge
  cases (no URLs, more than 10 URLs).
