# Pass 1: Foundation

**Date:** 2026-07-27
**Pass:** 1 of 6, from the requirements' build-order spine
(section 15 step 1).
**Binding inputs:** the technical design revision 2 and its ADRs,
the requirements revision 4, ADR-0014 (testing strategy), and the
build machine design (`2026-07-27-poplar-build-machine.md`), whose
section 5 defines this task format.

Pass 1 builds everything under the UI. At its close poplar has no
screens, and it has a store that a real Fastmail account fills, a
sync engine that converges against the live server, a durable
outbox, a full-text index, and the QA-1/2/3 perf harness reporting
numbers against a 100k-message store. The gate for the pass is
that those numbers hold.

## Scope decisions this plan settles

The Phase 5 digest surfaced five questions the specs left open.
They are settled here so no task has to guess.

1. **Pass 1 ships a real `internal/backend/jmap` read path.**
   ADR-0014 calls the scriptable fake "the seam's second
   implementation", which presupposes a first. A sync engine
   validated only against a fake proves the fake. Pass 1 builds
   the JMAP mail source for real (`Changes`, `FetchBodies`,
   capability probe from the live session) and exercises it in a
   tagged live-account test. What defers to pass 2 is the
   onboarding flow (ST-1 through ST-3): pass 1 reads the token
   from config or `$FASTMAIL_API_TOKEN` through a minimal
   `internal/keyring`, and no task builds an auth UI.
2. **The CalDAV transport is designed, not implemented.** The
   spine says the calendar store schema and engine shape land
   here. Task 3 creates the calendar tables and the occurrence
   index; `internal/backend/dav` gets its interface conformance
   and nothing more. Live CalDAV is pass 5.
3. **Each ADR-0014 layer-3 harness lands with its subject.** The
   requirements state the rule ("QA harnesses run from the step
   that creates their subject"), and most subjects are pass 1's.
   Transaction tests, SY-8's three failure tests, the EXPLAIN
   QUERY PLAN goldens, the randomized mutation-search script,
   synctest suites over the fake, idempotent-replay per intent
   kind, and the QA-1/2/3 harnesses all land in this pass. The
   migration-from-N-1 harness gets its scaffolding now and its
   first real exercise at v1.1, since v1 has no N-1.
4. **The QA-6 kill harness runs at smoke scope.** One seed, and
   only the three action kinds whose subsystems exist by the end
   of this pass: triage, bulk, and folder rename. Compose, send,
   RSVP, and event edit join it in passes 4 and 5. The script is
   written so adding an action is a table entry.
5. **`message.flags` is a bitfield plus JSON overflow**, with
   bits assigned only to the flags a query filters or sorts on
   (seen, flagged, draft, answered). Everything else lives in the
   `data` JSON. Task 1 records the assignment in a comment beside
   the constant block, because a bit assignment is a compatibility
   surface a later migration has to respect.

## Task order and dependencies

Tasks 1, 2, 3 are the store's spine and run in order. Task 4
(`uerr`) has no dependency and should dispatch first alongside
task 1, since every later task returns its errors. Tasks 5 and 6
depend on 2. Tasks 7 through 9 are the backend and sync chain.
Task 10 depends on 2 and 9. Tasks 11 through 13 close the pass.

---

## Task 1: Schema, migrations, and the flags encoding

**Requirements:** SY-1, C4, and the schema half of SR-1.

**Outcome.** A fresh poplar creates its store at
`$XDG_DATA_HOME/poplar/store.db` and applies schema version 1.
Every table from technical design section 3 exists with the
scalar columns that document names, every account-scoped table
carries `account_id`, and the reserved-for-LATER scalars
(`message.hidden_until`, `thread.muted`, `mailbox.visible`,
`message.origin`) are present from this first migration so the
horizon items stay mechanisms rather than promises.

**Acceptance criteria.**
- `TestMigrateFresh` applies version 1 to an empty file and the
  resulting schema matches a committed golden dump.
- `TestAccountScopingRule` walks the live schema and asserts the
  rule as stated: every table not reachable by foreign key from a
  scoped parent carries `account_id`. It must catch
  `sent_history`, which is the case the rule exists for.
- `TestExplainQueryPlan` holds a golden per hot query: the list
  query over `message_mailbox(mailbox_id, received_at DESC,
  message_id)`, the cross-folder thread query, the unread partial
  index, `outbox(state, next_attempt_at)`, and both occurrence
  indexes. A plan that degrades to a scan fails the test.
- `TestFlagsRoundTrip` covers each named bit plus overflow into
  `data`.
- The migration runner rejects a store whose `schema_version`
  exceeds the binary's known maximum, with a typed error naming
  the versions.

**Boundaries.** `internal/store` only. Migrations are the
hand-rolled `schema_version` runner over `go:embed` SQL from
ADR-0001; do not add goose or golang-migrate. Do not create the
read or write API here; this task is schema plus runner. Note
ADR-0001's revision 2: one store file holds every account, and
FTS5 uses a single content source, both of which supersede that
ADR's Decision text.

**Salvage.** `internal/cache/schema.go` and `schema_test.go` on
branch `legacy` carry the migration-runner shape and its version
tests. The schema itself is new; do not port the old tables.

---

## Task 2: The writer, the connection discipline, and admission

**Requirements:** SY-1, QA-2, and the CO-6 admission term.

**Outcome.** One writer goroutine owns the single write
connection and applies transactions arriving over a channel.
Every connection is built by one DSN-builder function that is the
only place the pragma set is spelled (`foreign_keys=ON`,
`busy_timeout`, `journal_mode=WAL`, `synchronous=NORMAL`,
`cache_size`). The writer runs two lanes: interactive work
preempts bulk at chunk boundaries, and any single transaction is
bounded at roughly 50 ms of work. Checkpointing is writer-owned:
`wal_autocheckpoint` off, PASSIVE between batches, TRUNCATE at
defined idle, bounded by `journal_size_limit`.

**Acceptance criteria.**
- `TestWriterSerializes` proves concurrent submitters commit in
  arrival order per lane and never interleave a transaction.
- `TestInteractivePreemption` shows an interactive transaction
  admitted while a chunked bulk job is in flight, with the
  measured wait under the admission ceiling.
- `TestBackfillSubordination` asserts backfill yields on recent
  interactive activity. The signal is recent activity, not queue
  depth: ADR-0003 revision 2 records that queue depth is empty
  exactly when the writer is busy, so a depth-based test would
  pass while the behavior was wrong.
- `TestCheckpointLifecycle` covers PASSIVE between batches and
  TRUNCATE at idle, asserting WAL size returns to bound.
- A disk-full injection commits nothing partially and returns the
  typed failure.

**Boundaries.** `internal/store` only. Pick and record the
`busy_timeout` and `cache_size` values; no document specifies
them, and the choice must be justified in one comment against the
QA-5 RSS ceiling. Do not build the read API here.

**Salvage.** `internal/cache/drainer.go` on `legacy` for the
serialized-writer shape. The two-lane admission policy is new.

---

## Task 3: The read API and the read-only handle type

**Requirements:** LT-1, QA-2, QA-3, and the SY-1 read path.

**Outcome.** Reads are served from a pool that never queues
behind the writer. The read API returns a named read-only handle
type exposing no `Exec` methods, so a write through a read
connection fails to compile. This is the type system carrying
what the write-call analyzer explicitly cannot: build machine
section 3 records that name-matching on `database/sql` handles is
a heuristic sold as a gate, so the compile-time guarantee is the
real mechanism. List reads are keyset-paginated windows over
scalar columns; the `data` JSON is never parsed on the list path.
Every read result carries the monotonic store revision it saw.

**Acceptance criteria.**
- `TestReadHandleHasNoExec` is a compile-failure fixture: a
  testdata package that attempts a write through the read handle
  and is asserted not to build. A runtime panic is not acceptable
  here; the point is that it cannot compile.
- `TestKeysetPagination` walks a seeded 100k-row mailbox forward
  and backward with no duplicate and no skipped row across
  boundaries, including rows sharing a `received_at`.
- `TestListReadTouchesNoJSON` asserts the list query's column set
  excludes `data`.
- `TestReadsDoNotBlockOnWriter` runs a sustained bulk write and
  asserts read p95 stays inside the QA-2 budget.
- `TestStoreRevisionMonotonic` proves revisions never regress and
  that a stale result is identifiable as stale.

**Boundaries.** `internal/store` only. Notifications post after
commit, never inside the transaction. Do not build the UI-side
100 ms coalescing here; that is pass 2's, and this task only
provides the revision it keys on.

**Salvage.** `internal/cache/reads.go` on `legacy`.

---

## Task 4: The error seam and the logging model

**Requirements:** ER-1 through ER-4, ADR-0013.

**Outcome.** `internal/uerr` provides the one exported
constructor through which every user-visible error is built,
carrying a typed class, a user-facing sentence, and the
underlying cause. Every error that reaches a banner, toast, or
modal also reaches the log through this one seam, so no
user-visible failure is invisible in the log.

**Acceptance criteria.**
- `TestConstructorIsTheOnlyPath` plus the error-construction
  analyzer green over the tree.
- `TestEveryClassLogs` covers each failure class and asserts a
  log line with the class, cause, and correlation to the user
  sentence.
- Log level and destination follow ADR-0013; a test asserts the
  default file location and that debug is opt-in.

**Boundaries.** `internal/uerr` only. No dependency on `store` or
any engine; every other package depends on this one.

---

## Task 5: FTS5 index and its transactional maintenance

**Requirements:** SR-1, and the index half of QA-3.

**Outcome.** `message_fts` is FTS5 over `message(subject,
search_text)` from a single content source, declaring
`prefix='2 3'` for search-as-you-type. One store-internal helper
owns every FTS maintenance statement, reads current row state
inside the same transaction, and runs in the same transaction as
the message write that produces `search_text`. A message indexed
before its body arrives is re-indexed in the backfill transaction
that lands the body. `--rebuild-index` regenerates the index as
derived state.

**Acceptance criteria.**
- `TestIndexTransactional` asserts that a rolled-back message
  write leaves no index row, and a committed one leaves exactly
  one.
- `TestBackfillReindexes` covers the index-before-body case.
- `TestMutationSearchConsistency` is the randomized script: a
  seeded sequence of inserts, updates, deletes, and moves, with
  a search assertion after each, closing with `INSERT INTO
  message_fts(message_fts) VALUES('integrity-check')`. The
  closing pragma is required because row-count equality does not
  detect term rot.
- `TestRebuildIndex` corrupts the index, rebuilds, and asserts
  search results match a pre-corruption baseline.

**Boundaries.** `internal/store` only. The query grammar and
search-as-you-type UI are pass 3's `internal/search`; this task
builds no grammar and no MATCH compiler beyond what its own tests
need. Do not use external-content tables: ADR-0002 revision 2
found that shape not constructible.

**Salvage.** `internal/cache/fts.go` and `fts_test.go` on
`legacy`.

---

## Task 6: Role classification

**Requirements:** FO-1.

**Outcome.** The backend maps server-declared mailbox roles, and
a store-internal classification helper applies a tested
name-heuristic fallback from a committed name-to-role table.
Duplicate roles resolve first-created, with a log line, never a
sync refusal.

**Acceptance criteria.**
- `TestRoleTable` is table-driven over the committed name list,
  including non-English and punctuated names.
- `TestDuplicateRoleResolution` asserts first-created wins, that
  a log line records the collision, and that sync continues.
- `TestServerRoleWins` asserts a server-declared role is never
  overridden by the heuristic.

**Boundaries.** `internal/store`, not a new package. The helper
is not exported beyond the store's API.

---

## Task 7: The backend seam and the scriptable fake

**Requirements:** the C-level backend seam, ADR-0004, ADR-0014.

**Outcome.** `internal/backend` declares the `Backend` interface
and `Capabilities` as shapes rather than JMAP vocabulary, and
ships the scriptable fake that ADR-0014 names the seam's second
implementation. Thread identity is the three-valued fact (`None`,
`ReferencesDerived`, `ServerHeuristic`). Capabilities carries
push transport, delta granularity, server-search support, the
RSVP mechanism, per-capability account ids, and the server limits
read from the live session.

**Acceptance criteria.**
- `TestFakeScripting` drives every scripted condition ADR-0014
  names: state reset, 412, push drop, and throttled first sync.
- `TestCapabilityDefaults` asserts a backend that declares no
  calendar returns a nil `Calendar()` and that callers handle it.
- The import-boundary analyzer is green: `backend` implementations
  are the only packages speaking a wire protocol.

**Boundaries.** `internal/backend` and `internal/backend/dav`
(interface conformance only). No live CalDAV. Do not mirror JMAP
method names into the interface: ADR-0004 revision 2 rejected
that because every future backend would then emulate JMAP
semantics it does not have.

**Salvage.** `internal/mailjmap/fake_test.go` on `legacy` for the
fake's scripting shape.

---

## Task 8: The JMAP mail source

**Requirements:** SY-2, SY-6, and the backend half of SY-1.

**Outcome.** `internal/backend/jmap` implements the mail source
against Fastmail. `Changes` composes changes and get with
back-references in one request and returns hydrated objects.
`FetchBodies` streams. The capability probe reads server limits
from the live session. Credentials resolve through
`Credentials.Token`, which owns expiry and single-flight refresh
even though v1's static token makes it a read.

**Acceptance criteria.**
- `TestChangesOneRoundTrip` asserts a single request carries
  changes plus get via back-reference, against a recorded
  fixture.
- `TestChangesPaging` covers `HasMore` and token advance.
- `TestCannotCalculateChanges` returns the typed signal the sync
  engine turns into a full resync.
- A `//go:build live` test against the real account fetches
  changes and one body, asserting the probe's limits are
  populated. It never runs in CI.

**Boundaries.** `internal/backend/jmap`. Read path plus the
submit and batch entry points the outbox needs; no compose
assembly (pass 4). No UI, no config beyond reading the token.

**Salvage.** `internal/mailjmap/changes.go`, `wire.go`,
`errors.go`, and `probe.go` on `legacy`, all copy-with-rewrite
against the new seam.

---

## Task 9: The sync engine

**Requirements:** SY-1 through SY-5, QA-4.

**Outcome.** Per account and object kind the engine persists two
watermarks, the opaque server state token and a local revision.
Changes apply through the writer's bulk lane. EventSource push
delivers `StateChange`, and the worker coalesces with a fixed
200 ms delay measured from the first event, never a resetting
debounce, so a steady remote burst cannot defer sync
indefinitely. Stream drop falls back to jittered exponential
backoff and re-establishes push, and a missing ping past twice
the requested interval counts as a drop. The dispatcher's state
tokens suppress self-echo. Token expiry or
`cannotCalculateChanges` runs a full resync as a normal path,
reconciling by `server_id` and preserving `origin='local'` rows,
their bodies, drafts, and undispatched outbox rows.

**Acceptance criteria.**
- `TestPushCoalescing` under `testing/synctest` asserts the fixed
  200 ms window and that a sustained event stream still syncs on
  schedule.
- `TestBackoffRecovery` asserts push re-establishment inside
  SY-2's 30 s p95 bound over 20 synthetic trials.
- `TestStallDetection` covers the missing-ping threshold.
- `TestSelfEchoSuppressed` proves a dispatched mutation does not
  round-trip into a re-apply.
- `TestFullResyncPreserves` asserts every member of the preserved
  set survives a resync, and that no surviving row is re-minted a
  new internal key.
- `TestConvergence` runs QA-4's trials under virtual time.

**Boundaries.** `internal/sync`. No `queryChanges` maintenance:
ADR-0005 revision 2 struck it as redundant against the local
store being the list view. Thread rows derive from message thread
ids; there is no `Thread/changes` round trip. Conflicts resolve
by server state ordering with local losing ties, never
field-merged.

**Salvage.** `internal/cache/syncer.go` and `backfill.go`, and
`internal/mailjmap/push.go`, on `legacy`.

---

## Task 10: The outbox

**Requirements:** SY-4, UX-9, CO-7's queue half, and the
mutation discipline of ADR-0006.

**Outcome.** Every mutation is a durable intent row carrying
kind, payload, and undo group, with payloads referencing
poplar's internal keys only and the dispatcher resolving to
server ids at dispatch time. The dispatcher claims an intent by
moving `queued` to `dispatching` inside a writer transaction
before any I/O, and undo annihilation is legal only against
`queued` rows and decided in that same transaction, so the race
with an in-flight dispatch cannot exist. Undoable intents hold in
`queued` for the 10-second UX-9 window. Bulk actions enqueue
chunked sub-intents sized under the backend's limit, sharing an
undo group and ordered by chunk sequence.

**Acceptance criteria.**
- `TestClaimIsTransactional` drives concurrent undo and dispatch
  and asserts no intent is both annihilated and sent.
- `TestIdempotentReplay` covers every intent kind: replaying a
  dispatched intent leaves the same server and store state.
- `TestChunkedBulk` asserts each chunk's store write fits the
  writer's admission ceiling, that the compensating group
  restores exact prior state per message, and that partial
  dispatch retries only unfinished chunks.
- `TestFailureClasses` covers `auth` (including the
  `refresh-failed` sub-reason), `not-found`, `connection`,
  `server`, and `throttled`, asserting `throttled` surfaces as a
  warn state and never an error toast.
- `TestKeyResolutionAtDispatch` covers an offline
  create-folder-then-move dispatching as one batch through
  creation-id back-reference.

**Boundaries.** `internal/outbox`. The optimistic overlay lives in
the UI root model and is pass 2's; this task provides the
writer-side commit and the ack the overlay will clear on. No send
UI, no compose.

**Salvage.** `internal/cache/outbox_reads.go` and
`outbox_gate_test.go` on `legacy`.

---

## Task 11: Instance lock and recovery

**Requirements:** SY-7, SY-8, and the startup half of QA-1.

**Outcome.** Startup takes `LOCK_EX|LOCK_NB` through
gofrs/flock on a lock file beside the store and writes its pid
into the file afterward as advisory display data. A second
instance refuses to start, printing the pid and what to do.
Integrity checking is event-driven: a clean-shutdown marker is
written at exit, and `quick_check` plus the FTS integrity check
run only on a missing marker, after a migration, or on explicit
request, with a visible progress state. Detected corruption or a
failed migration offers rebuild-from-server, which preserves and
re-imports undispatched outbox rows, drafts with their local
revisions, and `origin='local'` messages with their bodies.

**Acceptance criteria.**
- `TestSecondInstanceRefused` asserts the refusal and the pid in
  the message.
- `TestLockReleasedOnKill` SIGKILLs the holder and asserts a new
  instance starts, proving no stale-lock heuristic is needed.
- `TestIntegrityCheckSkipped` asserts a clean shutdown skips
  `quick_check` entirely. This is the QA-1 gate: the spike
  measured 14.5 s for `quick_check` on a 924 MB store, so a
  synchronous check would destroy startup.
- SY-8's three tests: forced corruption, failed migration, and
  full disk, each asserting the typed error, the offered
  recovery, and the preserved set after rebuild.

**Boundaries.** `internal/platform` for the lock,
`internal/store` for recovery. No UI; recovery surfaces through
`uerr` and the CLI for now.

---

## Task 12: The QA-1/2/3 perf harness

**Requirements:** QA-1, QA-2, QA-3.

**Outcome.** A harness measures startup, interaction latency, and
search latency against a seeded store at the QA-5 envelope, using
`testing.B.Loop` with benchstat and writing artifacts through
`T.ArtifactDir`. Baselines are recorded from the first list
render, which at this pass means the store's first list page.

**Acceptance criteria.**
- `QA1Startup` measures exec to first list page, asserting p95
  under 200 ms warm and 500 ms cold, with `quick_check` off the
  launch path.
- `QA2Interaction` runs the scripted 500-operation mix at the
  100k envelope and asserts p95 under 25 ms and p99 under 40 ms,
  including while a bulk backfill runs concurrently. The gate is
  25 ms; 20 ms is the design target the implementation aims
  under.
- `QA3Search` covers the committed query set in four classes
  against the 100k index, asserting the per-class p95 bounds.
- A recorded baseline artifact lands for each, and the pass
  reports measured numbers against the spike's: 22-25 ms p95 for
  QA-2, 0.9-4.5 ms p95 for QA-3.
- The harness fails loudly if invoked under `-race`. Race
  instrumentation costs 2-20x time and 5-10x memory, so a p95
  asserted under it measures the detector.

**Boundaries.** Test-only code plus whatever `--startup-trace`
instrumentation QA-1 needs in `cmd/poplar`. The seeded store
generator may amplify a small fixture set; it must not require
the private corpus.

**Salvage.** `cmd/perfspike/bench.go`, `store.go`, and
`amplify.go` on `legacy` are the direct lineage; the spike that
produced the numbers above is that tool.

---

## Task 13: The kill harness at smoke scope

**Requirements:** QA-6 at pass 1 scope.

**Outcome.** A seeded script drives store actions and SIGKILLs at
pseudorandom points, and each restart asserts the store's
invariants. Scope at this pass is one seed over triage, bulk, and
folder rename, the three action kinds whose subsystems exist.
Adding an action is a table entry, because passes 4 and 5 add
compose, send, RSVP, and event edit.

**Acceptance criteria.**
- After each kill, restart asserts: the integrity check passes,
  including the FTS5 `integrity-check` pragma; no outbox row
  exists without its committed mutation and none the reverse; and
  the store revision is monotonic across the restart.
- `TestActionTableComplete` asserts the table lists every action
  kind the current build supports, so a later pass adding an
  intent kind without adding its action fails the test.
- The smoke run is part of the `test` gate step, not a separate
  CI job. Full scope (30 actions, 200 kill points, three seeds)
  becomes a per-push CI job at pass 6.

**Boundaries.** Test and script code only.

---

## Pass close

The pass closes when every task is accepted, `make check` is
green, the reviewer fan-out is clean, and the QA-1/2/3 numbers
are recorded in the STATUS against the spike's baselines. The
carried obligations that open later passes (the clipboard spike
in pass 2, the column formulas and grading harness in pass 3, the
Catkin incremental-renderer spike in pass 4, the iCalendar
bake-off and the CalDAV probes in pass 5) are not this pass's
work and must not be started inside it.
