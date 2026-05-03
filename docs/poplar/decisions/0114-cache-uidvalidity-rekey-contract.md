---
title: Cache 0 — UIDVALIDITY re-key contract and anchor-lost outbox promotion
status: accepted
date: 2026-05-02
---

## Context

ADR-0110 / ADR-0112 noted that outbox rows reference `messages.id`
(not UIDs), so a UIDVALIDITY change is supposed to be transparent
to the queue: the IMAP folder sync code re-keys `protocol_id` and
the queue picks up the new UID at replay. Pass 8.4-review
(D-pattern-1, scenarios D1/D2/D9, plus D-pattern-2 for the JMAP
analog) found three load-bearing gaps:

1. The atomicity boundary of re-key was unspecified. A 10k-row
   re-key crashed mid-way leaves stale `protocol_id`s in
   unprocessed chunks; the drainer reads stale UIDs and silently
   acts on the wrong messages.
2. The old→new UID mapping strategy was unspecified. A wipe-and-
   refetch implementation silently CASCADE-deletes outbox rows
   pointing into the wiped folder.
3. UIDVALIDITY change observed on the IDLE connection while the
   command connection stays live mid-drain — the drainer keeps
   using stale UIDs on the command connection.

The same shape of failure occurs on JMAP `cannotCalculateChanges`
and on IMAP forced full-refetch when re-key matching fails: pending
intent gets silently destroyed by CASCADE.

## Decision

The Cache I implementation MUST follow the contract codified in
spec §D.4:

1. **Connection fence.** UIDVALIDITY change observed on either
   connection (command-after-SELECT or IDLE-on-reconnect) fences
   both connections: the per-folder drainer pauses, in-flight
   `executing` ops revert to `pending`, IDLE drops, and re-key
   runs against a freshly-opened command connection.
2. **Atomic re-key in a single SQLite transaction.**
   `UID SEARCH` + `UID FETCH (UID FLAGS RFC822.HEADER)` build an
   old→new `protocol_id` mapping by Message-ID + date. Inside the
   transaction: update matched `protocol_id`s, delete unmatched
   rows, **promote any pending/executing outbox row whose
   `messages` row would be deleted to `conflict` with
   `error.kind = 'rekey-orphaned'` before the implicit CASCADE**.
3. **Forced-refetch fallback shares the path.** JMAP
   `ErrCannotCalculateChanges` and IMAP re-key matching failure
   both run the same shape with `error.kind = 'anchor-lost'`.
4. **Resume on commit.** Drainer unpauses; syncer's next tick
   establishes the new baseline.

## Consequences

- Narrows ADR-0110 (storage architecture) and ADR-0112 (outbox
  state machine). The re-key contract is the binding text;
  ADR-0110 / ADR-0112 stand otherwise.
- Two new `error.kind` values are part of the v1.0-frozen
  outbox-error vocabulary: `rekey-orphaned`, `anchor-lost`. Both
  surface in the `!` Conflicts overlay (Cache III) with retry /
  discard actions.
- Connection fencing requires the IMAP backend to expose a
  pause/resume protocol that the cache scheduler can drive. This
  is a Cache I implementation concern.
- The remap-by-Message-ID approach is best-effort. Some servers
  rewrite or drop `Message-ID` on re-anchor; those rows fall
  through to the `anchor-lost` promotion. User-visible, not
  silent.
- The contract intentionally declines to special-case partial
  matches across re-anchor. Either a row remaps cleanly or it
  doesn't; the user sees the conflict either way.
