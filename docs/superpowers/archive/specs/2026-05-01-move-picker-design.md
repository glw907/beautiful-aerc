# Move-to-Folder Picker — Design

**Pass:** 6.5
**Date:** 2026-05-01
**Status:** approved

## Goal

Press `m` from the account view to open a modal picker, type-to-filter
the folder list, press `Enter` to move the selected message(s) to the
chosen folder. Mutation is optimistic and reuses Pass 6's toast +
undo bar. The picker mirrors the LinkPicker overlay shape (ADR-0087).

## Settled inputs

These come from the Pass 6.5 starter prompt and prior ADRs and do
not need re-litigation here:

- Optimistic mutation with toast + undo (ADR-0086, ADR-0089).
- Toast precedence + commit-on-folder-change (ADR-0089).
- `ActionTargets()` is mode-agnostic; visual marks consumed and
  visual mode auto-exits on dispatch (ADR-0090).
- Overlay pattern: `centerOverlay` + `DimANSI` + `PlaceOverlay`,
  App owns the open state, key short-circuit while open
  (ADR-0082, ADR-0087).
- `buildTriageCmd` is the canonical Cmd assembler for an
  optimistic action; reuse for `move`.

## Brainstorm decisions (this pass)

| Q | Decision |
|---|----------|
| Filter UX | Type-to-filter, implicit (no separate input box). Letters narrow; Backspace widens; matches list shrinks live. |
| Folder ordering | Sidebar order (Primary → Disposal → Custom) at open time. No recent-first in v1. |
| No-match handling | Empty list with a one-line dim hint `no folders match "xyz"`. `Enter` inert. No `ErrorMsg`. |
| Navigation keys inside picker | `↑`/`↓` only. `j`/`k` are filter input while picker is open. The modifier-free rule still holds. |

## Architecture

### Ownership

- New file `internal/ui/movepicker.go` defines `MovePicker`.
- `App` holds `movePicker MovePicker` and routes keys into it via
  the same overlay short-circuit used for help and link picker.
  Order in `App.Update`'s overlay routing: error banner is
  always foreground-rendered; help popover, link picker, and
  move picker are mutually exclusive overlays. At most one open
  at a time.
- `AccountTab` does not own the picker; it owns the `m` keybinding
  and the dispatch on `MovePickerPickedMsg`.

### Data flow

1. User presses `m` in `AccountTab.Update`.
2. Handler reads `uids := m.msglist.ActionTargets()`. If empty,
   return `nil` (silent no-op, matches triage convention).
3. Handler builds the entry list from `m.sidebar.OrderedFolders()`
   (new accessor — see below), excludes `srcFolder`, and emits:
   ```go
   OpenMovePickerMsg{
       UIDs:    uids,
       Src:     m.currentFolderName(),
       Folders: entries,
   }
   ```
4. `App.Update` receives `OpenMovePickerMsg`, calls
   `movePicker = movePicker.Open(uids, src, folders)`, and the
   subsequent render composites the dimmed underlay + box.
5. While open, `App.Update` short-circuits keys into
   `movePicker.Update(msg)`. Folder-jump keys (`I/D/S/A/X/T`),
   triage keys, and viewer-open keys are all swallowed by the
   short-circuit. `Esc` and `q` (q is swallowed) close.
6. On `Enter`, picker emits
   `MovePickerPickedMsg{UIDs, Src, Dest}` and a self-close
   message. App closes the picker and forwards the picked msg
   into `AccountTab` via the normal `Update` path.
7. `AccountTab.Update` handles `MovePickerPickedMsg` by:
   - `snapshot, positions := m.msglist.SnapshotSource(uids)`
   - `m.msglist.ApplyDelete(uids)`
   - `m.msglist.ExitVisual()`
   - returning `buildTriageCmd("move", uids, onUndo, fwd, rev)`
     where `fwd = func() error { return m.backend.Move(uids, dest) }`,
     `rev = func() error { return m.backend.Move(uids, src) }`,
     and `onUndo = func() { m.msglist.ApplyInsert(snapshot, positions) }`.

The forward toast text is `moved N to <Display>` (App formats from
the existing `triageStartedMsg.op` + a new payload field, OR by
extending `triageStartedMsg` with an optional `dest string` —
implementation-detail decision deferred to the plan).

### Sidebar accessor

`Sidebar` already has classified folders internally. Add:

```go
// OrderedFolders returns the folder entries in sidebar render order
// (Primary → Disposal → Custom). Each entry carries the canonical
// display name, the provider name (passed to backend.Move), and the
// FolderGroup tag (used by the picker for group-separator rendering
// when filter is empty). Used by the move picker to populate its
// list at open time.
func (s Sidebar) OrderedFolders() []FolderEntry
```

`FolderEntry` lives in `internal/ui/sidebar.go` next to the existing
folder types. The picker imports it.

## MovePicker

```go
type MovePicker struct {
    open    bool
    uids    []mail.UID
    src     string          // provider name; carried back in MovePickerPickedMsg
    all     []FolderEntry   // full sidebar-order list, src removed, snapshot at Open
    filter  string          // current filter, lowercased
    matches []int           // indices into all that match filter
    cursor  int             // index into matches
    offset  int
    width   int
    height  int
    styles  Styles
    theme   *theme.CompiledTheme
    keys    movePickerKeys
}

type movePickerKeys struct {
    Up        key.Binding   // ↑
    Down      key.Binding   // ↓
    Pick      key.Binding   // enter
    Close     key.Binding   // esc
    Backspace key.Binding   // backspace
}
```

### Filter behavior

- Substring match, case-insensitive, against `FolderEntry.Display`.
- `recompute()` rebuilds `matches`, resets `cursor = 0`, `offset = 0`.
- Any `tea.KeyMsg` whose `String()` is a single printable rune
  (`unicode.IsPrint` and not in the bound key set) appends to
  `filter` and calls `recompute()`.
- `Backspace` strips the last rune; if `filter == ""`, no-op.
- `q` is swallowed (consistent with help/link picker).

### Update contract

```go
func (p MovePicker) Update(msg tea.Msg) (MovePicker, tea.Cmd)
```

- Returns `nil` Cmd when not open.
- On `Enter` with `len(matches) == 0`: returns `nil`.
- On `Enter` with valid cursor: returns `tea.Batch(picked, closed)`
  where `picked` carries `MovePickerPickedMsg{p.uids, p.src,
  p.all[p.matches[p.cursor]].Provider}` and `closed` is
  `MovePickerClosedMsg{}`.
- On `Esc`: returns `MovePickerClosedMsg{}`.

State mutation lives only in `Update`. `View()` is pure.

### Layout

```
┌─ Move to (12) ───────────────────────┐
│   Inbox                              │
│ > Drafts                             │
│   Sent                               │
│                                      │
│   Archive                            │
│   Trash                              │
│   Spam                               │
│                                      │
│   Receipts/2026                      │
│   Receipts/2025                      │
│   Newsletters                        │
├──────────────────────────────────────┤
│ filter: rec                          │
│ ↑↓ select · enter pick · esc cancel  │
└──────────────────────────────────────┘
```

- `movePickerMaxWidth = 50`, capped to `w - 4`, floor 24.
- List rows = `h - 7` (title + 2 footer rows + rule + top/bottom borders).
- Group separators (blank rows) shown only when `filter == ""`.
  When filtering, the result list is dense.
- Empty-match feedback: when `filter != "" && len(matches) == 0`,
  the list area renders a single dim row
  `  no folders match "<filter>"`. Picker height is unchanged so
  the surrounding frame doesn't jump.
- Cursor row: `styles.MsgListCursor`. Filter hint + help row:
  `styles.FgDim`. No icon glyphs in the picker — `Display` text
  only.
- `View()` self-clips via `clipPane(box, w, h)`.
- Width math via `lipgloss.Width` (folder names are icon-free in
  the picker).

### Position

```go
func (p MovePicker) Position(box string, totalW, totalH int) (int, int) {
    return centerOverlay(box, totalW, totalH)
}
```

## Help popover

Add a wired row in the Triage section:

```
m  move…
```

All other help vocabulary unchanged.

## Edge cases

- **Empty `uids` at trigger.** `m` returns nil; picker never opens.
- **Source folder.** Excluded from `all` at `Open`.
- **Folder change while picker is open.** Folder-jump keys are
  swallowed by the App overlay short-circuit; no folder change can
  happen while picker is open. The picker is discarded on `Esc`
  with no dispatch and therefore no inverse to commit.
- **Backend `Move` failure.** Routes through `ErrorMsg{Op: "move",
  Err: ...}` exactly like delete/archive: App fires `onUndo` (which
  re-inserts via the snapshot) before setting `lastErr`, banner
  shows.
- **Multi-UID move (visual mode).** `ActionTargets()` already
  handles this; toast says `moved N to <folder>`; one inverse Cmd
  for the batch.
- **Non-Latin filter chars.** `tea.KeyMsg.String()` returns the
  rune verbatim; substring works on UTF-8.

## Tests

`internal/ui/movepicker_test.go`

- `TestMovePicker_OpenSetsFolderList` — `Open` populates `all`,
  excludes `src`, `matches == [0..len(all)-1]`.
- `TestMovePicker_FilterNarrows` — typing "rec" reduces matches to
  Receipts entries; Backspace widens.
- `TestMovePicker_CursorClampsOnFilter` — cursor resets to 0 on
  every filter change; never exceeds `len(matches)-1`.
- `TestMovePicker_EnterEmitsPickedMsg` — Enter at valid cursor
  emits `MovePickerPickedMsg` with the right provider name.
- `TestMovePicker_EnterInertOnEmpty` — Enter with
  `len(matches) == 0` returns nil.
- `TestMovePicker_EscClosesNoOp` — Esc emits `MovePickerClosedMsg`,
  no dispatch.
- `TestMovePicker_BoxFitsWidth` — `View()` rows ≤ width, total
  rows ≤ height.

`internal/ui/account_tab_test.go` additions

- `TestAccountTab_MKeyEmitsOpenMovePickerMsg` — `m` with non-empty
  `ActionTargets` emits `OpenMovePickerMsg` with snapshot fields
  populated.
- `TestAccountTab_MKeyNoOpOnEmpty` — `m` with no targets returns
  nil.
- `TestAccountTab_MovePickerPickedDispatchesMove` — receiving
  `MovePickerPickedMsg` flips local state via `ApplyDelete`,
  returns Cmd batching `triageStartedMsg` + forward `Move`.

`internal/ui/app_test.go` additions

- `TestApp_OpenMovePickerOpensOverlay` — receiving
  `OpenMovePickerMsg` opens picker; subsequent keys route to it;
  render dims underlay.
- `TestApp_FolderJumpInertWhilePickerOpen` — `I`/`D`/etc. are
  swallowed while picker is open.

## Bubbletea conventions

This component must conform to
`docs/poplar/bubbletea-conventions.md`. Specifically:

- `View()` self-clipped via `clipPane`; no row exceeds `width`,
  no rows beyond `height`.
- All state mutation in `Update`; `View()` pure.
- No I/O — picker has no `tea.Cmd` of its own beyond Msg-emitter
  closures.
- Width math with `lipgloss.Width` only (no icons in picker).
- Keys via `key.Binding` + `key.Matches`.
- `WindowSizeMsg` forwarded by App into `movePicker.SetSize` after
  App stores dims.
- Children signal parents via `tea.Msg` (`OpenMovePickerMsg`,
  `MovePickerPickedMsg`, `MovePickerClosedMsg`); no callbacks.

The plan doc must explicitly name the bubbles analogue (none —
custom because the implicit-filter + sidebar-order grouping has
no `bubbles/list` equivalent without significant custom work; we
follow LinkPicker as in-tree precedent rather than introducing a
list dependency for a 50-cell modal).

## ADRs to write at pass-end

- ADR-NNNN: Move picker — type-to-filter implicit input,
  sidebar-order folder list, arrow-key navigation inside the
  modal. Reuses `buildTriageCmd` for dispatch.

## Out of scope

- Recent-first ordering (future enhancement; layer on without
  breaking the picker's bones).
- Move via raw folder name typed-and-confirmed (no command mode
  in poplar; ADR-0024).
- Cross-account move (v1 single-account; cross-account is a
  v1.x concern).
- New folder creation from the picker (no v1 backend support
  for folder creation; separate decision).
