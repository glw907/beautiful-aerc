# Pass 9b — Catkin markdown commands

**Date:** 2026-05-05
**Spec:** `docs/superpowers/specs/2026-05-04-compose-design.md` § Catkin internals → Command dispatch + Smart Enter + Tab/Shift-Tab + QoL #11 (WS cleanup) + § Word + char count.
**Status:** in progress.

Pure implementation pass — all design questions are settled in the
spec. Pass 9 already shipped Catkin's core (buffer, classifier,
reflow, plain render, word nav, scroll-off). 9a layered live
markdown styling. 9b lands the editing vocabulary.

## Scope

Add to `internal/catkin`:

1. **Smart Enter** (`smartenter.go`). Detect the current line's
   prefix (`>` runs, list marker, task box). Empty body + non-empty
   prefix → strip prefix, blank line, cursor on blank line.
   Otherwise insert newline; new line starts with the same prefix;
   ordered-list markers increment (`1. ` → `2. `).
2. **Tab / Shift-Tab list indent** (`indent.go`). On a list-kind
   line: `Tab` prepends 2 spaces to the prefix; `Shift+Tab` strips
   2 leading spaces (no-op at depth 0). On non-list lines: `Tab`
   inserts 2 spaces at cursor; `Shift+Tab` no-op (so ComposeTab can
   route focus when at body `(0,0)`).
3. **Ctrl+B / Ctrl+I** (`wrap.go`). Find the word at cursor (rune
   range `[start, end)` over `isWordRune`). Wrap with `**…**` or
   `*…*`. Cursor lands at the end of the wrapped span.
4. **Ctrl+K** (`wrap.go`). Insert `[](url)` skeleton at cursor;
   cursor lands inside `[]`. No selection model in v1, so the
   non-empty-word variant from the spec is deferred — bare skeleton
   only. (The "delete-to-EOL" overload referenced in the spec is
   not in this pass; that's a readline shortcut, separate concern.)
5. **Ctrl+L / Ctrl+Q** (`toggle.go`). Toggle `- ` / `> ` prefix on
   the current line. Idempotent: re-applying strips. Quote prefix
   stacks (`>` → `> >`) on repeated invocation.
6. **Ctrl+Space** (`toggle.go`). On a task-kind line, toggle
   `[ ]` ↔ `[x]`. No-op on non-task lines.
7. **Trailing-WS cleanup** (`trim.go`). On Enter (commit boundary),
   strip trailing single space and 3+ trailing spaces from the
   line being left behind. Preserve exactly two trailing spaces
   (CommonMark hard break).
8. **WordCount / CharCount** (`count.go`). Methods on `Model` over
   the raw buffer. `WordCount` splits on `unicode.IsSpace`;
   `CharCount` is `utf8.RuneCountInString`. ComposeTab will render
   these in its footer in 9h.

All commands route through one `dispatch.go` entrypoint called
from `Model.Update` before the existing word-nav handler. The
dispatcher is itself called before `Buffer.Update` so commands
override textarea's default key handling for the keys we own.

## Out of scope

- Selection-aware bold/italic — Catkin v1 has no selection model.
- `Ctrl+A` / `Ctrl+E` / `Ctrl+U` / `Ctrl+W` / `Ctrl+Y` — readline
  motions; bubbles/textarea covers the common cases. The full
  vocabulary from the spec table can land in a follow-up if a gap
  shows up in dogfooding.
- Renumbering ordered lists on mid-list insert (post-1.0 polish).
- Auto-pair / find-replace / undo — those are Pass 9c QoL.

## Tests

Table-driven, one file per command file. Each test feeds a
`Buffer` initialised with a known source + cursor offset, runs
the command, asserts the resulting `(value, cursor)`. Cursor uses
rune-offset semantics (the buffer wrapper's `RuneOffset` /
`SetRuneOffset`).

## Pass-end ritual

- ADR for the command vocabulary (numbering mostly mirrors the
  spec's table, with the deferrals named explicitly).
- Update `invariants.md` Catkin section with the command surface.
- STATUS.md reconciliation: collapse the stale Pass 9.5 row,
  insert 9b/9c/9d/9e/9f/9g/9h/9i per the compose-design spec
  passes table, and a fresh starter prompt for 9c.
- Archive this plan to `docs/superpowers/archive/plans/`.
