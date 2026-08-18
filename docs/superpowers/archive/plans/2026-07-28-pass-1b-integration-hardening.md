# Pass 1b: Integration and hardening

**Date:** 2026-07-28
**Pass:** 1b, inserted between passes 1 and 2. It discharges no new
MUST from the requirements' spine; it discharges the ones pass 1
claimed.
**Binding inputs:** the pass 1b scope in
`docs/superpowers/specs/poplar-refounding-STATUS.md`, the technical
design revision 2 and its ADRs (revision blocks override ADR
bodies), the requirements revision 4, and four research documents
dated 2026-07-28 under `docs/poplar/research/`: the JMAP test
inventory (the library's acceptance criteria), the JMAP adoption
assessment and library decision (the seven known defects and the
abort condition), the outbox dispatcher design review, and the
SQLite driver audit (the benchmark design).

Pass 1 built the foundation and closed with two claims it could
not demonstrate: the engines are never started, and QA-2's number
was measured against a corpus ~30x lighter than real mail. Pass 1b
makes the foundation real, then hardens it. Its gate is a
demonstration, not a checklist: poplar runs against the live
Fastmail account and Geoff watches the store fill.

## Global constraints

- Every task ends with `make check` green, verified by the
  orchestrator between dispatches.
- `go-conventions` binds every Go file. No bubbletea code exists in
  this pass, so `elm-conventions` does not arise.
- Every reviewer verification that claims a fix works must prove
  revert-sensitivity by experiment: revert the fix in a scratch
  worktree with tests untouched and watch the test fail. Pass 1 had
  three separate occasions where reading missed what the experiment
  caught.
- Every user-visible error reaches the log through the `uerr` seam.
- Work lands on master. Commits are imperative-mood, specific
  files, co-authored footer.
- The perf and benchmark measurements (tasks 9 and 10) run on a
  quiet machine with no implementer dispatched concurrently.
  Measuring while an agent builds is the methodological error the
  pass 1 ledger records.

## Scope decisions this plan settles

1. **The runner lands first, against the current library.** The
   STATUS lists the runner as scope item 1 and the `jmap` rewrite
   as item 2. The runner wires the engines that exist today, on the
   pinned go-jmap, because that makes `deadcode` a meaningful
   signal for every later task and gives the whole pass a running
   program to regress against. The library cutover (task 6) swaps
   the transport underneath a runner that already works.
2. **The `jmap` library is four tasks, not one.** The inventory's
   own build order is tier 4, then 1, then 2, then 3, and the
   adoption assessment orders adoption before typed models. Tasks
   2 through 4 build the library (data model, transport, push);
   task 5 validates it against Stalwart; task 6 cuts poplar over.
   Coverage of all 46 JT items is a ledger checked at task 6, not
   an assignment frozen per task, because the tiers cut across
   components.
3. **Five of the seven pass-end Major findings were never
   persisted.** The close session's four-lens fan-out enumerated
   seven unfixed Majors; only two survive on disk (the EXPLAIN
   goldens pinned against an empty database, and `fullResync`
   holding one unbounded transaction against ADR-0003's 50 ms
   cap). Task 11 fixes the recorded set and harvests every
   deferred finding from the pass 1 progress ledger; the pass-end
   fan-out at task 12 re-covers the ground the lost five occupied.
   Lesson encoded in the ritual: fan-out findings are persisted to
   the sdd directory before any fix round starts.
4. **The inherited-dependency re-derivations are not 1b scope.**
   No MIME library exists in the tree (`go.mod` confirms), and no
   1b task parses MIME. The re-derivations bind to the pass that
   first leans on each: the MIME and render stacks to pass 3,
   `go-webdav` to pass 5, `go-keyring` to the pass that builds the
   auth surface. bubbletea is settled by owner intent.
5. **The driver decision is made inside the pass, on the audit's
   own criteria.** Audit section 4.5 states, precisely, which
   results change the ranking. The orchestrator runs the benchmark,
   applies those criteria, and records the outcome in an ADR-0001
   revision block. Only a result the criteria do not decide
   escalates to Geoff before the gate.
6. **`Last-Event-ID` resumption gets ADR-0018.** RFC 8620 section
   7.3 states it in prose with no worked example, Apache James
   does not implement it, and go-jmap never reads an SSE `id:`
   field, so poplar sets the policy unilaterally and records it.

## Task order and dependencies

Task 1 (runner) is first and alone. Tasks 2 through 4 build the
`jmap` library in order; task 5 (conformance) needs 2 through 4;
task 6 (cutover) needs 5 green. Task 7 (typed models) needs 6.
Task 8 (outbox) is independent of 2 through 7 and may interleave.
Task 9 (seeder and re-measurement) is independent but must run
quiet; task 10 (driver benchmark) needs 9's corpus generator.
Task 11 (carry-forwards) is independent. Task 12 closes the pass
and needs everything.

---

## Task 1: The headless runner

**Requirements:** the pass 1b gate itself; ADR-0005 (sync engine),
ADR-0006 (outbox), ADR-0015 (instance lock).

**Outcome.** `cmd/poplar` runs the engines. After the existing
startup sequence (lock, migrate, integrity check, orphan sweep),
it resolves the token via `keyring.Token` (config value or
`$FASTMAIL_API_TOKEN`), dials the live backend with
`jmap.Dial(ctx, sessionURL, creds)`, constructs
`sync.NewWorker(accountID, backend, writer, sync.DefaultConfig())`
and `outbox.NewDispatcher(accountID, backend, writer)`, and runs
until interrupted: the sync worker on its push loop
(`Worker.RunPush`) with poll fallback per ADR-0005, the dispatcher
driven by a loop that calls `DispatchOnce` on an interval. A
missing token is a typed startup error naming both sources, not an
idle process.

**Interfaces.** All constructors above exist today with those
signatures (`internal/sync/sync.go:73`, `internal/outbox/dispatch.go:28`,
`internal/backend/jmap/session.go:41`, `internal/keyring/keyring.go:20`).
No composition type exists; composition is this task's product and
lives in `cmd/poplar`, not in a new package.

**Engine panic policy, carried from the dispatcher review.** The
orphan sweep (`outbox.ReclaimOrphaned`) is startup-only. An engine
loop that recovers a panic and keeps running would make the
stranded-row cases permanent for the process with no sweep behind
them. Ruling: engine loops do not recover panics; a panic
propagates and the process exits, and the next start sweeps. If a
recover is ever added, it must re-sweep or exit, and the analyzer
or a test must pin that.

**Acceptance criteria.**
- A `live`-tagged test (or the runner under the live token,
  operator-driven) shows messages and mailboxes from the real
  account landing in the store, and a triage intent enqueued
  locally landing on the server.
- Clean shutdown on SIGINT/SIGTERM: engines stop, writer closes,
  `MarkCleanShutdown` runs; proven by a test over the fake backend.
- A synctest suite over `backendtest.Fake` proves the composition:
  push event in, store row out; intent enqueued, dispatch observed.
- `deadcode ./...` drops from 203 unreachable functions to a
  number the task report enumerates: every remaining entry is
  named and justified (test-only helpers, pass 2 surface), and
  none is in the outbox, jmap, or sync engine paths.
- `deadcode` joins the pass-end ritual in this plan's task 12 and
  in the STATUS ritual text.

**Boundaries.** No UI, no onboarding flow (ST-1 through ST-3 stay
pass 2). No new package. Config stays the minimal existing shape.

---

## Task 2: The `jmap` library: data model and wire foundation

**Requirements:** owner ruling of 2026-07-28 (adopt, rewrite to
poplar idiom, reusable by other projects); test inventory tier 4
plus the type-level tier 1 and 2 items.

**Outcome.** Package `jmap` exists at `poplar/jmap`, non-internal,
importable by another project: no poplar types in its API, no
imports of poplar packages, its own package documentation.
go-jmap v0.5.3's RFC-verified data model is the starting material,
rewritten to poplar's standards (code and comments both; no Vale
exemption). The seven known defects land fixed in this task where
they live in type code: the `errors.go` format-string bug, the
dead `Account.ID` field, the four RFC-meaningful `omitempty`
booleans, and `SetError` implementing `error`. Attribution per
inventory section 2: `THIRD-PARTY-NOTICES.md` at the `jmap` root
with go-jmap's full MIT text, one pointer line in `jmap/doc.go`,
and the derivation named in the landing commit. No per-file
headers.

**Interfaces.** Produces the wire types, the request/response
envelope, `ResultReference` back-references, the method registry,
and typed capabilities that tasks 3 and 4 build on and task 6's
adapter consumes. The three near-verbatim carries from inventory
section 2 (the `echo_test` method-type template generalized to all
method types, the direct-versus-manual construction agreement
pattern, the explicit-null patch case) come across with
attribution; the seven rewrite-don't-patch test files are authored
fresh against JT-36 through JT-43.

**Acceptance criteria.**
- The tier 4 JT items pass, plus every tier 1 and 2 item that is
  purely type-level, each test citing its JT id.
- Fixtures live in `jmap/testdata/`, RFC-derived fixtures named
  for their source per the inventory's convention.
- Executable examples with `// Output:` comments replace the
  discarded example file.
- `go vet`, the linters, and Vale pass on the package with no role
  exemption.

**Boundaries.** No transport code in this task: nothing from
go-jmap's `client.go` or `core/push/eventsource.go` is ported, per
inventory section 2. The trimmed-out packages (`thread`,
`searchsnippet`, `vacationresponse`, `mdn`, `core/blob` beyond
what upload/download need) stay out until a feature needs them.

**Abort condition, verbatim from the adoption assessment.** If
fixing the defects changes a type signature that ripples into
poplar's callers, or the fix diff exceeds roughly 30% of the
adopted lines, stop and re-present; the adopt case rests on the
defects being local.

---

## Task 3: The `jmap` library: transport

**Requirements:** test inventory transport items (JT-22 through
JT-30 read go-jmap's `client.go` as a failure catalogue, not as
source); the two adoption-assessment tests (httptest-driven
`Do`/`Upload`/`Download`, end-to-end `ResultReference` resolution).

**Outcome.** poplar-authored transport in package `jmap`: session
fetch, `Do` with a streaming decoder rather than `io.ReadAll`,
context on every call, upload accepting any 2xx (the Cyrus 201
case), download streaming, and the three error shapes (HTTP
problem-details, per-call `MethodError`, per-object `SetError`)
distinguished and typed. The session state is read under a
coherent locking discipline; the go-jmap `Do` race (unlocked
`Session.APIURL` read) has no equivalent by construction.

**Acceptance criteria.**
- The transport-tier JT items pass against a scripted
  `httptest.Server`, each test citing its id.
- The reliance-spec suite from the library decision's section 3a
  exists here: back-reference resolution in a changes-plus-get
  batch, the forced core capability in `using`, `Patch` encoding
  on `/set`, the three error shapes, streaming download, upload
  status handling, and the `core.Core` limit fields, all driven
  against the fake server.
- No `io.ReadAll` on a response body anywhere in the package.

**Boundaries.** No EventSource code; that is task 4. No poplar
imports.

---

## Task 4: The `jmap` library: push client, and ADR-0018

**Requirements:** ADR-0005's push-first sync with 30 s p95 push
recovery; the inventory's push items including JT-22
(`Last-Event-ID` fallback against a purpose-built fake that
ignores the header).

**Outcome.** A poplar-authored EventSource client in package
`jmap`, written against the failure catalogue of the discarded
go-jmap implementation: context-aware (`Listen` cancellable),
reconnection with backoff, `Last-Event-ID` capture and resend on
reconnect, no fixed 64 KB line cap on server-controlled data,
multi-line `data:` payloads handled, `Close` errors surfaced. The
adopted `statechange.go` types (25 lines) live here with the same
attribution umbrella as task 2.

ADR-0018 records poplar's `Last-Event-ID` policy: what poplar
sends, what it assumes when the server ignores the header (James
does; Stalwart and Fastmail unverified), and how the sync engine's
resume correctness never depends on the header being honored,
since `/changes` state tokens are the durable resume point and the
SSE id is an optimization.

**Acceptance criteria.**
- Push-tier JT items pass, including reconnect-with-resume against
  a fake that honors the header and JT-22's fallback against a
  fake that ignores it.
- A stream longer than 64 KB in one event is delivered intact.
- ADR-0018 committed under `docs/superpowers/specs/adr/`.

**Boundaries.** The sync engine keeps owning poll fallback policy;
this client only reports stream state, it does not decide when to
poll.

---

## Task 5: Second-server validation against Stalwart

**Requirements:** owner ruling (Stalwart ships before the library
does); test inventory sections 3 and 4.

**Outcome.** The conformance suite exists behind a `conformance`
build tag beside the existing `live` suite: Stalwart v0.16.15 in
Docker, config at `jmap/testdata/conformance/stalwart.toml`,
session at `/.well-known/jmap`, target selected by the four
`POPLAR_JMAP_*` env vars from inventory section 3. One Makefile
target (`make conformance`) starts the container, waits for the
session endpoint, runs `go test -tags conformance ./jmap/...`, and
tears down. It is not part of `make check`, which must not require
Docker.

**Acceptance criteria.**
- The 12 divergence tests DV-01 through DV-12 are implemented and
  pass against Stalwart, each citing its DV id.
- The tier 1 and 2 JT items that section 3 says Stalwart can
  exercise run green under the `conformance` tag.
- The JT coverage ledger stands at 46 of 46: every inventory item
  has a named test in tasks 2 through 5, checked off in this
  task's report. A deliberately unimplemented item is a finding,
  not a silent gap.
- The `live` suite still passes against Fastmail, scoped to the
  three Fastmail-only validations inventory section 3 names.

**Boundaries.** Do not vendor `jmapio/jmap-test-suite` or
Fastmail's suite; both are unlicensed. James and Cyrus assertion
logic may be adapted with attribution. The Cyrus third target from
inventory section 5 is explicitly deferred until after this suite
is green; it is not 1b scope unless a divergence forces it.

---

## Task 6: Cutover: `internal/backend/jmapsource` on `poplar/jmap`

**Requirements:** owner ruling on naming (the bare name means the
protocol only); the adoption assessment's dependency-drop table.

**Outcome.** poplar's adapter renames from `internal/backend/jmap`
to `internal/backend/jmapsource` and is rewritten against package
`jmap`. `git.sr.ht/~rockorager/go-jmap` and `golang.org/x/oauth2`
leave `go.mod` with their transitive graph. The import-boundary
analyzer gains the carve-out: only `internal/backend/jmapsource`
may import `jmap`, and the analyzer's own tests prove the rule
fires on a violation (the pass 1 write-call analyzer matched no
real symbol for the whole pass; a boundary rule that guards
nothing is the defect class to pin against).

**Interfaces.** Consumes package `jmap` from tasks 2 through 4.
Produces the same `backend.Backend` surface the runner (task 1)
and sync engine already consume: `Dial`, `Mail()`, `Capabilities()`
keep their contracts, so task 1's runner and the existing
`live_test.go` are the regression net.

**Acceptance criteria.**
- The full existing adapter test surface (fixtures, fake-transport,
  `live` tag) passes against the new library.
- `make conformance` green after the cutover.
- The runner from task 1 fills the store from the live account,
  re-demonstrated after cutover.
- `go mod why git.sr.ht/~rockorager/go-jmap` reports nothing; the
  analyzer test proves the carve-out enforces.
- A `go-jmap`-fork recheck ran the day the task starts (the
  adoption assessment's cheap pre-adoption check): a maintained
  fork appearing changes the decision and is re-presented instead
  of executed over.

**Boundaries.** No behavior changes to sync or outbox in this
task; a wire-type regression must not hide in a semantic diff.

---

## Task 7: Typed models at the backend seam

**Requirements:** owner ruling of 2026-07-28 (typed models over
`map[string]any`; right way before easy way), including its
consequences 2 and 3.

**Outcome.** `backend.Record.Fields map[string]any` is replaced by
typed per-kind models. The types live in `internal/backend` (the
seam owns its vocabulary; no premature `internal/mail`);
`internal/sync` translates them into the store's upsert structs.
`internal/backend/jmapsource` and `backendtest.Fake` produce them.
Two deferred defects ride this seam change, per the ruling that
their deferral reason is gone: the create-replay idempotency
window (task 11b's DOCUMENTED_AND_PINNED deferral), and
silent-failure finding 8 (a store-write failure after a successful
`CreateMailbox` misclassified as a server failure, guaranteeing a
duplicate folder).

**Interfaces.** Consumes the cutover adapter from task 6. Produces
the typed model set that pass 2's read path will consume; names
and fields follow the technical design's section 3 vocabulary so
the store and seam agree.

**Acceptance criteria.**
- No `map[string]any` remains at the seam; the compiler, not a
  convention, enforces the shape.
- The create-replay window and finding 8 are fixed, each with a
  test that fails on revert (proven in a scratch worktree).
- The synctest suites and the kill harness pass unmodified in
  behavior; assertion-shape edits from the type change are
  expected, semantic edits are findings.

**Boundaries.** The store's upsert structs do not change; the
translation lives in `internal/sync`.

---

## Task 8: The outbox disposition enum and recovery simplification

**Requirements:** the dispatcher design review (its Q3 structure
and all four findings), already specified in
`docs/poplar/research/2026-07-28-outbox-dispatcher-design-review.md`.

**Outcome.** The review's simpler structure, adopted whole: one
`disposition` enum (`delivered | retry | terminal | landed`)
end to end, replacing the `failed`/`final`/`landed` booleans on
`outcome` and the `landed` bool on `finalizeAction`, which closes
the unenforced `landed implies final` invariant by construction.
The best-effort fallback is deleted; the recovery's requeue writes
only the non-growing columns (`state`, `attempt_count` incremented,
`next_attempt_at`, `failure_class`) and drops `failure_detail`.
The detached-context recovery stays; it is essential. The two
IMPORTANT findings (the fallback's no-backoff hot loop with the
`uerr` gate re-armed every pass; the `slog.Warn` that asserts a
revert before it happens) and the two MINOR findings (the
fallback test pinning only `state`; `failUnresolvedBatch`'s doc
not naming batched moves) close with it.

**Acceptance criteria.**
- The recovery-path tests assert the three columns the review
  measured (`attempt_count`, `next_attempt_at`, `failure_class`)
  alongside `state == queued`.
- A test proves a landed row is never requeued by recovery, and
  fails when the enum's landed case is broken (revert-proven).
- Six stranded-intent variants came out of this one path in pass
  1, the last three caused by fixing the first three; the
  reviewer's brief for this task repeats the design review's Q1
  discipline: enumerate every path into and out of `dispatching`
  after the change.

**Boundaries.** `internal/outbox` only, plus its tests. No schema
change; `disposition` is an in-process type, not a column.

---

## Task 9: The perf seeder at realistic weight, and re-measurement

**Requirements:** QA-2, QA-3, QA-5; the corpus specification in
SQLite driver audit section 4.1.

**Outcome.** `storetest`'s seeder produces the audit's corpus:
100k messages, lognormal bodies (median 4 KB, mean ~8 KB, p90
22 KB, p99 180 KB, cap 2 MB), mail-shaped text rather than random
bytes, `search_text` of 200 to 800 words derived from the body,
threads with realistic fan-out, 5,000 events with ~50,000
occurrences, fixed seed, and a recorded corpus fingerprint (file
size, `page_count`, `freelist_count`, FTS index size). QA-2 and
QA-3 are then re-measured on a quiet machine by the orchestrator,
and the EXPLAIN goldens are regenerated against a corpus that
carries `sqlite_stat1`, closing the recorded Major that they
currently prove usability against an empty database rather than
plan choice at the QA envelope.

**Acceptance criteria.**
- Seeded store size lands near the audit's ~1.0 to 1.3 GB
  envelope; the fingerprint is committed with the harness.
- QA-2 re-measured against its real gate (p95 < 25 ms,
  p99 < 40 ms) and the numbers recorded in the task report; a
  miss is a finding for the gate, not a number to smooth.
- QA-5's two criteria are now exercisable and exercised: store
  size at or under 1.6x retained body bytes, and peak RSS against
  250 MB with one writer and four readers.
- The goldens' regeneration is driver-aware per audit T2: they
  are asserted against the driver poplar ships, not the distro
  CLI.

**Boundaries.** The seeder must not require the private corpus
(the pass 1 brief's rule stands); amplification from generated
text is the mechanism. `queryMailboxList`, superseded and
caller-less per the pass 1 ledger, is deleted with its golden
rather than re-pinned.

---

## Task 10: The SQLite driver decision

**Requirements:** SQLite driver audit sections 4 and 4.5; the
owner's rule that nothing inherited is assumed correct.

**Outcome.** The audit's benchmark runs as designed: a
driver-parameterized fork of the QA harness in the scratchpad
(not the repo), `driver_modernc` versus `driver_ncruces` build
tags, the task 9 corpus, operations R1 through M7, fidelity
checks T0 through T4 (T4 is the QA-6 kill harness unmodified
under each driver), five alternating repetitions, quiet machine,
orchestrator-run. The decision applies section 4.5's criteria as
written. The outcome lands as a revision block on ADR-0001 either
way, recording the numbers and the ruling. If ncruces wins, a
follow-on migration lands in this same task's scope: swap the
driver, re-run QA-2/3, regenerate the EXPLAIN goldens per driver,
and re-run the kill harness; if modernc holds, the ADR revision
records why and the T0 result.

**Acceptance criteria.**
- All four section 4.5 ranking-change conditions have a measured
  answer in the report; none is argued from reading.
- T0 (restoring modernc's deleted Tcl harness at 3.53.3) either
  reports an error count against the 28-of-839,686 baseline or
  reports that the harness is rotted, which section 4.5 treats as
  evidence in its own right.
- Raw sample vectors and the environment block are preserved in
  the scratchpad artifacts and summarized in the report.
- A result the criteria do not decide (both drivers fail a gate,
  or the evidence splits) is presented to Geoff rather than
  ruled.

**Boundaries.** The benchmark harness never enters the repo; only
the decision, the ADR revision, and any migration do. No other
implementer runs during measurement windows.

---

## Task 11: Recorded carry-forward findings

**Requirements:** honesty about pass 1's ledger; ADR-0003's 50 ms
transaction ceiling.

**Outcome.** The recorded, unfixed findings from pass 1 are
dispositioned one by one. The two persisted Majors:
`fullResync` (`internal/sync/resync.go:22`) holds one unbounded
transaction against ADR-0003's 50 ms cap and is rewritten to
chunk through the writer's bulk lane; the EXPLAIN-golden Major is
closed by task 9. Beyond those, every `minor (deferred)` and
`OPEN at cap` line in
`.superpowers/sdd/2026-07-27-pass-1-foundation/progress.md` is
harvested into a checklist; each entry is either fixed or
declined with a one-line reason in the task report. The known
cluster includes the non-atomic `MarkCleanShutdown`/pid writes,
`NeedsIntegrityCheck` mutating as a named predicate, the
swallowed quarantine renames, `RebuildIndex`'s lost error
provenance, the duplicated test helpers, and the startup-trace
encoder bypassing slog.

**Acceptance criteria.**
- `fullResync` respects the ceiling under a test that measures
  transaction scope, not wall clock, and fails on revert.
- The disposition checklist in the task report covers 100% of the
  harvested lines; "declined" always carries a reason.

**Boundaries.** This task fixes recorded findings; it does not
open new review fronts. Anything discovered en route that is not
on the ledger is filed for the fan-out, not fixed silently.

---

## Task 12: Pass close: consolidation, fan-out, demonstration

**Requirements:** the phase-end ritual in the STATUS; the pass
gate.

**Outcome.** Consolidation (the in-repo `simplify` skill over the
pass's diff), then the four-lens pass-end reviewer fan-out (spec
compliance, silent failures, code quality, design conformance),
with two changes learned from pass 1: every lens's findings are
persisted to the sdd directory before any fix round begins, and a
lens lost to a StructuredOutput retry cap is re-run rather than
silently absorbed (three agents died silently in pass 1,
including the silent-failure lens on its first run). `deadcode`
runs as a ritual step and its report is read in the close
session.
The STATUS is updated per the ritual (outcomes block, next
starter prompt, roadmap line), the plan archives, and the pass
closes at the gate: poplar running against the live Fastmail
account, Geoff watching the store fill.

**Acceptance criteria.**
- `make check` and `make conformance` green at the close commit.
- `deadcode` output enumerated in the outcomes block: zero
  unexplained unreachable functions in engine paths.
- Fan-out findings persisted under
  `.superpowers/sdd/2026-07-28-pass-1b-integration-hardening/`.
- QA-2/QA-3 numbers at realistic corpus, the driver ruling, and
  the JT coverage ledger all appear in the outcomes block.
- The demonstration runs live for Geoff. That is the gate.
