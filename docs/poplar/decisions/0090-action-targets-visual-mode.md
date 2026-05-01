---
title: ActionTargets and visual-mode multi-select
status: accepted
date: 2026-05-01
---

## Context

Pass 6 needs to define what "the messages this triage action operates
on" means. The cursor row is the obvious default, but a visual-mode
multi-select (vim-style) was already in the plan (ADR-0015,
ADR-0024). On top of that, threads default-on (ADR-0045): a folded
thread root represents N messages even though only one row shows.
The dispatch path must honor what the user *sees*.

## Decision

`MessageList.ActionTargets()` is the single source of truth for
"which UIDs does the next triage hit?":

- If any UIDs are marked (i.e. `len(m.marked) > 0`), return them in
  source order. This is **mode-agnostic**: marks set in visual mode
  remain consumable even after `ExitVisual`. The visual-mode flag
  controls input routing (`Space` toggles a mark only when the flag
  is on), not target selection.
- Otherwise, if the cursor row is a folded thread root with size > 1,
  return root UID + every child UID in source order ("WYSIWYG" — the
  user sees one row representing N messages, the action affects
  all N).
- Otherwise, return a single-element slice of the cursor row's UID.

`MessageList` owns `visualMode bool` and `marked map[UID]struct{}`
state. `EnterVisual` sets the flag, `ExitVisual` clears both, and
`ToggleMark(uid)` flips a single UID.

Visual mode auto-exits after every `dispatchTriage` call: marks are
consumed, the mode resets to ordinary single-row navigation. No
"sticky" multi-select.

The bulk-direction policy for star/read toggle is **cursor-row-decides**:
when targets contain a mix of starred and unstarred messages, the
action's direction (`star` vs `unstar`) follows whether the cursor row
is currently starred. Same for `read` vs `unread`.

## Consequences

- Triage on a folded thread is intuitive: pressing `d` while the
  cursor sits on a collapsed thread of 5 deletes all 5. Unfolding to
  see the children is unnecessary for bulk-thread operations.
- Marks set in visual mode survive `ExitVisual` only if the user
  triages immediately. Pressing `v` again (which is a toggle in
  many editors) here re-enters visual mode but doesn't reset marks
  — `Esc` and the auto-exit-on-triage handle clearing.
- The cursor-row-decides rule for bulk star/read makes the toast
  verb match the user's intent on the row they're looking at, even
  when other selected rows are in mixed states.
- `ActionTargets` is O(n) on the source slice for both the marked
  case and the folded-thread case; for typical inboxes (≤ a few
  thousand messages) this is negligible. The folded-thread expansion
  reuses the existing `threadUIDs` walk that the prefix renderer
  already pays for.
