---
title: Catkin live markdown styling — render-time overlay, chroma for fences
status: accepted
date: 2026-05-04
---

## Context

Pass 9 shipped Catkin's renderer in plain mode. iA-Writer-shaped
live styling was deferred to 9a: bold/italic/code spans render
with their delimiters visible AND a styled face; headings,
quotes, list markers, code blocks gain styled output; the raw
buffer is unchanged. Three open questions:

1. Code-fence syntax highlighting — off / chroma / treesitter?
2. Inline span precedence — `***triple***` and emphasis crossings.
3. Link `[text](url)` display — fold or keep delimiters?

Poplar's audience is coders (Geoff, 2026-05-04). Real syntax
highlighting in fenced code is a first-class feature for that
audience, not a polish item.

## Decision

**Library-purity widening.** Catkin's dep set, previously
bubbletea + bubbles + lipgloss + muesli/reflow + charmbracelet/x/ansi,
gains `github.com/alecthomas/chroma/v2`. Pure Go, no cgo. Catkin
remains poplar-import-free and extractable as
github.com/glw907/catkin. Binary cost ~600KB.

**Catkin-owned style table.** `catkin.Styles` (12 lipgloss.Style
fields) is the boundary. The host (poplar's compose layer, when
wired) maps `theme.CompiledTheme` onto it. The zero value yields
plain output identical to Pass 9.

**Inline span precedence.** Single-pass left-to-right tokenize
with this priority: code (backticks) > bold-italic (`***...***`)
> bold (`**...**` / `__..__`) > italic (`*...*` / `_..._`) >
link (`[text](url)`). Crossings not attempted. Broken markdown
(unclosed delimiter, cursor mid-delim) renders literally. iA
Writer-shaped: when the regex doesn't match cleanly, the source
shows literally — that *is* the correct rendering for in-progress
markdown.

**Link rendering.** `[text]` styled with the link face; `(url)`
styled with the URL face. No folding. Delimiters visible.

**Code fences.** Syntax-highlighted via chroma whenever the info
string identifies a known lexer (e.g., ` ```go `). Style is
"monokai" with a "terminal256" formatter. Unknown lexer or any
chroma error falls back to applying `Styles.CodeBlock` to each
line. Lexer/style/formatter triples are cached per language tag
in a package-level `sync.Map` so registry lookups happen once
per session, not once per render.

**Cursor on styled lines.** The cursor block (`█`) is placed in
the raw line BEFORE styling. Tokenization runs on the
cursor-mutated line: when `█` lands on a delimiter rune the span
regex no longer matches and that span renders unstyled — a
graceful degrade matching the iA-Writer-shaped invariant. Code
fence content lines fall back to `Styles.CodeBlock.Render` on
the cursor row (chroma is run on raw bodies; rune replacement
breaks the lexer).

**Viewport-bounded fence highlighting.** `renderFences` only
runs chroma on fenced blocks whose interior intersects
[top, top+height). Off-screen fences cost nothing per render.

**Soft-wrap is ANSI-aware.** Render's per-line soft-wrap uses
`charmbracelet/x/ansi.Hardwrap` so styled lines wider than width
split at the cell boundary without breaking SGR sequences.

## Consequences

- A consumer wires Catkin styling by allocating a `catkin.Styles`
  populated from its theme and calling `Model.SetStyles(s)`.
  Until that wiring lands (Pass 9.5 compose), Catkin renders
  plain because no caller passes styles yet.
- Chroma styles are not theme-driven — every fence renders in
  monokai. If/when this becomes a real seam (theme-aware code
  fences), add a `ChromaStyle string` field on `Styles`.
- Soft-wrap inside a chroma-styled line still works because
  ansi.Hardwrap preserves SGR escapes; visible cell width is
  unaffected by styling.
- The tokenize/renderSpans pair structurally parallels
  `internal/content`'s span renderer. Sharing is precluded by
  the library-purity invariant; this is intentional and
  documented here rather than in code.
