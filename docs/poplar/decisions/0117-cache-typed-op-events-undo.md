---
title: Cache 0 — typed Op sum, name-based folder ops, drainer events, undo via Cmd
status: accepted
date: 2026-05-02
---

## Context

ADR-0111 specified `(*cache.Account).QueueOp(ctx, op)` where `op`
carried a `Kind string` plus `Args map[string]interface{}` plus a
`Folder int64` (SQLite row id) plus a `Msg sql.NullInt64`. Pass
8.4-review found four problems:

- **C4** — `map[string]interface{}` is the stringly-typed JSON
  dispatch shape `go-conventions` explicitly rejects. The on-disk
  `args TEXT` JSON encoding is fixed by v1.0 freeze regardless;
  a typed Go boundary doesn't change that and gives compile-time
  safety to call sites.
- **C5** — `Op.Folder int64` requires the UI to translate folder
  names to SQLite row ids before calling `QueueOp`. The
  `mail.Backend` boundary has always taken folder names; the
  cache boundary should match for consistency.
- **C3** — The drainer goroutine has no `tea.Program` reference
  and the spec was silent on how a confirmed op signals the UI to
  re-render. Either the cache exposes a channel and the UI pumps
  it, or the cache holds `*tea.Program` (which violates the layer
  boundary in §B.1).
- **C2** — `App.pendingAction.onUndo` today closes over
  `m.msglist` and calls `MessageList.Apply{Insert,Flag,Seen}`
  in-place. Once `MessageList` reads from SQLite on every
  `SetMessages`, those in-place mutations have no effect (the
  next read overwrites them). Undo must fire a compensating
  `QueueOp` instead.

## Decision

Four changes to the Cache 0 boundary, codified in spec §B.4 / §C:

1. **Typed `OpArgs` sum.** `OpArgs` is a sealed interface with
   one concrete type per kind: `MoveArgs`, `FlagArgs`,
   `DestroyArgs` (Cache 0 scope) plus reserved `SendArgs`,
   `AppendArgs` (Pass 9). JSON encoding/decoding happens at the
   `QueueOp` and drainer boundaries; on-disk `args TEXT` format
   is unchanged from the pre-revise spec.

2. **Name-based folder boundary.** `QueueOp(ctx, folderName,
   msgID, args OpArgs)`. The cache resolves the folder name to a
   row id inside the same transaction. Keeps the UI/cache
   boundary at strings, consistent with `mail.Backend`.

3. **Drainer→UI channel.** `(*cache.Account).Events() <-chan
   CacheEvent` exposes a buffered channel that the drainer writes
   to after each terminal transition (`done`, `conflict`). `App`
   runs a `pumpCacheCmd` mirroring `pumpUpdatesCmd` for
   `mail.Update`; the pump ranges the channel and re-emits values
   as `tea.Msg`. The cache package never imports
   `github.com/charmbracelet/bubbletea`.

4. **Undo via compensating Cmd only.** The synchronous-mutation
   half of `App.pendingAction.onUndo` is removed. Undo fires only
   the saved `inverse tea.Cmd`, which calls `cache.QueueOp` with
   the reverse op. `App.pendingAction` retains the timer and
   `inverse`. Optimistic state lives in the cache (`ui_flags`,
   `ui_hide`); the undo Cmd's `QueueOp` produces the inverse
   optimistic state in the same transaction as the inverse outbox
   row.

## Consequences

- Supersedes ADR-0111's `QueueOp` signature and the
  `Op{Kind, Args, Folder, Msg}` struct; ADR-0111's other
  decisions (unified write path, optimistic UI columns, App
  threading `*cache.Account` instead of `mail.Backend`) stand.
- `triageStartedMsg` in `internal/ui/` loses its `onUndo func()`
  field. Pass 8.4a's strangler-fig migration sequence
  (cache writes → cache reads → delete legacy paths) handles
  the field removal as part of step 3 (delete `MessageList.Apply*`
  and old backend Cmds).
- The cache→UI dependency direction is preserved: UI imports
  `cache`; `cache` does not import `ui`. The pump pattern is the
  same one already proven for `mail.Backend` updates
  (`pumpUpdatesCmd`).
- Compile-time safety on op shape catches at build time the
  category of bug ("forgot to JSON-encode `set` as a bool") that
  `map[string]interface{}` would push to runtime.
- Folder-name boundary means `QueueOp` may return an
  "unknown folder" validation error — surfaces via the existing
  `ErrorMsg` banner path.
