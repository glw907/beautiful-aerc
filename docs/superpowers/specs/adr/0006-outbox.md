# ADR-0006: one durable intent queue for every mutation

Date 2026-07-27. Status: accepted (Phase 4).

## Context

SY-4 requires every mutation to flow through one durable outbox
with the mutation and its queue row in a single transaction, typed
failure classes, and nothing lost across a crash. UX-9 requires one
undo model; CO-7 requires undo send.

## Decision

Every mutation is an intent row: kind, payload, and the
compensating intent that undoes it. Enqueue commits the optimistic
local mutation and the intent row in one writer transaction. One
dispatcher drains intents in order per account, classifying
failures into the SY-4 reasons (auth → ST-5 flow with queue
preserved; connection → backoff retry; not-found → reconcile and
report; server → typed surface). Undo enqueues the compensating
intent: pre-dispatch, the pair annihilates in the queue;
post-dispatch, the compensation dispatches as a normal mutation
(the Mailspring local/remote task split). Send is an intent with a
10-second hold state; cancel restores compose, exit persists the
hold.

## Alternatives considered

- **Per-subsystem queues** (mail ops, sends, RSVP answers, event
  edits as separate mechanisms): multiplies the crash-consistency
  proofs QA-6 must make; ordering across queues becomes undefined
  exactly where the user notices (archive then send referencing
  the same thread).
- **Synchronous mutations with optimistic UI only for slow ops**:
  reintroduces the network on the interactive path (C1) the
  moment latency spikes.
- **Undo as state snapshot/restore**: snapshots race the sync
  engine; compensating intents ride the same ordering guarantees
  as everything else and resolve conflicts by the SY-3 rule.

## Consequences

The kill harness invariant is checkable by SQL: no committed
mutation without its intent row and none the reverse. Failure
handling is a closed enum with a rendered, logged outcome per
class (C7). RSVP answers are intents too, which is what makes the
CA-6 offline-queue and re-answer criteria uniform.

## Revision 2 (2026-07-27, post-review)

- **Claim discipline**: `queued → dispatching` inside a writer
  transaction before any I/O; annihilation is legal only against
  `queued`, decided in the same transaction, so the undo-versus-
  in-flight race cannot exist. Undoable intents hold in `queued`
  for the UX-9 window, making annihilation the common undo path.
- **Batches**: bulk actions enqueue chunked sub-intents under the
  backend's set limit, sharing an `undo_group`, per-chunk prior
  state for exact compensation, idempotent replay per intent
  kind, and retry of unfinished chunks only. Bulk-over-search
  re-runs the criteria uncapped at action time.
- **Payloads reference internal keys only**; the dispatcher
  resolves server ids at dispatch time (offline-created referents
  resolve through batch creation-id references).
- **`throttled` joins the failure enum**: retry-after-aware
  backoff surfaced as SY-5's warn state, never an error toast.
- Series-split edits (CA-5) are one intent carrying both CalDAV
  writes with defined partial-failure reconciliation.
