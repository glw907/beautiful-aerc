---
title: Mouse support — sidebar + cross-pane
status: accepted
date: 2026-05-11
---

## Context

ADR-0218 wired pointer events into the reader pane and declared the
`tea.View.MouseMode = MouseModeCellMotion` envelope on every frame.
`App.updateMouse` then ran a single check — overlay-absorb, else
viewer-ready forward — and treated every other state as inert.

The remaining surface is bigger: the sidebar folder rows, the
message-list rows, the tree expand/collapse on parent rows, the
scroll wheel inside each pane, and the cross-pane case where the
viewer is already open and the user clicks into the sidebar or back
into the list. Pass 34 settled the open questions and wired the
rest of the dispatch.

## Decision

**Mouse is a one-to-one shorthand for the keyboard.** Every gesture
maps to the equivalent keystroke; no new semantics emerge from the
mouse. This rules out viewport-scroll-without-cursor, double-click
state machines, drag-select, and outside-click overlay dismissal —
all of which would have made the mouse a distinct input language.

**Message list.** Single-click on a row = `Enter`: the cursor
moves to that row and the viewer opens. `messagelist.Model` grows
`ItemIndexAt(y)` that mirrors `bubbles/v2/list.Paginator` math
(`Page * PerPage + y`), an `Update` arm that calls
`m.list.Select(idx)` on `MouseClickMsg`, and a wheel arm that
calls `CursorDown`/`CursorUp` on `MouseWheelMsg`. The caller
(`account.Model.handleRightPaneMouse`) inspects
`SelectedMessage()` after dispatch and routes through
`openMessage` so the body fetch + mark-read Cmds are the same
ones `Enter` produces.

**Sidebar.** Single-click on a real folder row = `J`/`K`: the
folder cursor moves and `selectionChangedCmds` fires a folder
load. Single-click on a synthesized intermediate node (path
segment with no underlying server folder) toggles expand instead,
because selection on those rows is meaningless. Expand and
collapse on a real parent row stay keyboard-only (`→` / `←`);
clicks shouldn't both load and toggle. `sidebar.Model` grows
`RowAtLineOffset(y)` and `IsSynthesizedAt(idx)` (walking the same
group-separator-aware loop as `renderView`) and dispatches mouse
events through `handleMouseClick` / `handleMouseWheel`.

**Wheel.** Wheel up/down = the equivalent cursor key, repeated
per notch. In the sidebar this fires a folder load per notch
(same as holding `J`), which is acceptable: the cache-backed load
is sub-millisecond and matches what a keyboard hold would do.

**Cross-pane.**
- Viewer open + click on a message-list row = swap the viewer to
  that message (the `n`/`N` flow). Stays on the Pass 33 reader
  forward path.
- Viewer open + click on a sidebar row = close the viewer, then
  forward to account so the sidebar selects the folder. New
  `account.Model.CloseViewer()` is the seam — it calls
  `viewer.Close()` and cancels any in-flight body fetch.
- Click on the divider (x == sidebarWidth) is inert.

**App dispatch.** `App.updateMouse` drops the viewer-ready guard.
The cascade is now: overlay absorb → translate global Y to
account-local (subtract the 1-row top border) → branch on
`viewerOpen` × pane region. Account-local mouse events flow into
`account.Model.Update` via a new `tea.MouseClickMsg` /
`tea.MouseWheelMsg` arm; that arm partitions by X into
`handleSidebarMouse` and `handleRightPaneMouse` and rebuilds the
msg with pane-local coords before forwarding to the sub-models.

## Consequences

- The mouse is now complete for v1: every keyboard nav has a
  pointer equivalent. Triage (`d`/`a`/`s`/`r`/`m`), fold
  (`Space`/`F`), and visual mode (`v`) stay keyboard-only — they
  have no obvious pointer affordance and shouldn't be invented.
- `account.Model.CloseViewer` is the canonical programmatic
  close. Existing `q`/`Esc` paths still run through the viewer's
  own `Update`; mouse close-on-sidebar-click does not need to
  reproduce that key dispatch.
- `messagelist.Model.ItemIndexAt` is the authoritative seam for
  any future row-level hit-testing (drag-select, hover hints).
  No code outside messagelist reaches into `m.list.Paginator`.
- Tree expand-on-real-parent stays keyboard-only. If users
  report friction we can extend `handleMouseClick` to recognize
  the depth-prefix gutter and toggle expand there without an
  ADR amendment.
- `rebuildMouseMsg` (app) and `translateMouseX`/`translateMouseY`
  (account) handle only the two mouse variants the new flow uses
  (`Click` and `Wheel`). `MouseReleaseMsg` and `MouseMotionMsg`
  remain reader-only on the Pass 33 path — extend the helpers
  before adding new consumers.
- ADR-0218's "Pass 34 inherits the dispatch shape" note is now
  realized; the translation seam in `App.updateMouse` is the
  same one Pass 33 introduced, plus the close-then-forward
  branch for cross-pane sidebar clicks with viewer open.
