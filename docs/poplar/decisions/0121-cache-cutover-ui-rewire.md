---
title: Cache cutover — UI reads/writes flow through cache.Account
status: accepted
date: 2026-05-03
---

## Context

Pass 8.4a landed the cache foundation (schema, syncer, drainer). Pass
8.4a-cutover wires the UI through it. Before this pass `internal/ui/`
held a `mail.Backend` reference and called it directly for both reads
(ListFolders, OpenFolder, QueryFolder, FetchHeaders, FetchBody) and
writes (Move, Delete, MarkRead, MarkUnread, Flag, Destroy). The cache
existed in the binary but nothing read or wrote to it from the UI.

ADR-0089's optimistic-triage design layered local
`MessageList.Apply{Delete,Insert,Flag,Seen}` mutations on top of the
backend Cmds, with a callback-style `onUndo` saved in
`triageStartedMsg` and a snapshot-and-restore pattern via
`MessageList.SnapshotSource`. That worked but duplicated state: the
optimistic flip lived in two places (the in-memory msglist plus the
backend's eventual response), and undo had to reverse it in both.

## Decision

`internal/ui/AccountTab` holds `*cache.Account` instead of
`mail.Backend`. Reads come from `cache.Account.QueryFolder` /
`ListFolders`; writes go through `cache.Account.QueueOp`.
`mail.Backend.MarkRead`, `MarkUnread`, `MarkAnswered`, and `Delete`
are removed from the interface — `Flag` is the canonical primitive
and "delete" (move-to-Trash) flows through `MoveArgs{Dest: trash}`.
The cache's optimistic flip (set transactionally inside `QueueOp`) is
the sole local state; `MessageList.Apply{Delete,Insert,Flag,Seen}`,
`MessageList.SnapshotSource`, and `triageStartedMsg.onUndo` are
removed. `MessageList.RefreshSource` replaces them: a cursor-
preserving re-load triggered by every cache write.

`pumpCacheCmd` in `internal/ui/cmds.go` mirrors `pumpUpdatesCmd`,
ranging `(*cache.Account).Events()` and re-arming after each
`cacheEventMsg` so the UI re-queries the current folder when the
drainer transitions an op.

Undo is a compensating `cache.QueueOp` saved as
`triageStartedMsg.inverse tea.Cmd`. There is no separate local
roll-back — re-firing the cache's QueueOp path is the entire undo.

## Consequences

- The msglist becomes a presentation layer over cache state, not a
  separate state store. Removes ~200 lines from `msglist.go` and
  ~150 from `account_tab.go`.
- Every triage op is two SQL transactions (the original QueueOp +
  the post-op refresh `QueryFolder`), instead of an in-memory
  mutation plus a backend round-trip. Acceptable for the optimistic-
  flip path; the refresh is fast (LIMIT 500 from cache).
- Undo of a Move is racy: if the original op has flushed to the
  backend, the inverse queues a Move from dest back to src; if not,
  both ops sit in the outbox and process in order, ending in the
  same end-state. Documented; tightening to "cancel pending op"
  semantics is a Cache III concern.
- `App.cacheAcct` is not held — the cache reference lives only on
  `AccountTab`. App reaches the backend (for the connection-state
  pump) via `AccountTab.Backend()`.
- `cache.Account` exposes `AccountName()` / `AccountEmail()`
  proxies so the UI doesn't pierce through `Backend`.
- v3 schema migration: `folders.exists_total` and
  `folders.unseen_total` columns hold backend-reported counts so
  unopened folders still surface unread badges in the sidebar.
  Local cache counts (from `message_mailboxes`) take precedence
  once any messages are synced.
