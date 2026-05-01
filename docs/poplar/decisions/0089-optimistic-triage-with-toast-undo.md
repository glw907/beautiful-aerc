---
title: Optimistic triage with App-owned toast + undo
status: accepted
date: 2026-05-01
---

## Context

Pass 6 wires the message-list triage vocabulary (delete, archive, star,
read-toggle). Backend mutations are slow on real networks, but the
local UI must feel instant. We also need a recovery surface so a
mis-triaged message isn't lost — but without a modal that interrupts
the next keystroke. The error banner shape (ADR-0073) already
demonstrates how to render a one-row chrome line above the status bar
that resizes the content area instead of overlaying it.

## Decision

Triage is optimistic and uses the same chrome row as the error banner,
shared with banner-wins precedence.

- `MessageList` exposes `Apply*` mutation helpers (`ApplyDelete`,
  `ApplyInsert`, `ApplyFlag`, `ApplySeen`) that flip local state
  without firing any `tea.Cmd`. `AccountTab.dispatchTriage` snapshots
  inverse data, calls the appropriate `Apply*`, exits visual mode,
  and emits a `triageStartedMsg` plus the forward backend Cmd via
  `tea.Batch`.
- `App` owns `pendingAction` (the toast). `triageStartedMsg` sets it
  and schedules a `tea.Tick` for the configurable undo deadline
  (`[ui] undo_seconds`, default 6, clamped to [2, 30]).
- `u` while a toast is active emits `undoRequestedMsg`, which fires
  `pendingAction.onUndo` (local rollback) and the saved inverse Cmd.
- A folder change (`folderQueryDoneMsg{reset:true}`) clears the toast
  *without* firing the inverse — the navigation commits the action.
- An `ErrorMsg` while a toast is active runs `onUndo` (local rollback)
  before setting `lastErr`, so a backend failure visibly reverts the
  optimistic flip.
- Error banner wins precedence over the toast: `App.chromeBannerRow`
  renders the banner if `lastErr.Err != nil`, otherwise the toast,
  otherwise empty (the row collapses).
- `pendingAction.IsZero()` checks `op == ""` only — `op` is required
  for rendering, so it's the load-bearing field.
- `dispatchTriage` shares a `buildTriageCmd` helper that owns the
  forward/inverse/start closure assembly; the per-action helpers
  (`dispatchRemoval`, `dispatchFlagToggle`, `dispatchSeenToggle`)
  perform the local flip and call `buildTriageCmd`.

## Consequences

- One chrome row covers both error and toast: View height is unchanged
  whether neither, one, or (in race conditions) both states would
  apply. Resize fires only on transitions into or out of "row
  occupied".
- Undo is bounded by the deadline; once it elapses, the action is
  permanent. There is no "permanent undo log" — by design.
- Folder navigation is a commit signal, not a cancel signal: a user
  who triages then jumps folders accepts the action. This matches the
  triage model where the next folder is the next decision.
- An ErrorMsg always rolls back the optimistic flip — a backend
  failure cannot leave the UI in a desynced state. The banner then
  shows the verb that failed.
- Visual mode auto-exits on every triage dispatch; multi-select is a
  one-shot input modifier, not a sticky mode.
