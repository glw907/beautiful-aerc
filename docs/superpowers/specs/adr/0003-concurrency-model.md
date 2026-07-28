# ADR-0003: fixed worker cast with a single store writer

Date 2026-07-27. Status: accepted (Phase 4).

## Context

QA-2 requires p95 under 20ms key-to-render while initial sync or a
20k-body backfill runs concurrently, and names a mechanism test: a
UI-thread write must fail the build. SY-4 requires mutations and
outbox rows in one transaction. modernc/sqlite wants one writer
(ADR-0001).

## Decision

A fixed cast: the bubbletea UI loop (never blocks; reads as
`tea.Cmd` on the read pool), one writer goroutine owning the single
write connection (all writes arrive as transaction functions over a
channel), one mail sync worker per account, one groupware poll
worker, one outbox dispatcher, one body backfill worker. Workers
post messages to the UI through `Program.Send`; the store is the
only shared state. The store's write intake is the only write API;
a vet-class analyzer keeps write calls out of `internal/ui`.
Backfill subordinates itself by checking writer queue depth and
recent interactive read activity before each batch.

## Alternatives considered

- **Goroutine per folder or per request**: unbounded concurrency
  against a single-writer engine produces SQLITE_BUSY storms and
  makes the QA-6 invariants probabilistic. Mailspring's fixed
  two-worker design is the field evidence for the small cast.
- **Actor framework / message bus abstraction**: the cast is six
  named goroutines; a framework would abstract less code than it
  adds.
- **Synchronous UI reads in Update**: simpler, but couples frame
  time to store latency; a slow query would freeze input. Commands
  keep the loop honest and teatest can still assert the
  one-Update optimistic paint (LT-2).

## Consequences

All write ordering is trivially serial, which makes the outbox
transaction guarantee and the kill harness tractable.
`testing/synctest` can drive the entire cast deterministically.

## Revision 2 (2026-07-27, post-review)

- **Writer admission policy**: two lanes (interactive and bulk),
  a ~50 ms single-transaction ceiling, bulk work chunked with the
  interactive lane preempting at chunk boundaries. Backfill
  subordination reads recent interactive activity, not queue
  depth (which is empty exactly when the writer is busy). CO-6's
  loss bound is debounce plus the admission ceiling, measured.
- **Checkpoint policy is writer-owned**: `wal_autocheckpoint`
  off, PASSIVE checkpoints between batches, `journal_size_limit`,
  TRUNCATE at defined idle; WAL size is a QA-5 harness number.
- **Optimistic paint is a named mechanism**: a pending-intent
  overlay in the root model paints in the same Update (LT-2);
  post-commit store-changed notifications carry a monotonic store
  revision and stale read results are discarded, so re-query can
  never revert a paint; the writer ack clears the overlay or
  reverts it with an ER-1 toast. The SIGKILL window between paint
  and commit is bounded by the admission ceiling and stated
  honestly.
- Workers share one request budget derived from backend limits.
