# Pass 9a — Catkin live markdown styling

## Goal

Live render-time styling overlay for Catkin. Markdown delimiters
remain visible (iA-Writer shape); block kinds (heading, quote,
code) and inline spans (bold, italic, code, link) gain styled
faces wired through a Catkin-owned `Styles` struct. Code fences
get real syntax highlighting via chroma when the info string
identifies a known lexer.

The raw buffer is unchanged. Cursor block (`█`) keeps current
shape. ADR-0144 invariants hold.

## Scope

In:
- `internal/catkin/style.go` (new) — `Styles` struct, default
  factory, inline span tokenizer, chroma-based fence renderer.
- `internal/catkin/render.go` (extend) — accept styles, apply
  per-line block + span styling, splice in pre-styled fence
  lines.
- `internal/catkin/catkin.go` (extend) — add `styles` field,
  `SetStyles(s Styles)` method, wire styles through to `Render`.
- `internal/catkin/render_test.go` + `style_test.go` (new) —
  cover bold/italic/code/link span styling, heading/quote/code
  block styling, chroma-highlighted fences, cursor preservation
  on styled lines, broken-syntax fallback (cursor on `*`).

Out:
- Mapping `theme.CompiledTheme` → `catkin.Styles` (no consumer
  yet; defer to compose-wiring pass).
- `blocks.go` and `reflow.go` — untouched.
- New block kinds.

## Settled (do not re-brainstorm)

- Audience is coders (per Geoff, 2026-05-04). Code fences get
  real syntax highlighting via chroma; this widens catkin's
  dep set deliberately (still pure Go, no cgo). ADR-0145
  documents the dep widening.
- Library-purity invariant: `catkin.Styles` is a Catkin-owned
  struct. Catkin does **not** import `internal/theme`. The
  consumer (poplar's compose package, when wired) maps
  `theme.CompiledTheme` → `catkin.Styles`.
- Inline span precedence: code (backticks) > bold-italic
  (`***...***`) > bold (`**...**`/`__..__`) > italic
  (`*...*`/`_..._`) > link (`[text](url)`). Single-pass tokenize
  by longest-delim-first; nested crossings not attempted.
  `**bold _italic_ end**` renders bold across the whole thing
  with literal underscores visible. iA-shaped: when the regex
  doesn't match cleanly, the source shows literally — that *is*
  the correct rendering for broken/in-progress markdown.
- Link rendering: `[text]` styled (link face), `(url)` styled
  (dim/url face), brackets and parens shown literally. No
  folding.
- Cursor handling: place `█` on the raw line first (existing
  `insertCursorBlock`), apply styling after. When the cursor
  rune lands on a delimiter (e.g., one of the `*`s in `**bold**`),
  the regex fails and that span renders unstyled. Acceptable —
  the source is mid-edit at that rune.

## Approach

### 1. `style.go` — Styles + tokenizer + fence highlighter

```go
package catkin

import "github.com/charmbracelet/lipgloss"

// Styles is Catkin's render-time style table. Zero value =
// no-op styles (plain output identical to Pass 9).
type Styles struct {
    Heading    [6]lipgloss.Style // [0]=H1, [5]=H6
    Quote      lipgloss.Style
    DeepQuote  lipgloss.Style    // depth >= 2
    Bold       lipgloss.Style
    Italic     lipgloss.Style
    BoldItalic lipgloss.Style
    CodeInline lipgloss.Style
    CodeBlock  lipgloss.Style    // applied to whole fenced block as bg
    Link       lipgloss.Style    // [text]
    URL        lipgloss.Style    // (url)
    ListMarker lipgloss.Style
    TaskBox    lipgloss.Style
}
```

Inline tokenizer: `tokenize(s string) []span` where `span =
{kind, text}`. Walk left-to-right. At each position, try delim
matchers in priority order:

1. backtick: `` `[^`]+` ``
2. triple-asterisk: `\*\*\*[^*]+\*\*\*`
3. double-asterisk / double-underscore: `\*\*[^*]+\*\*` /
   `__[^_]+__`
4. single-asterisk / single-underscore: `\*[^*]+\*` / `_[^_]+_`
5. link: `\[[^\]]+\]\([^)]+\)`

If none match at the current position, advance one rune as plain
text. Spans render their full delimited form (delimiters visible)
plus the inner styled face. For links, render `[text]` with
`Link` face and `(url)` with `URL` face.

Fence highlighter: `highlightFence(infoString, body string,
codeBlock lipgloss.Style) []string`. If chroma has no lexer for
`infoString`, fall back to applying `CodeBlock` to each line
unmodified. Otherwise, lex once, write to a custom formatter
that emits one-line-at-a-time lipgloss strings, return the slice.
The formatter uses a small palette derived from chroma token
classes (Keyword, String, Comment, Number, Function, Type) mapped
to lipgloss colors held on `Styles.CodeBlock`'s foreground for
default plus a hard-coded ANSI palette for token kinds. Keep the
palette in `style.go`; documenting in ADR-0145 that tokens use a
fixed palette (not theme-driven) for now.

### 2. `render.go` — apply styles

`Render` signature gains a `styles Styles`:

```go
func Render(src string, width, height, top, cursor int, styles Styles) string
```

Pre-pass: walk lines once, locate fenced blocks, run
`highlightFence` per block to get `map[lineIdx]string`
(post-prefix styled body for each line inside the fence).

Per-visible-line render order:

1. Place cursor `█` on raw line if cursor row matches.
2. Classify the (possibly cursor-mutated) line via `Classify`
   for THIS line's role. (Re-run classifier on the cursor-
   mutated source so insideFence stays correct around the
   cursor.) — actually no: use the classifier on the raw
   source; the cursor placement happens after classification.
   Order: classify raw → soft-wrap raw → insert cursor in the
   correct visual segment → style the post-prefix portion of
   the visual segment.
3. For BlockHeading: apply `Styles.Heading[level-1]` to the
   styled-body portion (after the `# ` prefix, which renders
   in dim ListMarker face).
4. For BlockQuote: apply `Styles.Quote` (or `DeepQuote` if depth
   >= 2) to the whole line including `>` prefix.
5. For BlockCodeFence: if line is the fence marker, render with
   `Styles.CodeBlock`. Else look up pre-rendered styled body for
   line index; if missing (no lexer / fallback path), apply
   `Styles.CodeBlock` to the raw post-prefix.
6. For BlockListItem / BlockTaskItem: marker via
   `Styles.ListMarker`, task box via `Styles.TaskBox`, body
   tokenized.
7. For BlockParagraph: tokenize body, apply per-span styles.
8. Re-attach the soft-wrap and cursor outputs.

### 3. `catkin.go` — plumb styles

Add `styles Styles` field and `SetStyles(s Styles)` method. `View`
passes `m.styles` to `Render`. Default `New()` leaves `styles` as
zero value → no-op styles → behavior identical to Pass 9 for
existing callers (none, but tests for Pass 9 still pass).

### 4. Tests

- `style_test.go`: tokenizer cases — bold, italic, bold-italic,
  code, link; longest-delim-first priority; broken/unclosed
  delims pass through as plain text.
- `render_test.go` (extend): given a `Styles{}` with non-default
  faces (Bold = `lipgloss.NewStyle().Bold(true)` etc.), the
  output for `**bold**` contains the bold ANSI escape `\x1b[1m`;
  output for `# Heading` contains the H1 face; output for fenced
  ```` ```go ```` block contains chroma keyword highlighting.
- `render_test.go` cursor cases: cursor inside a delimited span
  preserves `█`; cursor on a `*` makes the bold styling drop on
  that line (fallback path); cursor outside any span unaffected.
- `render_test.go` zero-styles regression: default `Styles{}`
  produces output identical to Pass 9's plain rendering for
  existing test inputs.

## Pass-end checklist

Standard ritual per `poplar-pass`:

1. /simplify
2. ADR-0145 (chroma dep widening + Catkin styling design)
3. Update invariants.md (Catkin section)
4. STATUS.md flip (Pass 9a done; next = 9.5)
5. Archive plan
6. `make check`
7. Commit, push, install

## Risks

- chroma binary-size impact (~500–800KB for embedded lexer
  registry). Acceptable per the audience call.
- Cursor-on-styled-text correctness; mitigated by "place cursor
  in raw text first, style after" approach.
- Inline span regex performance on very long lines; mitigated by
  width-bounded soft-wrap (we only style visible content, max
  height × width runes).
