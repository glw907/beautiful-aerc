---
title: Pass 17b — messagelist on bubbles/v2/list with custom item delegate
status: accepted
date: 2026-05-10
---

## Context

Pass 17a moved the sidebar onto a `bubbles/v2/tree` component
(ADR-0198). 17b is the second of the bubbles-adoption remainder
before Polish II and the v0.9.0 freeze: `messagelist.Model` was
hand-rolling cursor, viewport, and key dispatch even though
`bubbles/v2/list` provides them. The 16-series (ADR-0196) locked
the modern-Go conventions that let the `iter.Seq2` thread walk
land in this pass instead of as a follow-up.

## Decision

`messagelist.Model` embeds `bubbles/v2/list.Model` with a custom
`*rowDelegate` (held by pointer so per-frame context refreshes
mutate in place). The list owns cursor, viewport, and navigation
key dispatch via the exported `messagelist.KeyMap` (Down/Up/Top/
Bottom). The package's imperative movement methods
(MoveDown/MoveUp/MoveToTop/MoveToBottom/HalfPage*/Page*) are
removed; `account.Model.handleKey` falls through into
`m.msglist.Update(msg)` for those bindings. `MoveCursor(delta)`
survives — the viewer's programmatic `n`/`N` path needs the
`(UID, moved)` return.

Hidden rows (folded thread children) stay in `m.rows` for tests,
`Rows()`, `ActionTargets` thread-expansion, and `threadRootIndex`
lookups. Only the visible subset reaches `list.SetItems` via
`syncList`. Fold toggles re-rebuild and `snapToUIDInList`
re-anchors the cursor on the same UID.

`appendThreadRows`'s manual prefix walker becomes
`walkThread(root) iter.Seq2[*threadNode, walkStep]`. Closes
BACKLOG #46.

## Consequences

The `*rowDelegate` pointer is an Elm-immutable-model escape
hatch. ADR-0130 codified the pattern for overlay caches
(`*helppopoverCache`, `*movepickerCache`, `*conflictCache`); this
ADR extends the carveout to per-frame render context for
`bubbles/v2/list` delegates. The constraint stays: the pointer
holds context only, not memoized output; mutations route through
`messagelist.Model` setters, never from `View()` or a `tea.Cmd`
closure.

Pass 17c can audit `bubbles/v2/help` against the new exported
`messagelist.KeyMap` and the existing `sidebar.KeyMap` /
`account.keys` to wire help-popover rows directly off the
binding sources, deduplicating the help-vocabulary table.
`account.keys.MsgList*` bindings remain in 17b for the
fall-through; 17c removes them.

The list-style configuration disables every chrome surface
(filter, status bar, pagination, help, title) — none compose
with poplar's account-pane chrome and most are forbidden by
ui-invariants (no pgup/pgdown, modifier-free, sidebar-owned
search per ADR-0188).

The §10 review checklist passed at 80×24 and 120×40.
