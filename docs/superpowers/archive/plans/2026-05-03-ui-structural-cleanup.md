# Pass 8.5c — UI structural cleanup (plan)

**Date:** 2026-05-03
**Spec:** `docs/superpowers/specs/2026-05-03-ui-structural-cleanup.md`
**Predecessor:** Pass 8.5b (Elm conformance audit; ADR-0128)

## Settled open questions

1. **`ModalShell` shape:** **embedded struct with a `Box(title, rows)`
   method.** Each overlay embeds `ModalShell{open bool; width,
   height int}`, gets `IsOpen()` / `SetSize(w, h)` / box rendering
   for free, and supplies its own `Open(...)` / `Close()` (the
   signatures differ, so they can't be hoisted). Matches the
   embedded-`bubbles`-component pattern; maximum extraction with
   no API loss.
2. **Help-popover render cache location:** **on `HelpPopover`
   itself.** It's the only consumer. A separate cache layer would
   be premature abstraction. Cache key is `(context HelpContext,
   w, h int)` stored alongside the rendered string; invalidate on
   `Open` (context may change) and `SetSize`.
3. **Sidebar-column lifecycle:** **owns its dims via `SetSize(w,
   h)`.** Matches every other UI component (`Sidebar`,
   `MessageList`, `Viewer`, `SidebarSearch` all already do this).
   `AccountTab.View` just calls `m.sidebarColumn.View()`; size
   threading happens in the `WindowSizeMsg` branch.

## Deliverables

### 1. `ModalShell` (new file: `internal/ui/modal_shell.go`)

```go
type ModalShell struct {
    open          bool
    width, height int
}

func (s ModalShell) IsOpen() bool                       { return s.open }
func (s ModalShell) SetSize(w, h int) ModalShell        { s.width, s.height = w, h; return s }
func (s ModalShell) WithOpen(open bool) ModalShell      { s.open = open; return s }
func (s ModalShell) Width() int                         { return s.width }
func (s ModalShell) Height() int                        { return s.height }

// Box renders the shared ┌─ title ─┐ / ├─┤ / └─┘ frame around
// contentRows. styles supplies the border style; contentW is the
// inner content width (caller has already wrapped/padded rows to
// exactly contentW cells). Returns the assembled box string.
func (s ModalShell) Box(styles Styles, title string, contentRows []string, contentW int) string
```

Tests: `modal_shell_test.go` — golden output at typical sizes
(28×8, 60×20), title truncation when title+borders exceed
contentW, empty contentRows, single-row content.

### 2. Apply `ModalShell` to four overlays

For each of `LinkPicker`, `MovePicker`, `ConfirmModal`,
`HelpPopover`:

- Replace `open bool`, `width int`, `height int` fields with
  embedded `shell ModalShell` (named field, not anonymous embed —
  keeps grep-ability and avoids method-name collisions on `Width`
  if any arise later).
- Replace `IsOpen()` / `SetSize` bodies with delegation:
  `m.shell.IsOpen()`, `m.shell = m.shell.SetSize(w, h)`.
- `Open(...)` / `Close()` per-overlay logic now sets
  `m.shell = m.shell.WithOpen(true/false)` instead of `m.open = …`.
- `View()` builds `contentRows []string` (existing per-overlay
  layout logic) and calls `centerOverlay(m.shell.Box(styles,
  title, rows, contentW), totalW, totalH)`.
- Update `NewXxx` constructors to seed the shell.

Existing per-overlay tests stay green (no behavior change).

### 3. `SidebarColumn` (new file: `internal/ui/sidebar_column.go`)

Extract from `AccountTab.View`. Currently the sidebar column is
~60 LOC of:

- Account header rows (3-row block: blank / `acct.Email` / blank,
  per `sidebarHeaderRows = 3`).
- Folder region (`m.sidebar.View()`).
- Spacer to push the search shelf to the bottom.
- Search shelf (`m.sidebarSearch.View()`, 3 rows per
  `searchShelfRows`).
- Row-by-row join with `│` divider for the right edge of the
  column.

Shape:

```go
type SidebarColumn struct {
    styles        Styles
    icons         IconSet
    sidebar       Sidebar
    sidebarSearch SidebarSearch
    accountEmail  string
    width, height int
}

func NewSidebarColumn(styles Styles, icons IconSet, ...) SidebarColumn
func (c SidebarColumn) SetSize(w, h int) SidebarColumn
func (c SidebarColumn) Sidebar() Sidebar              // accessor (Elm: parents read after delegation)
func (c SidebarColumn) SidebarSearch() SidebarSearch
func (c SidebarColumn) WithSidebar(s Sidebar) SidebarColumn
func (c SidebarColumn) WithSidebarSearch(s SidebarSearch) SidebarColumn
func (c SidebarColumn) View() string
```

`AccountTab` holds a `SidebarColumn` instead of separate `sidebar`
+ `sidebarSearch` fields. All call sites that today read/write
those fields go through accessors / `With*`. Update is unchanged
in shape (delegation to children); only the fields move.

Tests: `sidebar_column_test.go` — golden output at sidebar widths
14, 22, 30 (the three layout tiers from ADR-0109).

### 4. `MovePicker` render cache

`MovePicker.buildListRows` is currently called every frame. Cache
the rendered list rows keyed on `(query, cursor, selected,
matches len, matches identities)`. Simplest correct cache: hash
the input tuple into a `string` key, store rendered `[]string`
rows.

Better: invalidate on `Update` events that change the list (key
input, `Open`, `SetSize`). The cache is one stored `[]string` plus
a dirty flag flipped in those handlers; `View()` rebuilds when
dirty, otherwise returns the cached slice. Smaller surface, no
hashing.

Going with the **dirty-flag approach** — fewer moving parts,
matches how `bubbles/list` invalidates.

### 5. `HelpPopover` render cache

Similar dirty-flag, but the inputs are `(context HelpContext, w,
h)`. `Open(ctx)` sets dirty + new context; `SetSize(w, h)` sets
dirty if dims changed. `View()` returns cached string when clean.

## Out of scope

- No new overlays, no behavior changes, no key changes.
- No styling changes (border characters, colors).
- No changes to `centerOverlay`, `clipPane`, or `App`'s overlay
  routing.

## Verification

Per the spec: capture golden-output snapshots of all four
overlays + the sidebar column at 80×24 and 120×40 **before** the
refactor. After each step, re-render and assert byte-for-byte
equality against the snapshots.

`make check` green throughout. `make install` + tmux capture at
both 80×24 and 120×40 before commit.

## Step order

1. Add golden-output capture tests (no implementation changes
   yet) — get green baselines.
2. Implement `ModalShell` + tests.
3. Migrate `ConfirmModal` to `ModalShell` — smallest overlay,
   verifies the shape. Run goldens.
4. Migrate `LinkPicker`, `MovePicker`, `HelpPopover` to
   `ModalShell`. Run goldens after each.
5. Add `MovePicker` render cache. Run goldens (still equal) +
   add a benchmark to confirm allocations dropped.
6. Add `HelpPopover` render cache. Same verification.
7. Implement `SidebarColumn` + tests + migrate `AccountTab`. Run
   goldens.
8. `/simplify` pass on the new files.
9. Pass-end ritual: ADR for `ModalShell` and `SidebarColumn`
   shapes; update invariants to reference the new components;
   archive plan + spec; `make check`; commit; push; install.

## Risks

- Embedded `ModalShell` with named field `shell` — if a method
  name collides between the embedder and the shell (`Width()` is
  the candidate), Go's method promotion rules require explicit
  delegation. Mitigation: use a *named* `shell` field, not
  anonymous embedding; explicit `m.shell.X()` everywhere.
- `SidebarColumn` extraction shifts where `WindowSizeMsg`
  threading happens. The current `AccountTab.Update`
  `WindowSizeMsg` branch sets sidebar layout, calls
  `SetLayout`/`SetSize` on each child. After extraction, those
  calls go through `c.WithSidebar(c.Sidebar().SetLayout(…))`-
  style chains. Verify with the existing AccountTab tests +
  goldens.
