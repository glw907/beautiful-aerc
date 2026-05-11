---
title: Sidebar folder hierarchy renders as a tree
status: accepted
date: 2026-05-10
---

## Context

Pre-0198, `.claude/rules/ui-invariants.md` declared "Nested folder
names (containing `/`) render flat. The `/` in the display name is
the only affordance." That was a placeholder — every major mail
client (Thunderbird, K-9, Geary, Apple Mail) renders nested folders
as a tree, and the flat rendering caps usefulness at one or two
nested folders before the sidebar becomes unreadable.

17a lifts the placeholder. Custom folders with `/`-paths render as
a tree with `├─` / `└─` prefixes; parents expand/collapse; the
Primary group (canonical leaves only) and Disposal group
(canonical + synthetic Outbox) are unaffected.

## Decision

The sidebar's Custom group is a tree built per-render from a
transient `*node` map and discarded; expand state lives on
`sidebar.Model` as `map[string]bool` keyed by full provider path.
`→` expands a parent row; `←` collapses (or, on a child, jumps to
the ancestor and collapses it). Default state is collapsed.
Collapsed parents show the sum-of-descendants unread count
synthesized in the walk; `aggUnread` is descendants-only, so
`renderRow` displays `entry.cf.Folder.Unseen + r.aggUnread`.

`Digital-Shane/treeview/v2` was vetted (active, v2-compatible)
and rejected — its `NodeProvider` interface would fight the
existing `renderRow` selection-indicator + icon + unread-badge
pipeline. The transient-walk pattern from
`messagelist.appendThreadRows` is reused: build tree, yield
`rowMeta` triples carrying depth/isLast/ancestorIsLast, drop
tree, render flat.

Spartan tier (W=80–89, `LayoutMode.Spartan == true`) caps
`maxDepth` at 1. Depth-2+ entries fold into their depth-1
ancestor (still selectable, count still aggregates), keeping the
14-cell sidebar legible.

Keys: `→` / `←` were unbound; reusing `Space` was rejected
because the sidebar has no focus state and `Space` already
disambiguates across visual mode / thread fold / viewer page /
attach toggle via *focus*, so a 5th meaning disambiguated by
*cursor row contents* breaches ADR-0052's cognitive-load bar.

`sidebar.Model` gains a stock `KeyMap` + `Update` (Down/Up/Top/
Bottom/Expand/Collapse). The imperative `MoveUp` / `MoveDown` /
`MoveToTop` / `MoveToBottom` methods are gone. `account.Model`
rebinds the sidebar KeyMap to its J/K/→/← keys at construction
and forwards `tea.KeyPressMsg`s directly through `sb.Update(msg)`
— no synthetic key construction at the dispatch site. This
closes #45 item (4).

## Consequences

- The `Sidebar` section of `ui-invariants.md` loses "Nested folder
  names render flat" and gains the tree invariant.
- `Styles` gains `SidebarTreeRule` (FgDim); `styling.md` updated.
- A backlog issue (#46) remains for applying the same
  transient-walk shape (yielding `(Node, Depth, IsLast)` via
  `iter.Seq2`) to `messagelist.appendThreadRows` in Pass 17b.
- Tree expand state is per-session; no persistence to config or
  cache. A future ADR could add `[ui.sidebar.expanded]` if users
  push back.
- Keyboard discoverability: `?` help vocabulary grows two rows
  (`→ expand`, `← collapse`) wired immediately.
