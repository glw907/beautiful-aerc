# Pass 1c: Measurement and the Driver Decision

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development to implement this plan
> task-by-task. Tasks use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close pass 1b's split remainder: disposition pass 1's
deferred findings, land the reconnect backoff ruled in at the 1b gate,
re-measure QA-2/QA-3/QA-5 against a realistic corpus, and decide the
SQLite driver by the audit's benchmark.

**Architecture:** No new subsystems. This pass hardens and measures
what passes 1 and 1b built: the store, the sync engine's push loop,
the perf harness, and the driver underneath all of it.

**Tech Stack:** Go 1.26, modernc.org/sqlite (incumbent) vs
ncruces/go-sqlite3 (challenger), SQLite FTS5, Stalwart v0.16.15 via
podman for conformance.

**Specs and inputs:**
- Pass cursor: `docs/superpowers/specs/poplar-refounding-STATUS.md`
  (pass 1c section; this plan implements it).
- Task briefs, carried from pass 1b's ratified split, all under
  `.superpowers/sdd/2026-07-28-pass-1b-integration-hardening/`:
  `task-11b-brief.md` + `task-11b-harvest.md` (task 1),
  `task-9-brief.md` (task 3), `task-10-brief.md` (task 4), with
  `progress.md` as the pass 1b record.
- Benchmark design: `docs/poplar/research/2026-07-28-sqlite-driver-audit.md`
  sections 4 and 4.5. The decision criteria are applied as written.
- BACKLOG #65 (task 2): the verified fix shape is in the entry.
- Requirements are at revision 4. ADR revision blocks override ADR
  bodies. The RFC obligations map
  (`docs/poplar/research/2026-07-28-jmap-rfc-obligations-map.md`) is a
  standing input, not a work item.

## Global Constraints

- One `poplar-implementer` per task. `poplar-reviewer` and
  `poplar-go-reviewer` run in parallel on each diff. Reviewers prove
  revert-sensitivity by experiment and attack each round's new guards
  first (pass 1b's signature defect arrived inside the guard written
  to close the previous finding, repeatedly).
- Tasks 3b and 4 are measurement: quiet machine, NO implementer or
  other agent dispatched during a measurement window.
- Any reported gate result is verified from a captured exit code
  (`$?`), never from reading a piped tail. One false green-gate claim
  in 1b came from exactly that.
- Never set `run_in_background` on any Bash call. Unique names for
  every scratch file, container, and worktree; Stalwart ports
  19081-19083 have been used before.
- `make conformance` runs on podman (docker is not installed); the
  image is `docker.io/stalwartlabs/stalwart:v0.16.15`.
- Commits land on master: imperative mood, specific files,
  `Co-Authored-By: Claude <noreply@anthropic.com>`. Implementers do
  not push; the orchestrator pushes.
- Geoff is at the pass gate only. A result the written criteria do
  not decide is presented at the gate, never ruled by the
  orchestrator.
- The `go-conventions` skill binds every Go change; `make check` is
  the floor for every task.
- This pass exists because 1b burst its scope. It is on notice: no
  scope joins a task after dispatch, and a second task split inside
  this pass triggers a split proposal to Geoff.

## Task order and why

1 → 2 → 3a → 3b → 4 → 5. Tasks 1, 2, and 3a are implementer work and
run before any measurement so the machine is quiet afterward. Task 3b
(measurement) needs 3a's corpus; task 4 needs the same corpus and its
own quiet windows; the close (5) needs everything. Tasks 1 and 2 are
independent of everything else and of each other but run serially per
the one-executor rule.

---

### Task 1: Deferred-findings harvest and the conformance rows (carried 11b)

**Deliverables (3):** dispositions for all 135 harvest rows, fixes for
the seven named clusters, the two conformance-suite corrections.

**Brief:** `task-11b-brief.md` is the authoritative dispatch text;
`task-11b-harvest.md` is the input triage (79 FIX / 33 ALREADY CLOSED /
18 DECLINE / 31 NEEDS READING). The scope ruling in the brief holds:
fix only the seven clusters (`mark-clean-shutdown-and-pid-atomicity`,
`needs-integrity-check-mutating-predicate`,
`swallowed-quarantine-renames`, `rebuild-index-error-provenance`,
`duplicated-test-helpers`, `startup-trace-bypasses-slog`,
`conformance`); every other row gets a written decision with a reason,
not a fix. The standing reason: pass 1b fixed the clusters its brief
named, and the remainder is carried as a standing input to the pass
that next touches the file.

**Files:**
- Modify: the cluster files the harvest names (chiefly
  `internal/store/recovery.go`, `internal/store/fts.go`,
  `internal/store/recovery_test.go`, `internal/platform/lock.go`,
  `cmd/poplar/main.go`, `jmap/conformance_dv_test.go`).
- Update: `task-11b-harvest.md` dispositions (gitignored; the report
  states it was updated).

**Steps:**
- [ ] Verify every ALREADY CLOSED claim by check (grep or read), not
  inheritance; a claim that fails the check is a finding.
- [ ] Resolve all 31 NEEDS READING rows into real dispositions.
- [ ] Fix the seven clusters test-first where a behavior changes.
- [ ] DV-11: correct the RFC citation to 8620 section 5.4 and demote
  the `existingId` check to a recorded, server-conditional divergence
  (Stalwart sends it; Fastmail does not; no RFC mandates it on a
  `Mailbox/set` create refusal).
- [ ] Add the DV row for the sibling-uniqueness normalization
  divergence (Stalwart case-insensitive and trimming; Fastmail
  byte-exact), asserting poplar's mechanism, not one server's answer.
- [ ] If the two rows' divergences can be recorded in a way a green
  run does not hide, do it and say why it generalizes; if that
  outgrows two rows, say so and leave it.

**Acceptance criteria:**
- Every one of the 135 rows carries a decision with a reason; none is
  left NEEDS READING.
- The seven clusters are fixed or shown closed by a check the
  implementer ran.
- `make check`, `make jmap-boundary`, `make tagged-vet`,
  `make conformance`, and an uncached `go test -race` on touched
  packages all pass by captured exit code.
- One commit on master. Anything discovered off-ledger is filed in
  the harvest document, never fixed silently.

**Boundaries:** `fullResync` and the two `internal/store` `-race`
tests were settled by 11a; do not rework them. No new review fronts.

---

### Task 2: BACKLOG #65, the silent-stop reconnect backoff

**Deliverables (1):** the escalation fix with its rewritten pinning
test.

**Why now:** ruled in at the 1b gate (Geoff, 2026-08-18). A push
stream that opens cleanly and dies at once reopens at the zero-delay
floor forever (~4 Listen calls/s measured); the pinning test
deliberately asserts that floor, so the test rewrite is part of the
ruling, not collateral.

**Files:**
- Modify: `internal/sync/push.go` (`RunPush`, `pushState`),
  `internal/sync/push_test.go`
  (`TestRunPushDoesNotEscalateOnAnUnexplainedStop`), `BACKLOG.md`
  (mark #65 closed).

**Interfaces:** no exported surface changes; `pushState` is package
internal.

**Steps:**
- [ ] Rewrite the pinning test first to assert the new escalation
  curve: a silent stop advances the backoff attempt; a delivered
  notification and `proved()` reset it. Run it, verify it fails
  against current behavior.
- [ ] Apply the verified fix shape from the backlog entry:
  `pushState.stopped` advances `attempt` (not only `fail`); the tail
  `sleepBackoff` call in `RunPush` takes `push.attempt` instead of a
  fixed 0; both reset on delivery and in `proved()`.
- [ ] Confirm the test passes and is revert-sensitive: reverting the
  push.go change alone must fail it.
- [ ] Mark #65 closed in BACKLOG.md with the landing commit.

**Acceptance criteria:**
- The reconnect rate under a silently-dying stream rides the backoff
  schedule (the scratch experiment measured ~0.1/s against ~4/s).
- A healthy stream's behavior is unchanged: delivery resets the
  curve; the poll fallback timing is untouched.
- `make check` and an uncached `go test -race ./internal/sync/...`
  pass by captured exit code. One commit on master.

---

### Task 3: The perf seeder at realistic weight, and re-measurement (carried 9)

**Deliverables (3):** the audit-spec seeder with committed fingerprint,
regenerated driver-aware EXPLAIN goldens plus the
TestInteractivePreemption rewrite, and the QA-2/QA-3/QA-5 numbers.

**Brief:** `task-9-brief.md`. Split into an implementer half (3a) and
an orchestrator measurement half (3b).

#### 3a (implementer)

**Files:**
- Modify: `internal/store/storetest/perf.go` (the seeder),
  `internal/store/queries.go` and `internal/store/queries_test.go`
  (delete superseded `queryMailboxList` and its golden),
  `internal/store/testdata/` (EXPLAIN goldens),
  `internal/store/writer_test.go` (`TestInteractivePreemption`).

**Steps:**
- [ ] Rebuild the seeder to the audit section 4.1 corpus spec:
  100k messages, lognormal bodies (median 4 KB, mean ~8 KB, p90
  22 KB, p99 180 KB, cap 2 MB), mail-shaped text from the existing
  vocabulary (never random bytes), `search_text` of 200-800 words
  derived from the body, subjects 4-12 words with the
  common/medium/rare mix, threads in groups of 1-30 skewed small,
  1 account / 4 mailboxes with the 80/20 inbox skew, 5,000 events
  with ~50,000 occurrences, fixed seed. Amplification from generated
  text only; the private corpus stays out (pass 1 rule).
- [ ] Record the corpus fingerprint (file size, `page_count`,
  `freelist_count`, FTS5 index size via dbstat) and commit it with
  the harness.
- [ ] Delete `queryMailboxList` and its golden rather than re-pinning
  it (superseded, caller-less per the pass 1 ledger).
- [ ] Regenerate the EXPLAIN goldens against a corpus carrying
  `sqlite_stat1`, asserted against the driver poplar ships (audit
  T2), closing the recorded Major that they currently prove
  usability against an empty database.
- [ ] Rewrite `TestInteractivePreemption` against transaction
  ordering, not elapsed time (it is load-sensitive at 3/20 flaky on
  the 1b base).
- [ ] `make check` green by captured exit code; one commit on master.

**Acceptance criteria (3a):**
- Seeded store size lands near the audit's ~1.0-1.3 GB envelope and
  the fingerprint is committed.
- Goldens are driver-aware and stat-carrying; the flake rewrite is
  deterministic under load (demonstrated by a repeated uncached run).

#### 3b (orchestrator, quiet machine, no agents dispatched)

**Steps:**
- [ ] Re-measure QA-2 against its real gate (p95 < 25 ms,
  p99 < 40 ms), quiescent and under concurrent write, on the new
  corpus.
- [ ] Re-measure QA-3 against the spike baseline (0.9-4.5 ms p95).
- [ ] Exercise both QA-5 criteria: store size at or under 1.6x
  retained body bytes, and peak RSS against 250 MB with one writer
  and four readers.
- [ ] Record every number in the task report and the pass outcomes
  block. A miss is a finding for the gate, not a number to smooth.

---

### Task 4: The SQLite driver decision (carried 10)

**Deliverables (2, plus a conditional third):** the benchmark report
with raw vectors, the ADR-0001 revision block recording the ruling,
and (only if ncruces wins) the migration.

**Brief:** `task-10-brief.md`; the harness and criteria are audit
sections 4.0-4.5, applied as written.

**Steps:**
- [ ] Dispatch an implementer to build the driver-parameterized fork
  of the QA harness in the scratchpad (never the repo):
  `driver_modernc` vs `driver_ncruces` build tags, DSN builder and
  FTS5 registration as the only per-driver differences, reusing
  `storetest.Measure`/`Percentile`/`WriteBaseline` and the QA-2
  scripted mix. Seed once under one driver; run T1 to verify the
  other reads it identically.
- [ ] Orchestrator runs the suite on a quiet machine: operations
  R1-R10, W1-W4, M1-M7, fidelity checks T0-T4 (T4 is the QA-6 kill
  harness unmodified under each driver), five alternating
  repetitions (A/B/A/B/A/B/A/B/A/B), governor pinned to performance,
  page caches dropped before cold measurements. No other work runs
  during measurement windows.
- [ ] T0: restore modernc's deleted Tcl harness at SQLite 3.53.3 and
  report an error count against the 28-of-839,686 baseline, or
  report the harness rotted (which section 4.5 treats as evidence).
- [ ] Record everything section 4.3 lists: latency percentiles with
  raw sample vectors, allocations, the three-way memory split with
  the four RSS checkpoints, storage, build costs, and the
  environment block. Artifacts stay in the scratchpad; the report
  summarizes them.
- [ ] Apply section 4.5's criteria as written. All four
  ranking-change conditions get a measured answer; none is argued
  from reading. Latency alone moves nothing unless a driver misses
  QA-2's gate where the other holds.
- [ ] Land the ADR-0001 revision block recording the numbers and the
  ruling, whichever way it goes.
- [ ] If ncruces wins: dispatch the migration in this task's scope:
  swap the driver, re-run QA-2/3, regenerate the EXPLAIN goldens per
  driver, re-run the kill harness. If modernc holds: the revision
  block records why and the T0 result.
- [ ] If both drivers fail a gate or the evidence splits, present it
  to Geoff at the pass gate rather than ruling.

**Boundaries:** the benchmark harness never enters the repo; only the
decision, the ADR revision, and any migration do.

---

### Task 5: Pass close and consolidation

**Deliverables (3):** the consolidated diff, the reviewer fan-out
verdicts folded, and the updated STATUS with the pass 2 starter.

**Steps:**
- [ ] Run the in-repo `simplify` skill over the pass diff; apply what
  survives review.
- [ ] Reviewer fan-out over the pass diff via parallel Agent
  dispatches (Opus lenses per the 1b shape); reviewers attack the
  pass's new guards first and prove revert-sensitivity by
  experiment. Fold confirmed findings; a workflow-scale fan-out is
  suggested to Geoff at the gate only if the diff warrants it.
- [ ] Run `deadcode ./...`; record the count and per-entry
  justifications in the outcomes block.
- [ ] Update `poplar-refounding-STATUS.md`: pass 1c outcomes block
  (including the QA-2/QA-3/QA-5 numbers and the driver ruling),
  roadmap line, and the pass 2 (design language and shell) starter
  prompt. Pass 2 inherits the routed items already recorded in the
  STATUS (ADR-0005 self-echo suppression; the wireframe ritual
  applies as a screen pass).
- [ ] Delete `.superpowers/sdd/2026-07-28-pass-1b-integration-hardening/`
  (its briefs are consumed; this plan and the STATUS carry the
  record).
- [ ] Archive this plan per repo convention, commit, push.
- [ ] Pass gate with Geoff: the numbers, the driver ruling (or the
  undecided evidence), the budget score (tokens and interaction
  points), and anything the criteria did not decide.
