---
title: Conflict resolution primitives in cache
status: accepted
date: 2026-05-03
---

## Context

When the drainer marks an outbox row `conflict`
(auth-failure, max-attempts-exceeded, args-decode,
crashed-mid-execute), the row sits idle until the user resolves
it. Two resolution paths are possible:

- **Retry** — flip back to `pending` and let the drainer
  attempt it again.
- **Discard** — give up on the op; clean up local state.

The architectural question for discard was where the rollback
lives. Three shapes were considered:

1. UI-side discard via `cache.QueueOp` of an inverse op.
2. Cache-side discard with a forward-style inverse op.
3. Cache-side discard with a local-state revert mirroring
   `applyOptimisticTx`.

(1) and (2) round-trip through the drainer to undo something
the drainer never executed in the first place — wrong shape.

## Decision

Discard reverts local optimistic state and deletes the outbox
row in one transaction. No round-trip through the drainer, no
forward-style inverse op.

`internal/cache/conflicts.go` introduces three primitives:

- `(*Account).RetryOp(ctx, opID)` — `attempts = 0`,
  `next_eligible_at = NULL`, status → pending; signals drainer.
- `(*Account).DiscardOp(ctx, opID)` — calls
  `revertOptimisticTx`, then deletes the outbox row.
- `revertOptimisticTx(tx, msgID, args)` — private mirror of
  `applyOptimisticTx`. Move/Destroy → `ui_hide = 0`; Flag →
  flip the bit the other way; Send/Append → unsupported error.

Both public methods reject non-conflict rows with the
`ErrNotConflict` sentinel; UI treats this as benign (the row
was resolved by another path).

`attempts = 0` on user-initiated retry grants a fresh budget so
an `auth-failure` with attempts ≥ max doesn't immediately
re-enter conflict on the very next failure.

## Consequences

- Discard cleans local state but does not roll back across
  earlier successful ops. The user's mental model is
  per-conflicted-op.
- `revertOptimisticTx` must grow Send/Append cases when Pass 9
  introduces those op kinds. The compile-time error from the
  unsupported branch is the forcing function.
