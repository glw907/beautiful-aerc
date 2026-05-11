# Pass 17b — messagelist on bubbles/v2/list

**Status.** Accepted, 2026-05-10.
**Closes.** BACKLOG #45 item 2 (messagelist KeyMap + Update),
BACKLOG #46 (`iter.Seq2` thread walk).
**Follows.** Pass 17a (sidebar tree on a v2 tree component, ADR-0198).
**Precedes.** Pass 17c (`bubbles/v2/help` audit + bubbles-deviation
ADRs).

## Context

Pass 17a moved the sidebar onto a `bubbles/v2/tree` component with a
custom node renderer. 17b is the second of the bubbles-adoption
remainder before Polish II and the v0.9.0 freeze: `messagelist.Model`
still hand-rolls cursor, viewport, and key dispatch even though
`bubbles/v2/list` provides those primitives. The header comment in
`internal/ui/messagelist/model.go` admits the deviation ("Hand-rolled,
not bubbles/list") without an ADR.

The 16-series locked modern-Go conventions (ADR-0196 — slices, maps,
iter, slog). 17b lands the `iter.Seq2` thread walk (#46) inline so
the migration finishes in v2-native shape, not as a follow-up pass.

## Decision

### Architecture

`messagelist.Model` keeps its public surface (data setters,
fold/sort/search routing, `ActionTargets`, `SelectedMessage`,
`Marked`, `MessageByUID`, `Count`, `IsNearBottom`, etc.) but its
viewport + cursor + key dispatch move into an embedded
`bubbles/v2/list.Model`.

```
messagelist.Model
├── data:     source, rows []displayRow, folded, marked, filter, ...   (unchanged)
├── list:     list.Model                                                (new)
├── delegate: *rowDelegate                                              (new)
└── keys:     KeyMap                                                    (new, exported)
```

Fields removed: `selected int`, `offset int`. Cursor + viewport
offset live on `list.Model` (its `cursor` field + paginator).

### Update + KeyMap

New entry point:

```go
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)
```

Imperative movement methods removed: `MoveUp`, `MoveDown`,
`MoveToTop`, `MoveToBottom`, `HalfPageDown`, `HalfPageUp`, `PageDown`,
`PageUp`. `account.Model.handleKey` switches each call site from
`m.msglist.MoveDown()` (etc.) to `m.msglist, cmd = m.msglist.Update(msg)`.

`MoveCursor(delta int) (mail.UID, bool)` survives — the viewer's
`n`/`N` navigation calls it programmatically (not via a key) and
needs the boolean "did we move" return.

Exported `messagelist.KeyMap`:

| Binding | Keys | Action |
|---|---|---|
| `Down` | `j`, `down` | cursor down (visible rows only) |
| `Up` | `k`, `up` | cursor up |
| `Top` | `g` | first visible row |
| `Bottom` | `G` | last visible row |

Only the navigation bindings live on `messagelist.KeyMap` — they
have no account-level preconditions, so `Update` dispatches them
directly. `ToggleFold`, `ToggleFoldAll`, and `EnterVisual` stay in
`account.keys` because each requires an account-level guard (visual-
mode `space` override, search-active inertness for fold keys); they
call mutator methods (`ToggleFold`, `ToggleFoldAll`, `EnterVisual`)
that survive on `messagelist.Model`.

Triage, open-message, folder-jump, search-open, and viewer-navigation
keys stay in `account.keys` — not msglist's concern.

The embedded `list.Model.KeyMap` is overridden to neutralize built-in
bindings that conflict with poplar's UX rules (no modifier chords, no
pgup/pgdown, no filter input — see ui-invariants.md):

```go
ls.KeyMap = list.KeyMap{
    CursorUp:   key.NewBinding(),  // disabled — messagelist.KeyMap.Up routes through Update
    CursorDown: key.NewBinding(),
    // all other list.KeyMap fields zero
}
```

### Item + delegate

`displayRow` becomes a `list.Item` directly:

```go
func (r displayRow) FilterValue() string { return "" }
```

`list.Model`'s filter is disabled (`SetFilteringEnabled(false)`), so
`FilterValue` is never consulted. Returning empty is correct.

`rowDelegate` holds per-frame render context:

```go
type rowDelegate struct {
    styles      Styles
    layout      uicore.LayoutMode
    icons       uicore.IconSet
    now         time.Time
    width       int
    resultsMode bool
    originByUID map[mail.UID]string
}

func (d *rowDelegate) Height() int                              { return 1 }
func (d *rowDelegate) Spacing() int                             { return 0 }
func (d *rowDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd  { return nil }
func (d *rowDelegate) Render(w io.Writer, lm list.Model, idx int, item list.Item) {
    row := item.(displayRow)
    isSelected := idx == lm.Index()
    fmt.Fprint(w, d.renderRow(row, isSelected))
}
```

`renderRow` is the existing function lifted onto `*rowDelegate` —
identical width math, SPUA-A flag-cell adjustment, sender truncation,
`[Folder]` results-mode prefix, thread prefix, subject budget, date
column, `uicore.FillRowToWidth`. Selection branch is unchanged.

The delegate is held as `*rowDelegate` on `messagelist.Model` so
context refreshes (layout change, size change, results-mode toggle,
new clock snapshot on `SetMessages`) mutate the existing delegate
instead of rebuilding the list's item slice. This is one pointer
escape from the Elm-immutable-model contract; it parallels
ADR-0130's overlay-cache carveout and is scoped here to per-frame
render context (not memoization across frames).

### Fold + visible-row materialization

Today `m.rows` carries both visible and hidden rows so fold toggles
re-flatten without a backend hit. `list.Model` can't carry hidden
items (they'd render and steal cursor keystrokes).

Resolution: `rebuild` keeps producing the full `m.rows` slice (for
`Rows()`, tests, `threadRootIndex`, `ActionTargets` thread expansion),
then materializes the visible subset into `list.SetItems`:

```go
func (m *Model) syncList() {
    visible := make([]list.Item, 0, len(m.rows))
    for _, r := range m.rows {
        if r.hidden {
            continue
        }
        visible = append(visible, r)
    }
    m.list.SetItems(visible)
}
```

Fold flips → `rebuild` → `syncList` → `snapToUIDInList(uid)` lands
the list cursor on the same UID (or its now-folded root, since
`snapToVisible` already walks back through hidden rows in `m.rows`).

`snapToUIDInList(uid mail.UID)` walks `list.Items()` for the
matching `displayRow.msg.UID` and calls `list.Select(i)`. Empty UID
or no match → `list.Select(0)`.

### bubbles/v2/list configuration

| Setting | Value | Why |
|---|---|---|
| `FilteringEnabled` | `false` | sidebar shelf owns search (ADR-0188) |
| `ShowTitle` | `false` | account chrome owns the panel header |
| `ShowFilter` | `false` | sidebar shelf owns filter UI |
| `ShowStatusBar` | `false` | App owns the status bar |
| `ShowHelp` | `false` | help popover owns vocabulary (ADR-0072) |
| `ShowPagination` | `false` | single-window scroll |
| `InfiniteScrolling` | `false` | poplar lists are bounded |
| `Styles` | minimal | row styling lives in the delegate |

### iter.Seq2 thread walk (closes #46)

`appendThreadRows`'s manual `ancestorLastFlags` buffer becomes a
pull iterator:

```go
type walkStep struct {
    depth             uint8
    ancestorLastFlags []bool
    isLast            bool
}

func walkThread(root *threadNode) iter.Seq2[*threadNode, walkStep] {
    return func(yield func(*threadNode, walkStep) bool) {
        var rec func(n *threadNode, ancestors []bool) bool
        rec = func(n *threadNode, ancestors []bool) bool {
            for i, c := range n.children {
                isLast := i == len(n.children)-1
                step := walkStep{
                    depth:             uint8(len(ancestors) + 1),
                    ancestorLastFlags: ancestors,
                    isLast:            isLast,
                }
                if !yield(c, step) {
                    return false
                }
                if !rec(c, append(ancestors, isLast)) {
                    return false
                }
            }
            return true
        }
        rec(root, nil)
    }
}
```

Caller:

```go
rows = append(rows, displayRow{msg: root.msg, isThreadRoot: true, threadSize: len(bucket)})
for node, step := range walkThread(root) {
    rows = append(rows, displayRow{
        msg:    node.msg,
        depth:  step.depth,
        prefix: buildPrefix(step.ancestorLastFlags, step.isLast),
    })
}
```

Self-contained inside `messagelist`. No external API change. The
nested closure recursion stays — go's `iter.Seq2` does not require a
flat implementation. Yield's early-stop return is honored even though
no current caller breaks.

### account-side call sites

`internal/ui/account/model.go` `handleKey` switch:

```go
case key.Matches(msg, m.keys.MsgListBottom):
    m.msglist.MoveToBottom()
case key.Matches(msg, m.keys.MsgListTop):
    m.msglist.MoveToTop()
case key.Matches(msg, m.keys.MsgListDown):
    m.msglist.MoveDown()
case key.Matches(msg, m.keys.MsgListUp):
    m.msglist.MoveUp()
```

becomes a single fall-through to `m.msglist.Update(msg)` after the
account-level switch finishes. The `account.keys.MsgList*` bindings
themselves are deleted; help-popover rows for these keys read from
`messagelist.KeyMap` via a new `Msglist().KeyMap()` accessor (or the
help popover learns to compose `KeyMap`s from multiple sources — TBD
in 17c, not this pass).

For 17b: leave the `account.keys.MsgList*` bindings in place but
make them no-ops in account's switch, letting the fall-through to
`m.msglist.Update(msg)` handle them via the matching
`messagelist.KeyMap` entries. 17c's help-popover audit will remove
the duplicates.

The viewer-context `n`/`N` path (lines 305–334) still calls
`m.msglist.MoveCursor(delta)` directly — programmatic, not via
`Update`, because the viewer needs the `(UID, moved)` return.

`ToggleFold` / `ToggleFoldAll` / `EnterVisual` stay in account's
switch because they require account-level guards (visual-mode space
override, search-active inertness, etc.) before calling into
msglist. They're not routed through `messagelist.Update`.

### Tests

`internal/ui/messagelist/model_test.go` splits into three concerns:

- **Pipeline tests** (group/sort/flatten, fold, filter, results
  mode, ActionTargets, thread expansion, sort direction). Assertions
  read `m.Rows()` and `m.ActionTargets()`. **No change.**
- **Cursor/navigation tests.** `m.MoveDown()` → `m, _ = m.Update(keyPress("j"))`.
  Same observable behavior; helper `keyPress(s string) tea.KeyPressMsg`
  added to the test file.
- **Render tests.** `m.View()` byte-equality unchanged — the delegate
  routes through the same `renderRow` logic and the same
  `FillRowToWidth` shape.

New unit tests:

- `walkThread` yields the same `(prefix, depth)` sequence the
  imperative walker produced, for a representative thread tree.
- `KeyMap` matches against `j`/`k`/`g`/`G`/`space`/`F`/`v` and
  rejects modifier chords.
- Fold toggle preserves cursor UID across `SetItems` regeneration.

## Consequences

**Unlocks.** Pass 17c can audit `bubbles/v2/help` against the new
exported `messagelist.KeyMap` and the existing `sidebar.KeyMap` /
`account.keys` to wire the help popover off the keybinding sources
directly instead of the duplicated help vocabulary table.

**Forecloses nothing.** The Item interface keeps `displayRow`
extensible; the delegate pointer carveout is bounded to per-frame
render context.

**ADR-0130 scope extension.** The named-field embedded modal-cache
pointer pattern (overlay caches) is extended to per-frame render
context for `list.Model` delegates. The constraint stays: the pointer
holds context only, not memoized render output; it is mutated through
`messagelist.Model` methods, never written from `View()` or a
`tea.Cmd` closure.

**Invariants update.** `.claude/rules/ui-invariants.md` Message-list
section: drop "Hand-rolled (not bubbles/list)" framing; add the
`bubbles/v2/list` + delegate fact; note `KeyMap` + `Update` as the
dispatch surface. `docs/poplar/invariants.md` does not need an edit —
the messagelist binding facts live in the path-scoped UI rules.

## Out of scope

- Replacing the sidebar search shelf with `list.Filter`.
- Per-folder threading toggle changes.
- Visual-mode UI changes.
- 17c (help popover bubbles audit).
- Help-popover refactor to read directly from `KeyMap` sources
  (deferred to 17c).

## Pass-end deliverables

- **ADR (next number)** — "Pass 17b: messagelist on `bubbles/v2/list`
  with custom item delegate." Covers the delegate-pointer carveout
  (extends ADR-0130 scope), the `KeyMap` + `Update` shift, and the
  `iter.Seq2` thread walk closure.
- **`.claude/rules/ui-invariants.md`** — messagelist section
  rewrite as described above.
- **STATUS.md** — Pass 17b row → done; next starter prompt = 17c.
- **§10 review checklist** — verified at 80×24 (Spartan) and
  120×40, captured via tmux.

## Scope budget

Estimated task shape (8–12 envelope):

1. Wire `bubbles/v2/list` + exported `KeyMap`.
2. `rowDelegate` type + `Render`/`Height`/`Spacing`/`Update`.
3. Materialize-visible `syncList` + `snapToUIDInList`.
4. `Update` entry + remove imperative movement methods.
5. `iter.Seq2 walkThread` + `appendThreadRows` refactor.
6. `account.Model` call-site swap (msglist nav fall-through).
7. Test renames (MoveDown → Update(keyPress)).
8. New unit tests (walkThread, KeyMap rejection, fold-cursor preservation).
9. tmux verify 80×24 + 120×40.
10. ADR + ui-invariants update.
11. STATUS update + plan/spec archive.
12. `make check`, commit, push, install.
