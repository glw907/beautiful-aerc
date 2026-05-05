---
title: Catkin power-user QoL — undo, find/replace, auto-pair, smart paste, bracket match, display modes
status: accepted
date: 2026-05-05
---

## Context

Catkin shipped its core editor (ADR-0144), live markdown styling
(ADR-0145), and command vocabulary (ADR-0146) in passes 9 / 9a / 9b.
The compose design spec
(`docs/superpowers/specs/2026-05-04-compose-design.md` § "Catkin
QoL additions") lays out eleven additions surfaced from a survey
of micro / nano / helix / iA Writer / Typora / Obsidian / VS Code
markdown / aerc-compose. Items #5 (word nav), #6 (scroll-off),
#8 (task lists), and #11 (trailing-whitespace) shipped in passes
9 / 9b. Pass 9c lands the remaining seven plus the `Ctrl+\` mode
cycle key that #9 and #10 share.

## Decision

Eight features land inside `internal/catkin/` with no host-side
wiring; library-purity (bubbletea + bubbles + lipgloss + reflow +
chroma + x/ansi) is preserved.

- **Undo / redo (`undo.go`).** 50-step linear ring of
  `(value, cursor)` snapshots. Successive intra-word edits
  coalesce by replacing the ring's top when both the prior and
  incoming snapshots end on a word rune. Whitespace, newline,
  or punctuation forces a fresh entry. `Ctrl+Z` / `Ctrl+Y`
  drive undo / redo. Recording any new edit truncates the
  redo tail. `SetValue` re-seeds the ring (programmatic loads
  are not user edits).
- **Find / replace (`find.go`).** Internal `findState` on `Model`;
  `Ctrl+F` enters find, `Ctrl+R` enters / promotes to replace.
  Literal substring with `Tab` toggling case-insensitivity.
  `Enter` next match, `Shift+Enter` previous, `Esc` cancels.
  In replace mode `Ctrl+Y` accepts current, `Ctrl+N` skips,
  `Ctrl+A` accepts all in a single pass. The overlay reserves
  one row (find) or two rows (replace) at the bottom of
  Catkin's render area; body height shrinks by `footerRows()`.
- **Markdown auto-pair (`autopair.go`).** Six pair triggers:
  `*`, `_`, `` ` ``, `[`. Inside an existing emphasis pair
  (`prev == r && next == r` for `r ∈ {*, _}`) the trigger
  expands the pair to bold (`**▌**` / `____`). When the next
  rune already equals the trigger, the typed key steps over.
  Backspace between matched delimiters deletes both halves.
  Pairing is suppressed inside fenced or indented code blocks
  and inside inline code spans. Bracketed-paste runs
  (`k.Paste`) bypass the handler entirely.
- **Smart URL paste (`paste.go`).** A bracketed-paste KeyMsg
  whose payload is a single whitespace-free token starting
  with `http://`, `https://`, or `mailto:` wraps the word
  containing or adjacent to the cursor as `[word](url)`.
  Empty cursor or code-context paste falls through to ordinary
  paste behaviour.
- **Bracket / span match highlight (`match.go`).** When the
  cursor sits on `*`, `_`, `` ` ``, `[`, or `]` and the rune
  belongs to an inline span, the matching delimiter is
  re-rendered through `Styles.MatchHighlight` via an
  ANSI-aware overlay (`x/ansi.Cut`). The scanner shares
  `walkSpans` with `tokenize`. Match-highlight is a pure
  render-time overlay; the buffer is not touched.
- **Display modes (`mode.go`).** `DisplayMode` enum
  `{Normal, Typewriter, Focus, FocusTypewriter}` cycled by
  `Ctrl+\`. Typewriter centers the cursor vertically in the
  viewport (`top = cursorLine - height/2`, clamped to the
  document range). Focus dims paragraphs not containing the
  cursor through `Styles.Dim`; paragraphs are runs delimited
  by `BlockBlank` lines from `Classify`.

`Render`'s signature gains a `DisplayMode` arg. `Styles` gains
`MatchHighlight` and `Dim`; the zero values are no-ops.
`tokenize` and `scanSpans` share the new internal `walkSpans`
iterator so the two never diverge on which span shapes are
recognised.

## Consequences

- Catkin is now editor-complete for v1 compose. Remaining QoL
  items (smart quotes) stay deferred to post-1.0.
- `Render` callers must pass a `DisplayMode`; only `Model.View`
  calls it externally, and the test corpus was updated to
  pass `ModeNormal`.
- `walkSpans` consolidates the inline-span regex priority list.
  Future delimiter additions live in one place.
- The undo ring's coalescing rule trades fine-grained undo for
  natural word-level granularity. Acceptable for prose email;
  a richer history (per-keystroke + idle-timer flush) is a
  post-1.0 refinement.
- Find/replace is single-line literal with case-insensitive
  toggle. Regex and multi-line are out of scope per spec.
