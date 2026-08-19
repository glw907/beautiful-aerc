# ADR-0001: modernc.org/sqlite is the store engine

Date 2026-07-27. Status: accepted (Phase 4). Survey:
`docs/poplar/research/2026-07-27-phase4-library-survey.md`.

## Context

C1 requires a local store behind every interactive read; C3 requires
`CGO_ENABLED=0`; SR-1 requires a full-text index transactionally
consistent with message mutations; QA-6 requires surviving SIGKILL
at arbitrary points; QA-2/3 set latency gates at a 100k-message
envelope.

## Decision

The store is SQLite through modernc.org/sqlite (pure-Go transpile,
FTS5/WAL/JSON1 compiled in), one database file per account, WAL
mode, with a single write connection owned by one writer goroutine
and a separate read pool. Full-text search is FTS5 with
external-content tables. Migrations are a hand-rolled
`schema_version` runner over `go:embed` SQL files.

## Alternatives considered

- **ncruces/go-sqlite3** (wasm): faster in some published read
  benchmarks, but its FTS5 module is a separate pre-1.0 package
  with near-zero adoption, the runtime just migrated
  wazero→wasm2go, and an open WAL corruption bug (Windows) is
  disqualifying evidence against the QA-6 bar today. Re-evaluate
  at a future major.
- **mattn/go-sqlite3**: cgo; C3 excludes it outright.
- **KV store (bbolt/pebble) + bleve**: hand-builds the
  transactional store-index consistency FTS5 provides, doubles the
  stores to keep crash-consistent, and only pays off at 10-100x
  the envelope.
- **Migration frameworks (goose, golang-migrate)**: fine tools
  aimed at server fleets; the embedded runner is under a hundred
  lines and keeps a dependency out of the trust base.

## Consequences

The concurrency discipline (ADR-0003) is load-bearing: modernc
connections are full-mutex, so reads must never queue behind the
writer. The measurement spike validates the discipline at the
envelope; its relief valves (statement caching, and the
denormalized `search_text` column keeping hot reads off the body
table) are pre-planned. FTS5 is derived state and rebuildable, so
index corruption is a repair path, never data loss.

## Revision 2 (2026-07-27, build boundary)

Two clauses of the Decision above are superseded. Read them
through this block; the Decision text is left intact as the
record of what was decided when.

**One store file holds every account.** The Decision says "one
database file per account". Technical design section 3 settled it
the other way, and ADR-0002's revision 2 agrees, so the Decision
has been wrong since the design landed. One database is what
makes a cross-account view a query rather than a merge, keeps the
FTS5 index single and transactionally consistent with the
mutations it indexes, and holds ADR-0003's discipline to one
writer rather than one per account. Multi-account is the named
first post-v1 priority, so the schema carries `account_id` from
the first migration even though v1 ships one account.

The cost is that a corrupt file takes every account with it.
SY-8's failure tests cover that path, and FTS5 stays derived
state that rebuilds without data loss.

**FTS5 uses a single content source, not external-content
tables.** The Decision's "external-content tables" is retracted by
ADR-0002 revision 2, which found the two-table external-content
shape not constructible as specified. `message_fts` indexes
`message(subject, search_text)` from one source, maintained in the
same transaction as the message write.

## Revision 3 (2026-08-19, driver decision)

The Decision's engine choice is superseded. **poplar's store is now
github.com/ncruces/go-sqlite3 v0.35.3**, a pure-Go WASM build run
through wazero, keeping C3's `CGO_ENABLED=0` constraint intact. The
Alternatives section's ncruces entry, which named an open WAL
corruption bug and a pre-1.0 FTS5 module as disqualifying, is
superseded by this revision: pass 1c's driver audit (section 4.5)
re-measured both drivers against the same 100,000-message envelope
and neither defect reproduced. The ruling applies audit 4.5 as
written: every one of ncruces's four disqualifying conditions was
measured and none fired, and modernc's own outright-win condition on
peak RSS did not fire either.

**The QA-6 kill harness found no corruption on either driver.** 600
SIGKILL trials per driver (200 kill points across 3 seeds), run
against both the incumbent and the ncruces swap, reported zero
corruption and zero failed integrity check throughout.

**T1 through T3 turned up one divergence, and it runs against
modernc, not ncruces.** T1's pragma read-back found modernc's
separate-`_pragma`-parameter DSN form silently drops `page_size`
(reverting to SQLite's 4096 default): the driver re-sorts separate
parameters alphabetically, which runs `journal_mode` ahead of
`page_size`, and both are fixed the instant a file enters WAL mode.
poplar's own DSN builder (`store/dsn.go`) already used the
single-script form that avoids this defect, and that form carries
forward unchanged under ncruces, which holds the correct pragmas
under either DSN spelling. T2's EXPLAIN QUERY PLAN showed zero
difference between the two drivers across all eight queries probed,
matching this repository's own committed goldens exactly. T3's
identical-statement-order replay produced byte-identical database
files across modernc, ncruces, and the stock sqlite3 CLI (3.50.1).

**Peak RSS clears QA-5's 250MB ceiling by roughly 7x on both
drivers**, at the ~2.25GB corpus scale and across all four
checkpoints (pool warm-up, concurrent backfill, immediately after an
FTS5 rebuild, and a real 10-minute idle wait): modernc peaks at
28.6MB, ncruces at 35.3MB. Neither the ncruces-disqualifying
condition nor modernc's outright-win condition fires here; both
drivers are comfortably under budget.

**M2's TRUNCATE checkpoint under a live reader is the sharpest
divergence in the whole dataset, and it sits on the incumbent's own
arm.** Under a live reader snapshot, modernc's TRUNCATE checkpoint
returns `busy=1` on 100 of 100 samples, at 49.8-52.2ms (median
51.10ms), essentially the full 50ms `busy_timeout` on every sample.
ncruces's TRUNCATE checkpoint under the identical setup returns
`busy=0` on 100 of 100 samples, completing in single-digit-to-low-
hundreds of microseconds. This is modernc against its own timeout on
its own drained WAL, not a comparative regression measured against
ncruces, and it is flagged here as a `checkpoint.go` policy
follow-up independent of which driver won.

**T0, modernc's own Tcl conformance suite, is rotted rather than
merely unrun.** `TestTclTest`, its generator, and the pregenerated
fixtures were deleted upstream in modernc.org/sqlite v1.29.0
(2024-02); there is no public rebuild path, and the dependency a
rebuild would need, modernc.org/tcl@v1.15.3, fails to compile
against modernc's own current libc pin. Full evidence is in the
driver-decision dispatch's T0 report (`task-4-t0-report.md`). Audit
4.5 treats a rotted suite as weakening the incumbent, not as neutral.

**Latency is non-decisive everywhere it was measured.** R10's QA-2
composite budget (25ms p95 / 40ms p99) holds on both drivers by a
30-70x margin, quiescent and under concurrent write. R7's length-5
FTS5 prefix probe blows its own budget by 10-20x on both drivers
alike, a shared miss rather than a driver difference, which routes
to the QA gate as a schema question (widening the prefix index
declaration) rather than to this decision.

Three measurement caveats carry forward so a future reader does not
mistake this data for a clean benchmark. The CPU governor drifted off
`performance` for 4 of the 5 alternating repetitions (only repetition
1, both drivers, ran under `performance`; repetitions 2-5 ran under
`powersave`); the drift is paired per repetition, so the
modernc-versus-ncruces ratios above hold, but every absolute latency
figure in the underlying data over-weights powersave conditions 4-to-1
against the audit's own preflight intent. R7's length-5 class ran at
100 iterations, not the audit's specified 5,000, an orchestrator
ruling made after the realistic corpus put a single length-5 query at
0.6-1s (roughly 80 minutes per repetition at full count on this one
non-decisive operation); 100 samples still resolve p95/p99, and
lengths 2-4 stayed at 5,000. R5's body BLOB read shows a roughly 2x
bytes-per-op gap (modernc ~29.9KB versus ncruces ~15.3KB for the same
read) that this data does not explain; it favors ncruces, it is not
gating, and it is worth a targeted look if R5 ever becomes gating.

`dbstat` is unavailable on ncruces's default build, which affects
only test and perf tooling, not production store code. The perf
corpus fingerprint's FTS-index-size field (`storetest/corpus.go`)
now sums `message_fts_data`'s stored block bytes rather than
`dbstat`'s page-aligned accounting; the two committed fingerprints
(`storetest/testdata/perf-corpus-fingerprint-10000.txt` and
`-100000.txt`) were regenerated against the unchanged corpus bytes,
and every other field held identical to the modernc-seeded values.

Every raw artifact behind this revision (all ten JSON repetitions, the
T0-T4 reports, and the section 4.5 analysis) lives in the driver
decision dispatch's sdd workspace snapshot,
`2026-08-18-pass-1c-measurement-and-driver-decision/`. The scratchpad
harness module and its results directory are session-scoped and are
not preserved past the pass.
