---
title: Fold state is per-session, reset on folder reload
status: accepted
date: 2026-04-13  # Pass 2.5b-3.6
---

## Context

Per-thread fold state has to live somewhere. The options were:
persisted on disk (across runs of poplar), persisted across
folder switches in one session, or reset on every folder load.
Persistence-across-runs adds storage complexity and a stale-key
problem (folded UIDs that no longer exist). Persistence-across-
folder-switches needs a per-folder map and a folder→state
lookup. Reset-on-reload is the simplest model and matches the
behavior of every other terminal mail client we looked at.

Threads also default to expanded — folding is a deliberate user
gesture, not the default state. A user who walks into a folder
expects to see the conversation contents.

## Decision

`MessageList` holds `folded map[mail.UID]bool` keyed by thread
root UID. `SetMessages` resets the map (alongside cursor and
viewport). Default state is expanded — only roots explicitly
toggled by `Space`, `F`, or `U` appear in the map.

`F` (fold all) sets every multi-message-thread root to
`folded[UID] = true`. `U` (unfold all) replaces the map with an
empty one. `Space` toggles a single root.

`AccountTab.folderLoadedMsg` calls `m.msglist.SetMessages(...)`
for every folder load, so the reset is automatic — no special
case in the AccountTab.

## Consequences

- Folder switches always show fully expanded threads. Users who
  habitually fold long conversations re-fold after every switch.
  This is a deliberate trade for simplicity; if it becomes
  annoying we revisit with per-folder state.
- The `folded` map can grow to at most `T` entries (one per
  thread root in the current folder). `UnfoldAll` releases the
  whole map; switching folders does the same via `SetMessages`.
- No serialization layer for fold state to maintain. No
  migration path to worry about when `UID` semantics change
  across backends.
