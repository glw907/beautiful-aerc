---
title: Cache 0 — drain-first sync ordering and syncer/drainer coordination
status: accepted
date: 2026-05-02
---

## Context

ADR-0112 specified that on reconnect the syncer runs
`ChangeTracker.Changes()` *before* the drainer drains queued ops,
on the intuition "fetch first, then act." Pass 8.4-review (subagent
A, finding A1) found this contradicts RFC 4549 §6 ("Processing
Offline Queues"), which mandates the opposite for IMAP/JMAP-class
offline-queue clients: queued client actions first, then pull
server-side changes. The FK CASCADE protection from remote-removed
messages still works under drain-first because `notFound` from the
backend is already an idempotent-success outcome in the conflict
matrix.

Subagent B (finding B4) further found that under either ordering,
nothing in the spec prevented the syncer from updating `ui_flags`
for a message that has an in-flight queued `flag` op — a server
push received while the drainer is mid-execute would revert the
optimistic display before the local action lands.

## Decision

**Drain-first ordering.** The per-account scheduler drains the
outbox to completion before running `ChangeTracker.Changes()` on
reconnect. Cited in spec §D.3 with an RFC 4549 §6 reference. IMAP
UIDVALIDITY signals received during drain still fence both
connections (see ADR-0114) — drain-first does not weaken the re-key
contract.

**Syncer/drainer coordination invariant.** The syncer (`Changes()`
poll loop, plus JMAP push and IMAP IDLE deltas) MUST NOT update
`ui_flags` for any message whose `messages.id` appears in an outbox
row with status `pending` or `executing`. The syncer updates only
`flags`. The drainer is solely responsible for converging
`ui_flags → flags` after backend confirmation. Implemented via the
`outbox_message` partial index and an EXISTS check during sync
writes.

## Consequences

- Supersedes ADR-0112's sync-first ordering. The drain-first
  text in spec §D.3 is the binding statement; ADR-0112's other
  decisions (state machine, conflict matrix shape, no-coalescing,
  enqueue-order replay) stand.
- The "I went offline, queued moves, came back online" case now
  applies the queued moves before learning about server-side
  parallel changes. Server changes that arrive in the next
  `Changes()` cycle are reconciled via the same conflict matrix —
  apply-ours on flag conflict, idempotent success on `notFound`.
- The coordination invariant adds an EXISTS subquery to the
  syncer's `ui_flags` UPDATE path. Cost is negligible with the
  partial index; correctness is significant.
- The `outbox_message` partial index is part of the v1.0-frozen
  schema. Adding it later would be a beta-soak migration.
