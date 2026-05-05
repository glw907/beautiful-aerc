# Pass 9c — Catkin power-user QoL

## Goal

Land eight QoL features inside `internal/catkin/`. Library-purity
preserved — no poplar imports, no host-side wiring this pass.

Spec: `docs/superpowers/specs/2026-05-04-compose-design.md` §
"Catkin QoL additions" items #1–#4, #7, #9, #10, plus the `Ctrl+\`
mode-cycle key. Items #5 (word nav), #6 (scroll-off), #8 (task
lists), #11 (trailing-whitespace) already shipped in 9 / 9b.

## Scope

| # | Feature | Module |
|---|---------|--------|
| 1 | Undo / redo (50-step ring buffer of buffer + cursor snapshots) | `undo.go` |
| 2 | Find / replace overlay (literal, case-insensitive toggle) | `find.go` |
| 3 | Markdown auto-pair (six pairs, code-context carve-out) | `autopair.go` |
| 4 | Smart URL paste → `[word](url)` | `paste.go` |
| 7 | Bracket / span match highlight (render overlay) | `match.go` |
| 9-10 | Typewriter + focus modes + `Ctrl+\` cycle | `mode.go` |

Each module gets unit tests. Total budget ≈600 LOC.

## Tasks

### 1. Undo / redo

- `undoRing` holds `[]snap` where `snap{val string, cur int}`,
  capped at 50.
- `Model` gains `undo undoRing`. Mutate via methods.
- After every `Update` cycle: if `m.buf.Value() != lastVal`, push
  a snapshot. Coalesce when the new and old trailing rune are
  both word-runes (typing inside a word) — replace the top of
  the ring instead of pushing a fresh entry. Whitespace / newline
  / punctuation forces a new entry.
- `Ctrl+Z` undo; `Ctrl+Y` redo. Both restore the snapshot's
  buffer + cursor and leave the ring index where it lands so
  further undo continues backward.
- New keys handled in a top-level dispatch step before
  `handleCommand` (since they don't operate on `(src, cur)`
  alone — they operate on the model's history).

### 2. Find / replace

- New file `find.go` with `findState` on `Model`:
  - `active bool`, `mode findMode` (`findFind`/`findReplace`),
  - `query, replacement string`,
  - `caseInsensitive bool`,
  - `matches []int` (rune offsets), `cursor int`,
  - `inputFocus int` (0=query, 1=replacement).
- `Ctrl+F` opens find mode; `Ctrl+R` opens replace mode. While
  active, the catkin Update routes keys into find handling
  before the buffer.
- Render: when active, the bottom 1–2 rows of catkin's output
  show the prompt overlay (`Find: foo  [3/12]`). Catkin's
  `Render` reserves those rows, reducing the body height.
- Match navigation: `Enter` jumps to next match, `Shift+Enter`
  prev, `Esc` cancels. Replace mode adds `y` accept-and-next,
  `n` skip-and-next, `a` apply-all. `Tab` toggles
  case-insensitivity. Body cursor follows the active match.
- Pure rune-offset search; UTF-8 safe.

### 3. Markdown auto-pair

- Six pair triggers: `*`, `_`, `` ` ``, `[`. Triple backtick at
  line start expands to a closed fence ``` ```\n\n``` ```.
- Disabled inside inline code spans and fenced code (use
  `Classify` + intra-line span scan).
- On printable insert, intercept before buffer.Update: insert the
  pair, position cursor between. Backspace immediately after the
  pair deletes both halves.
- Implemented in `autopair.go` with `tryAutoPair(src, cur, r)
  (newSrc string, newCur int, ok bool)`.

### 4. Smart URL paste

- Listen for `tea.KeyMsg` paste runs (multiple runes in one msg)
  and `tea.PasteMsg` if available.
- If pasted content matches a URL pattern (start `http(s)://` or
  `mailto:`, no whitespace) AND the cursor is on or adjacent to
  a non-empty word, wrap as `[word](url)`. Otherwise insert raw.
- Implemented in `paste.go` with a `tryURLPaste(src, cur, paste)
  (newSrc string, newCur int, ok bool)` helper.

### 7. Bracket / span match highlight

- When the cursor sits on a delimiter rune (`*`, `_`, `` ` ``,
  `[`, `]`), find its match using the same single-line span
  scanner the styler runs. Report `(matchOffset int, ok bool)`.
- Render: matched delim position rendered with an additional
  `MatchHighlight lipgloss.Style` (added to `Styles`). Plain
  output unchanged when match-highlight style is the zero value.
- Pure render-time overlay; no buffer mutation.

### 9–10. Display modes + `Ctrl+\` cycle

- `DisplayMode` enum: `ModeNormal`, `ModeTypewriter`, `ModeFocus`,
  `ModeFocusTypewriter`. Stored on `Model`.
- `Ctrl+\` cycles
  `Normal → Typewriter → Focus → Typewriter+Focus → Normal`.
- Typewriter: in `applyScrollOff`, when typewriter is on, set
  `top = cursorLine - height/2` clamped to `[0, total-height]`.
- Focus: in `Render`, paragraphs not containing the cursor are
  rendered through a new `Styles.Dim` style. Paragraphs are
  delimited by `BlockBlank` lines from `Classify`.

## Pass-end checklist

Standard ritual: `make check`, simplify, ADR per design decision
(undo coalescing, find overlay shape, auto-pair carve-out, paste
detection, match highlight semantics, mode-cycle), invariants
update (Catkin section), STATUS bump to 9d, archive plan + spec.

No `internal/ui/` work this pass — bubbletea conventions UI
checklist N/A. Catkin's own size + render contract still applies.
