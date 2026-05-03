---
title: Cache 0 — unified write path through cache.QueueOp
status: accepted (parts superseded by 0117)
date: 2026-05-02
---

## Context

Today (Pass 8.3), poplar's triage actions call `mail.Backend` methods
directly from `tea.Cmd` closures, with optimistic UI applied at the
App layer (ADR-0089). Adding offline triage requires deciding whether
the offline path is a parallel code path or an extension of the
online one.

Mailspring's TaskProcessor model (read at source level for this
design) routes every action — online and offline — through a single
queue: optimistic local change, then network call, then status
update. This is a deliberate architectural choice; it gets one code
path tested in both contexts and eliminates the "started online,
went offline mid-action" edge case.

The alternative (separate online and offline write paths) doubles
the surface area and introduces "which path am I on right now"
state.

## Decision

Every triage write — online or offline — flows through
`(*cache.Account).QueueOp(ctx, op)`:

1. `BEGIN TRANSACTION`
2. `INSERT INTO outbox` with `status = 'pending'`.
3. Apply the optimistic state to the `messages` table (`ui_flags`,
   `ui_hide`).
4. `COMMIT`.
5. Signal the per-account drainer goroutine.

There is no separate "online write" function. When online, the
drainer picks up the row within milliseconds and runs it. When
offline, the row sits until reconnect.

Optimistic UI is encoded in two new columns on `messages`:

- `ui_flags` — UI-side flags. UI reads this; sync writes confirmed
  state to `flags`.
- `ui_hide` — boolean; mid-move source rows hide from the source
  folder before the server has confirmed.

UI reads filter `WHERE ui_hide = 0`. The cache's drainer updates
`flags` and clears `ui_hide` on backend confirmation.

`AccountTab` no longer holds `mail.Backend` directly; it holds
`*cache.Account`. Triage `tea.Cmd` closures call cache methods, not
backend methods. `App` constructs `*cache.Cache` and threads
`*cache.Account` into each tab.

Op kinds in Cache 0 scope: `move`, `flag`, `destroy`. `send` and
`append` reserve their kind strings for Pass 9 (Compose) but are not
implemented in Cache 0–III.

## Consequences

- One write code path. Online performance is measured against the
  same code that runs offline.
- Optimistic UI becomes a property of every action, not a bolt-on
  per-action.
- The "online → offline mid-action → online" case is structurally
  handled — the row is in the outbox; the drainer picks it up when
  online resumes.
- Migration cost: existing triage Cmds (move, flag, destroy) move
  from "call backend directly" to "call `cache.QueueOp`". The Pass
  8.4a plan must spell out the migration order so the UI never
  observes a partially-converted state.
- Existing optimistic-UI scaffolding in App (ADR-0089) gets
  partially superseded; the post-action toast/undo machinery stays
  but its state lives in the cache instead of in App.
- The cache becomes the action API the UI talks to; the protocol
  abstraction (`mail.Backend`) becomes invisible to the UI.
- Pre-beta (ADR-0105): refactor freedom permits the API rename.
  Beta-soak does not.
- **Superseded in part by ADR-0117** — `QueueOp` takes a folder
  *name* (not a row id), an `OpArgs` sealed sum (not
  `map[string]interface{}`), and the cache exposes
  `Events() <-chan CacheEvent` for drainer→UI signaling. Undo's
  synchronous-mutation half is dropped; undo fires only the
  compensating `tea.Cmd`. The unified-write-path principle and the
  `ui_flags` / `ui_hide` columns stand.
