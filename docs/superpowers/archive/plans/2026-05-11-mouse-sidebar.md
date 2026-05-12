# Pass 34 — Mouse support: sidebar + cross-pane

Extends ADR-0218 (Pass 33) from the reader/right-pane dispatch
into the sidebar and message list. Mouse is **additive to the
keyboard**: every gesture maps to the equivalent keystroke.

## Settled answers to the starter-prompt questions

1. **Single-click on a message-list row** = `Enter` equivalent
   (move cursor + open viewer). Mouse mirrors keyboard; the
   viewer-open swap is symmetric with `n`/`N`. Cost of an
   accidental open is low (`Esc` closes, `m`/`r` toggles unread).
2. **Sidebar tree expand-on-click** = synthesized intermediate
   nodes (path segments with no real folder) toggle expand on
   click, since selection is meaningless there. Real folder rows
   select + load. Expand/collapse on a real parent stays
   keyboard-only (`→`/`←`) — clicks shouldn't both load and toggle.
3. **Scroll-wheel** = repeated keyboard nav for the pane under the
   pointer. Wheel in the message list = `j`/`k` repeats (one row
   per notch). Wheel in the sidebar = `J`/`K` repeats. Mouse never
   gets a new viewport-scroll-without-cursor mode; that would
   diverge from "mouse = keyboard shorthand."

Cross-pane behaviors (covered by the prompt's scope):

- Viewer open + click on a message-list row = swap viewer to that
  message (the `n`/`N` flow).
- Viewer open + click on a sidebar row = close viewer + jump
  folder. Matches Thunderbird / Apple Mail.
- Viewer open + click on the divider or chrome = inert.

## Components

### `internal/ui/messagelist`

- Expose `SelectIndex(i int)` thin wrapper over `list.Select`.
- Add `(m Model) ItemIndexAt(y int) (int, bool)` — computes
  `Paginator.Page * PerPage + y`, clamps to
  `len(VisibleItems)-1`, returns `false` when the y is past the
  last item on the current page.
- `Update` grows a `tea.MouseClickMsg` case: when
  `m := ItemIndexAt(msg.Y); ok`, call `SelectIndex(m)`. Wheel
  case: `CursorDown` on `MouseWheelDown`, `CursorUp` on
  `MouseWheelUp`.
- No new Msg type — the caller (`account.Model`) inspects the
  click result via `SelectedMessage()` after dispatch.

### `internal/ui/sidebar`

- Add `(s Model) RowAtLineOffset(y int) (rowIdx int, ok bool)` —
  walks the same loop as `renderView` to map a line offset
  (relative to the folder list, i.e. zero-based within the
  `Model.View()` output) to a `visibleRows` index. Returns
  `(-1, false)` when y lands on a group-separator blank line or
  past the last row.
- Add `(s Model) IsSynthesizedAt(rowIdx int) bool` —
  convenience over `visibleRows()[rowIdx].entry.synthesized`.
- `Update` grows `tea.MouseClickMsg` and `tea.MouseWheelMsg`
  branches. Click: if synthesized, toggle expand; else set
  `s.selected = rowIdx`. Wheel: cursor down/up by 1, like
  `Down`/`Up`. All mutations invalidate the render cache.

### `internal/ui/account`

- `Update` grows a `tea.MouseMsg` branch that dispatches by
  region:
  - Translate global `(X, Y)` to account-local. `account`'s
    top-left in the App frame is `(0, 1)` (App's row 0 is the
    top border).
  - Sidebar region (`x < sidebarW`): translate y to
    folder-list-local (subtract `sidebar.HeaderRows`). Drop the
    event when y < HeaderRows (acct header) or
    y >= h - ShelfRows (search shelf). Call `sidebar.Update`
    with the synthesized local msg. On click that changes
    selection, fire `selectionChangedCmds`. Wheel always fires
    `selectionChangedCmds` (matches J/K).
  - Right pane (`x > sidebarW`): translate x by `-(sidebarW+1)`,
    forward to `msglist.Update`. After a click, call
    `openSelectedMessage` to mirror `Enter`.

### `internal/ui/app`

- `updateMouse` drops the "viewer-ready only" guard. Cascade:
  1. Overlay open → absorb.
  2. Translate to account-local (subtract `(0, 1)`).
  3. Determine sub-pane.
  4. **Viewer open + sidebar click**: close viewer, then forward
     the click to `account.Update`. (Viewer-open + right-pane
     click stays on Pass 33's reader path; viewer-open + wheel
     in the sidebar follows the same close-then-jump rule, so
     scrolling the sidebar with viewer open closes the viewer.)
  5. **Viewer open + right-pane click/wheel**: existing Pass 33
     path — translate to right-pane-local and forward to
     `account.UpdateViewer`.
  6. **Viewer closed**: forward translated msg to
     `account.Update`. Account internally routes to sidebar /
     messagelist by x-coord.

## Bubbletea convention check

Mandatory analogues (per `bubbletea-conventions.md`):

- `messagelist` is built on `bubbles/v2/list` — that library
  has no native mouse handler, so we add the click/wheel
  translation in our wrapper. Documented deviation: list's
  pagination math (`Paginator.Page * PerPage + y`) is the
  authoritative row→index mapping.
- `sidebar` has no bubbles analogue (custom tree renderer);
  mouse handling is symmetric with its existing key handling
  in `Update`.
- No new mutation in `View()`; clicks update `selected` /
  `s.list.Select` inside `Update`. No new I/O — all Cmds remain
  in `account.Model` (folder load, body fetch).
- Width math unchanged. SPUA-A icons in sidebar/messagelist
  rows do not affect hit-testing because we test on Y only and
  the rendered row is full-width.

## Risks

- Wheel-over-sidebar fires a folder load per notch. Acceptable —
  matches holding `J`. Logs N cache reads on a fast wheel but
  the cache is in-process SQLite, sub-millisecond.
- Bubbles `list.Paginator` may have a one-cell off-by-one when
  the page contains fewer than `PerPage` items. Tests pin both
  full-page and short-page cases.

## Tasks

1. messagelist: `SelectIndex` + `ItemIndexAt` + tests.
2. messagelist: `Update` handles `MouseClickMsg` and
   `MouseWheelMsg`; tests pin click→cursor and wheel→cursor.
3. sidebar: `RowAtLineOffset` + `IsSynthesizedAt` + tests for
   group-separator rows and synthesized intermediates.
4. sidebar: `Update` handles `MouseClickMsg` / `MouseWheelMsg`
   (synthesized = toggle expand, real = select); tests.
5. account: route `tea.MouseMsg` to sidebar vs right pane;
   trigger `selectionChangedCmds` on sidebar click,
   `openSelectedMessage` on right-pane click; tests.
6. App.updateMouse: drop viewer-ready guard; new cascade
   covering viewer-open + sidebar click (close + jump); tests.
7. Update `.claude/rules/ui-invariants.md` mouse paragraphs and
   add an ADR (`0219`) for sidebar + cross-pane mouse.
8. tmux live verification at 120×40 and 80×24; capture before /
   after for the consolidation entry.
