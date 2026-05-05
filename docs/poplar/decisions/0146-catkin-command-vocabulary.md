---
title: Catkin command vocabulary (Pass 9b)
status: accepted
date: 2026-05-05
---

## Context

Pass 9 shipped Catkin's core (buffer, classifier, reflow, plain
render, word nav, scroll-off). Pass 9a layered live markdown
styling. Pass 9b lands the editing vocabulary so Catkin behaves
as a markdown-aware editor and not just a styled textarea: smart
Enter that continues list/quote/task prefixes, Tab/Shift-Tab list
indent, single-key bold/italic/link/list/quote toggles, and a
Ctrl+Space task-box flip.

The compose-design spec (`docs/superpowers/specs/2026-05-04-compose-design.md`)
listed the full vocabulary; this ADR records what landed in 9b
and what was deferred.

## Decision

Catkin's `Update` runs `handleCommand` before the existing word-nav
handler. Bindings claim each keystroke before bubbles/textarea
sees it; everything else falls through unchanged.

| Key | Command | File |
|---|---|---|
| `Enter` | smart-newline (continue prefix; ordered increment; double-Enter ends block; trailing-WS cleanup preserving CommonMark hard break) | `smartenter.go` |
| `Tab` | list indent (prepend `"  "`) on list lines; insert `"  "` at cursor otherwise | `indent.go` |
| `Shift+Tab` | list outdent (strip `"  "`) on list lines; pass through otherwise (so the host can route focus) | `indent.go` |
| `Ctrl+B` | wrap word at cursor in `**…**` | `wrap.go` |
| `Ctrl+I` | wrap word at cursor in `*…*` | `wrap.go` |
| `Ctrl+K` | insert `[](url)` skeleton; cursor inside `[]` | `wrap.go` |
| `Ctrl+L` | toggle `"- "` prefix on current line | `toggle.go` |
| `Ctrl+Q` | toggle `"> "` prefix on current line | `toggle.go` |
| `Ctrl+Space` | toggle `[ ]` ↔ `[x]` on task lines; pass through otherwise | `toggle.go` |

Helpers in `commands.go` provide rune-offset ↔ `(line, col)`
conversion and a `linePrefix` reconstruction that reads
`LineContext.PostPrefix` to derive the prefix bytes. Indent and
task helpers detect list/task lines via direct shape check
(`isListLine`, `isTaskLine`) so nested lists — which the
classifier treats as indented paragraphs — still indent and
toggle correctly.

`Model.WordCount` and `Model.CharCount` are exposed for the
compose-tab footer.

## Deviations from the spec

- **`Ctrl+K` overload deferred.** The spec table lists
  delete-to-EOL as a context-dependent overload of the link
  skeleton. v1 ships the skeleton form only; readline-style
  motions (`Ctrl+A/E/U/W/Y`) stay on bubbles/textarea defaults
  and can be revisited if dogfooding surfaces a gap.
- **Selection-aware bold/italic deferred.** Catkin v1 has no
  selection model; the bare commands wrap the word at cursor.
- **Ordered-list mid-insert renumber deferred** (post-1.0
  polish, per spec out-of-scope list).

## Consequences

- Catkin is now usable as a markdown editor in its own right.
  Pass 9c lands the QoL layer (undo, find/replace, auto-pair,
  smart paste, bracket match, typewriter/focus modes).
- The dispatcher runs before word-nav, which runs before the
  buffer's default handler. Adding new commands means adding a
  case to `handleCommand`; word-nav stays a separate file because
  its concern is character-rune navigation, not block-aware
  editing.
- `linePrefix` is now used outside the renderer and depends on
  `LineContext.PostPrefix` matching the line tail byte-for-byte;
  the classifier already guarantees this.
- Dispatcher returns `(handled, Buffer, tea.Cmd)` like the word
  nav handler, mirroring the established pattern.
