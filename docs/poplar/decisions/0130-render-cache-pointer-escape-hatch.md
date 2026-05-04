---
title: Render cache for view-stable overlays — pointer escape hatch from the Elm immutable-model contract
status: accepted
date: 2026-05-03
---

## Context

`AccountTab.View()` runs on every `tea.Msg` reaching the App,
including `spinner.TickMsg` (~10 Hz while loading), `cacheEventMsg`,
keystrokes, and `WindowSizeMsg`. Two overlays carry significant
per-frame render cost relative to how often they actually change:

- `MovePicker.Box` rebuilds `buildListRows` every frame —
  `[]string` allocation and `padOrTruncate` per row, scaling with
  `len(p.matches)`. Inputs are stable between key events (cursor,
  query, matches, contentW, visible row count).
- `HelpPopover.Box` re-renders the entire static layout from
  constant `accountGroups` / `viewerGroups` tables. Inputs are
  exactly `(context HelpContext, w int, h int)`. Between
  open and close, none of these change.

Bench (1000×, 80×24, sampleFolders) before this pass: MovePicker
View 10138 ns / 8829 B / 79 allocs per frame; HelpPopover Box cold
~168 µs / ~1005 allocs.

The Elm immutable-model contract (ADR-0023, ADR-0044) says state
lives in models and mutates only in `Update`. A naive in-value
cache field would be lost on every `With*` / `Open` / `SetSize`
return.

## Decision

Both overlays store a `*<T>Cache` pointer field initialized in
their constructor; the pointer is copied with the value, so every
generation of the overlay shares the same cache. Mutators flag a
`dirty bool` on the cache. `View()` (or `Box()`) reads the cache and
its dimension keys; on miss, it rebuilds, stores, and clears dirty.

Established in:

- `internal/ui/movepicker.go` — `movePickerCache{dirty, rows,
  contentW, visibleRows}`. Mutators that flip dirty: `recompute`
  (covers query changes via Open/typing/backspace) and the cursor
  up/down handlers.
- `internal/ui/help_popover.go` — `helpPopoverCache{dirty, box,
  tooNarrow, context, w, h}`. The cache key check inside `Box`
  covers context/w/h transitions; the constructor seeds dirty=true.
  App always reconstructs `HelpPopover` on open via
  `NewHelpPopover(styles, ctx).SetSize(w, h)`, so stale-context
  service is impossible.

Both follow the same pattern: pointer initialized once at
construction, dirty defaults true, mutators flag dirty, View/Box
checks dirty plus dimension keys. Documented inline at each cache
struct's doc comment.

## Consequences

- MovePicker View: -37% time, -39% allocations (10138→6428 ns,
  79→48 allocs/op).
- HelpPopover Box warm path: 545 ns / 0 allocs vs 168 µs / 1005
  allocs cold (~309×).
- The pattern is a deliberate escape hatch from the Elm
  immutable-model contract: `View()` mutates a pointer-reachable
  field. The value identity (MovePicker / HelpPopover) is unchanged,
  and the `dirty` flag ensures stale renders are never served.
  Acceptable because the alternatives (pointer receivers on the
  whole model, or no cache) are both worse.
- A zero-value `MovePicker{}` or `HelpPopover{}` (struct literal,
  not via the constructor) would panic on cache access. Acceptable
  because no such construction path exists in the codebase; all
  callers go through `NewMovePicker` / `NewHelpPopover`.
- `Sidebar.View()` is the next obvious cache candidate (60–80
  lipgloss renders/frame at typical folder counts). Logged as
  BACKLOG #30; the same pattern applies.
- `SidebarColumn.View()` is *not* a cache candidate today —
  the wrapper's own overhead is trivial; cost lives in
  `Sidebar.View()` (the inner consumer).
