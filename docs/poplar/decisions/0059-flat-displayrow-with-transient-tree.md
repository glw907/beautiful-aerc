---
title: Threading via flat displayRow with transient tree (Camp 2)
status: accepted
date: 2026-04-13  # Pass 2.5b-3.6
---

## Context

Two designs were on the table for threaded display. Camp 1: an
owned tree of `*threadNode` lives on `MessageList`; the renderer
walks the tree on every frame. Camp 2: `MessageList` keeps a flat
`[]displayRow` (the wire-up of the Elm-architecture model state);
a transient tree is built only when needed for prefix
computation, then discarded.

The current message list rendering is hand-rolled and indexed by
slot — `View()` iterates a slice, `renderRow(idx)` reads
`m.rows[idx]`. Adopting an owned tree would require either
threading the tree into View (expensive per-frame walk) or
maintaining a parallel flattened slice anyway (the worst of both
worlds).

## Decision

`MessageList` holds two slices. `source []mail.MessageInfo` is
the raw backend payload; `rows []displayRow` is the rebuilt
flattened display list. `rebuild` runs the full
group→sort→flatten pipeline against `source`, builds a transient
`*threadNode` tree per bucket inside `appendThreadRows`, walks it
once to emit `displayRow`s with the right depth and box-drawing
prefix, then discards the tree. The renderer never sees the tree.

Children inside a thread always sit in the slice immediately
below their root, in chronological-ascending order. Hidden rows
remain in the slice with `hidden = true` — `View` and `moveBy`
skip them, but indexing math stays stable.

## Consequences

- `View` and `renderRow` stay simple slice iterators; nothing
  needs a tree walk on the hot path.
- Fold mutations call `rebuild`, which is cheap because tree
  construction is O(N) per bucket and tree count is small.
- Subsequent passes that need parent/child relationships at
  render time (e.g., per-message scoring, conversation summary)
  will need to either re-compute on demand or stash extra
  metadata on `displayRow` — the tree itself is unavailable.
- Forecloses Camp 1 designs (owned tree, recursive view
  rendering). If a future pass needs nested boxes or per-branch
  styling that can't be expressed via prefix strings, the
  decision will need to be revisited.
