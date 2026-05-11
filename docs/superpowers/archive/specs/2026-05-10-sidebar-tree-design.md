# Sidebar Tree — Design

**Pass:** 17a. First of the bubbles-adoption remainder (15a, 15a.5
done; **17a** / 17b / 17c queued) after the 16 series locks
modern-Go conventions. Lands before Polish II and the v0.9.0
freeze.

**Goal.** Replace the sidebar's flat-folder rendering with a real
tree for Custom folders whose display names contain `/`. Top-level
groups (Primary / Disposal / Custom) and existing folder mechanics
(`J/K` nav, `Tab` to search, sort, hide, rank, label) stay; what
changes is how Custom folders with hierarchy render.

## Scope

- `internal/ui/sidebar/` — `Model`, `Column`, `renderRow`, fold
  state, expand/collapse keys.
- Add #45 item (4): a `sidebar.KeyMap` + `Update` on `*Model`,
  replacing the imperative `MoveUp` / `MoveDown` / `MoveToTop` /
  `MoveToBottom` mutators. Routed through
  `account.Model.Update` like `messagelist`.

Out of scope: messagelist refactor (#46), Primary group nesting
(invariant — canonicals are leaf names), threading interactions
with tree, runtime icon toggling.

## Settled decisions

These are not open questions; recorded here so the plan can cite.

1. **Primary group stays single-level** — canonicals (Inbox /
   Drafts / Sent / Archive) have leaf names by construction; no
   tree possible. Disposal likewise (Spam / Trash / synthetic
   Outbox). The tree only appears inside the Custom group.
2. **Collapsed parents show sum-of-descendants unread** —
   matches Thunderbird / K-9 / Geary. Synthesized from the
   classified folder list at `effectiveEntries` build time, not
   stored on the wire `mail.Folder.Unseen`.
3. **Expand state persists by full provider path** —
   `map[string]bool` on `Model`, keyed by `mail.Folder.Name`
   (provider name, which is what carries `/`). Survives
   `SetFolders` calls (cache events refresh the list often).
   Prune missing paths at the end of `SetFolders`.
4. **Expand/collapse keys: `→` expand, `←` collapse.** Inert
   when cursor is on a leaf or on a non-Custom row. No conflict
   with `Space` (thread fold / visual toggle / viewer page /
   attach toggle, all disambiguated by focus today — sidebar has
   no focus state, so reusing `Space` would be a 5th meaning
   disambiguated by *cursor contents*, which trips ADR-0052's
   cognitive-load bar). Arrows are unbound today.
5. **Spartan tier (W=80–89, sidebar=14): tree on, indent capped
   at depth 1.** Depth-0 (top-level Custom) renders with no
   prefix; depth-1 renders with `├─ ` / `└─ ` (3 cells, 11-cell
   label budget); depth-2+ collapses visually into the depth-1
   parent (still selectable, still counted under it, just not
   indented further). Intermediate (W=90–107) and Full (W=108+)
   render the full depth.
6. **Library: hand-roll on the existing `renderRow`.** The
   current renderer threads selection indicator + icon + label
   + unread badge through SPUA-aware width math; wrapping
   `Digital-Shane/treeview/v2` (or `bubbles/v2/list`) would
   fight that. Hand-rolling adds an indent + box-drawing prefix
   string per row, computed once per `SetFolders` and once per
   expand/collapse toggle. **Borrow the pattern** — yield
   `(path, depth, isLast)` triples from a transient walk, then
   drop the tree, mirroring `messagelist.appendThreadRows`.

## Architecture

### Transient walk

```go
// rowMeta is yielded by walkCustom and drives renderRow.
// Tree nodes are built then discarded; the renderer never sees them.
type rowMeta struct {
    entry      folderEntry
    depth      int        // 0 = top-level Custom; ≥1 = nested
    isLast     bool       // last child at this depth among visible siblings
    ancestorIsLast []bool // for each ancestor depth, was the ancestor a last child?
                          // drives whether the indent column shows │ or space
    childCount int        // visible descendant count; 0 = leaf
}
```

`walkCustom(custom []folderEntry, expanded map[string]bool, maxDepth int)`
parses paths on `/`, builds a transient `node{name, children,
entry}` map, DFS-walks it honoring `expanded`, applies the
`maxDepth` cap (depth-2+ folded under the deepest visible
ancestor), and emits `[]rowMeta`. Aggregation of collapsed-child
unread counts happens in the same walk (one pass, sum
ascends).

### Render

`renderRow` grows a `prefix string` argument:

- depth 0: `""`
- depth ≥1: for each ancestor `ancestorIsLast[i]` choose `"│  "`
  or `"   "`; then append `"├─ "` or `"└─ "` per `isLast`. All
  three cells wide; SPUA-A safe (box-drawing is BMP, width 1).

Selection indicator (`┃`) still lives in column 0, prepended
before the tree prefix. The whole prefix occupies
`uicore.ApplyBg(s.styles.SidebarTreeRule, bgStyle)` —
`SidebarTreeRule` is a new compiled style (`FgDim`).

Icon column shifts right by `lipgloss.Width(prefix)`; label
budget shrinks by the same. Tree parents render with their
short name (last `/`-segment), not the full path.

### Expand state

`Model` grows:

```go
expanded map[string]bool  // keyed by provider name (e.g. "Lists/golang")
```

Three accessors / mutators:

```go
func (s *Model) ToggleExpanded(path string) { ... } // routed by Update
func (s Model) IsExpanded(path string) bool { ... }
func (s *Model) pruneExpanded(known map[string]struct{}) { ... }
```

`SetFolders` calls `pruneExpanded` against the new entry set.

Default expand state: **collapsed**. (User hits `→` to drill in.
Matches the file-manager / Thunderbird default.)

### Keys

New `sidebar.KeyMap` (exported per #45 item 5):

```go
type KeyMap struct {
    MoveUp   key.Binding  // J
    MoveDown key.Binding  // K
    MoveTop  key.Binding  // unbound today — keep slot for completeness
    MoveBot  key.Binding
    Expand   key.Binding  // →
    Collapse key.Binding  // ←
}
```

`sidebar.Model.Update(msg tea.Msg) (Model, tea.Cmd)` dispatches
keys via `key.Matches`. The previous imperative `MoveUp` /
`MoveDown` / `MoveToTop` / `MoveToBottom` methods on `*Model`
are removed; `account.Model` switches to forwarding the relevant
`tea.KeyPressMsg` into `Sidebar().Update`. #45 item 4 closes
with this.

`account.Model` continues to *re-map* `J`/`K` (capital) to the
sidebar's `MoveUp`/`MoveDown` bindings — the sidebar exports the
internal binding as `j`/`k` so `bubbles`-style consumers
work, but in the account view they fire on `J`/`K`. (Net: the
sidebar publishes a stock bubbles `KeyMap`, account.Model swaps
the bindings in place after construction. Pattern matches how
`messagelist` will publish its `KeyMap` in 17b.)

### Unread aggregation

In the walk, when a parent is collapsed:

```
displayedUnread(node) = node.entry.Unseen + Σ displayedUnread(child) for child in node.children
```

When expanded, the parent shows its own `Unseen` only; children
render their own rows.

Implemented as a single bottom-up pass during walk: the visit
function returns the aggregate, parent decides whether to use
own or aggregate based on `expanded[path]`.

### Spartan cap

`maxDepth` for the walk comes from `Layout`. Three tiers:

| Tier | Width | maxDepth |
|---|---|---|
| Spartan | 80–89 | 1 |
| Intermediate | 90–107 | unbounded |
| Full | 108+ | unbounded |

`maxDepth == 1` means depth-2+ entries are not emitted as
distinct rows; they're merged into their depth-1 ancestor's
aggregate. The depth-1 row still expand/collapse-toggles, but
there's no further nesting visible.

### Selected-row preservation

Existing logic in `SetFolders` preserves `prevName` (provider
name) across reloads. Extend: also preserve `expanded` state by
path (already covered by `pruneExpanded`).

When the user collapses an ancestor of the currently-selected
row, selection jumps to that ancestor. (Otherwise the cursor
hides behind a collapsed parent.)

## Style additions

`Styles` grows `SidebarTreeRule` (`FgDim`). Listed in
`docs/poplar/styling.md` under "Sidebar".

## Test surface

`internal/ui/sidebar/model_test.go` gains:

- Tree walk with mixed depths emits expected `rowMeta` order
  and prefix shapes.
- `pruneExpanded` drops vanished paths.
- Collapsed parent's unread is sum of descendants.
- Spartan tier caps depth.
- `Update` routes `→` / `←` to expand/collapse on a parent row;
  inert on a leaf.

Existing `column_test.go` and `search_test.go` should be
unaffected; spot-check.

## ADR

Single ADR (next number): "Sidebar folder hierarchy renders as
a tree; expand state persists per session by provider path;
arrows expand/collapse; Spartan tier caps at depth 1." Updates
the `Sidebar` section of `.claude/rules/ui-invariants.md` —
replaces the flat-rendering invariant. Closes the
"Nested folder names ... render flat" invariant.

#45 items (4) and partial (5 sidebar slice) close with this
pass.

## Verification

`make check` plus tmux capture at 80×24 (Spartan) and 120×40
(Full) showing a 3-deep custom folder tree expand/collapse.
