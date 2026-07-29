# poplar SQLite driver audit

Date: 2026-07-28. Method: source reading of local clones, issue trackers, release
histories, SQLite's own documentation, plus direct reads of poplar's store package.
Every claim carries a label.

- **[F] field-supported.** Verified against a file, a commit, an issue number, or a
  published document, cited inline.
- **[J] judgment.** My inference from field-supported facts. Argue with it freely.

Clones inspected under
`/tmp/claude-1000/-home-glw907-Projects-poplar/47f73ae1-3feb-488d-a9f3-147d090563bd/scratchpad/research/`:
`go-sqlite3` (ncruces, HEAD `1c3fb78`, 2026-07-21), `go-sqlite3-wasm` (HEAD `ea020b9`,
2026-07-05), `modernc-sqlite` (HEAD `cfb9734`, 2026-07-20), `mattn-go-sqlite3`,
`go-sqlite` (zombiezen), `sqlite` (crawshaw), `go-sqlite-bench`.

---

## 1. Ranking

### 1. `github.com/ncruces/go-sqlite3` — the recommendation

Wins on the axis that predicts future correctness better than any other, which is
independent, automated, per-commit verification of the exact layer poplar leans on
hardest. It matches or beats the incumbent everywhere else poplar cares, and it is the
only candidate whose API removes rather than documents the DSN defect poplar already
paid for.

**Verification posture [F].** `.github/workflows/test.yml` runs, on every push and PR:
`macos-latest`, `ubuntu-latest`, `windows-latest` full suites; the same suite again
under `-tags sqlite3_flock` and `-tags sqlite3_dotlk` (all three locking modes);
cross-built and VM-executed suites on FreeBSD 15.1 (amd64 and arm64), NetBSD 10.1 (amd64
and arm64), OpenBSD 7.9, illumos r151058, DragonFly BSD, Solaris; QEMU runs on 386,
loong64, riscv64, ppc64le, s390x; native runs on linux/arm64, linux/arm (GOARM=7),
macos-15-intel, windows-11-arm, and ubuntu riscv/s390x/ppc64le runners; a wasip1 run
under wasmtime; `gofmt`, `go mod tidy`, `go vet`, and a coverage report. Additionally
`vfs/tests/mptest/mptest_test.go` runs SQLite's **own** multi-process stress harness
(`mptest`) against the Go VFS, with 15 test functions covering `config01`, `config02`,
`crash01`, and `multiwrite01`, each in plain, memdb, MVCC, WAL, and encrypted-VFS
variants (`Test_crash01_wal`, `Test_multiwrite01_wal`).

**Real SQLite [F].** `go-sqlite3-wasm/build/download.sh` fetches
`https://sqlite.org/2026/sqlite-autoconf-3530300.tar.gz` under a pinned SHA3-256 hash and
pulls `ext/misc/*.c` from `github.com/sqlite/sqlite/raw/version-3.53.3`. Three small
patches apply before compilation, and only one is semantic: `vfs_find.patch` disables
SQLite's internal VFS registry so the Go VFS can replace it. `busy_timeout.patch` makes
the busy handler context-cancelable. `export.patch` is linkage visibility only.

**Runtime [F].** The driver no longer uses wazero. Since v0.33.0 (2026-03-21, PR #362) the
WASM binary is machine-translated to plain Go by `wasm2go`, checked into
`go-sqlite3-wasm` as `sqlite3.go`. The README title is literally "Go bindings to SQLite
using wasm2go", and no `.go` file in the tree references wazero. There is no interpreter
and no JIT at runtime, and poplar's build never invokes a WASM toolchain. `go.mod`'s only
direct non-ncruces dependency is `golang.org/x/sys`.

**API fit to poplar's design [F].**

- `_pragma` order is the caller's order. `conn.go:125` iterates
  `query["_pragma"]` in encounter order and concatenates one `PRAGMA x;` per value into a
  single `sqlite3_exec` script. `url.ParseQuery` preserves repeated-key order, so
  `page_size` and `auto_vacuum` land before `journal_mode(wal)` when written that way.
  poplar's existing single-script spelling also works unchanged.
- A bad pragma fails the open cleanly. `conn.go:131-139` wraps the error as
  `sqlite3: invalid _pragma` and calls `closeDB`.
- `driver/driver.go:255-268` reads back `PRAGMA query_only` after applying `_pragma` and
  records `conn.readOnly`, which is exactly how poplar spells its read pool
  (`dsn.go:81-83`).
- `config.go:276` exposes `Conn.WALCheckpoint(schema, mode) (nLog, nCkpt int, err error)`,
  a direct binding of `sqlite3_wal_checkpoint_v2`. poplar today builds
  `"PRAGMA wal_checkpoint("+mode+")"` by string concatenation and discards the result row
  (`checkpoint.go:35-38`), so it cannot tell a busy checkpoint from a complete one [F].
- `busy_timeout` retries are context-cancelable and jittered rather than a blind fixed
  backoff, which targets the reader-blocks-checkpoint case `dsn.go:21-31` describes.
- Incremental BLOB I/O is exposed as `io.Reader`/`Writer`/`Seeker`/`WriterTo`
  (`blob.go`), relevant to the `body.content` column.

**FTS5 [F].** Real `fts5.c`, shipped as `ext/fts5` and linked through SQLite's genuine
dynamic-extension ABI. One line wires it for every connection the process opens:
`sqlite3.AutoExtension(fts5.Register)` (`ext.go:21`). poplar's `content='message'`
external-content table, `prefix='2 3'`, `'integrity-check'`, and `'rebuild'` are all
ordinary SQL against unmodified FTS5 internals.

**The real risk, stated plainly [F+J].** The OS interface is a from-scratch Go
reimplementation, and `vfs/README.md` says so in its own words: "Since this is a from
scratch reimplementation, there are naturally some ways it deviates from the original.
It's also not as battle tested as the original." [F] That layer is precisely where
poplar's checkpoint discipline lives. Mitigating facts, all [F]: the default Linux and
macOS path uses OFD locks rather than the POSIX advisory locks SQLite's own `os_unix.c`
calls "broken by design"; WAL shared memory uses real `mmap`, matching SQLite's
documented design; the README declares default builds "compatible with the standard Unix
and Windows SQLite VFSes"; macOS additionally implements WAL-mode blocking locks
(`doc/wal-lock.md`), which bears directly on poplar's TRUNCATE-under-a-reader case; and
`mptest`'s crash and multiwrite scripts run against this layer on every commit. My
judgment [J] is that a 5k-line hand-written layer under SQLite's own crash harness on 15
platforms is a better-evidenced bet than a 250k-line machine translation under no public
harness at all, which is the incumbent's actual position (section 3).

**Correction to ADR-0001 [F].** ADR-0001 rejects ncruces on three grounds. One is now
false: "an open WAL corruption bug (Windows) is disqualifying evidence against the QA-6
bar today" refers to issue #404, which was root-caused to the Windows shared-memory
copying scheme and fixed in v0.35.2 on 2026-07-06, three days after the report (PR #405,
mapping `-shm` directly via `VirtualAlloc2`/`MapViewOfFile3` as the Unix build already
did). The Unix path was never affected, and the trigger required a multi-connection
concurrent-**write** pool that poplar's single-writer design cannot produce. One is
stale-ish: "the runtime just migrated wazero→wasm2go" was four days old when the ADR was
written and is now four months old with the CI matrix above behind it. One stands as
written: the FTS5 subpackage has low independent adoption [J], though the mechanism is
SQLite's own extension ABI over unmodified `fts5.c` [F].

### 2. `modernc.org/sqlite` — the incumbent, second on merit

Not disqualified. Second because its correctness evidence is three years old and its API
has a demonstrated, still-unfixed defect in poplar's own configuration path.

It keeps one advantage nothing else in the pure-Go field can claim [F]: it transpiles
SQLite's **actual** `os_unix.c`. I verified this by grepping the generated tree, where
`unixShmMap`, `unixOpenSharedMemory`, `F_SETLK`, and `os_unix` all appear across
`lib/sqlite_g_*.go` and the per-platform files. Its locking and WAL-index semantics are
SQLite's by construction, not by reimplementation-plus-testing. For a design as
dependent on WAL internals as poplar's (autocheckpoint off, manual PASSIVE and TRUNCATE,
`journal_size_limit`, bounded `incremental_vacuum`, a lowered `busy_timeout` around
TRUNCATE), that is worth real weight [J], and it is the single fact that could reverse
this ranking if the benchmark's crash test goes badly for ncruces.

Everything else is in section 3.

### 3. `zombiezen.com/go/sqlite` — a real contender that fails on freshness

Not a cgo driver [F]: `go.mod` requires `modernc.org/sqlite` and `modernc.org/libc`
directly, `sqlite.go` imports `lib "modernc.org/sqlite/lib"`, and no `import "C"` exists
in the tree. It is crawshaw's low-level `Conn`/`Stmt` API over modernc's engine.

Its shape genuinely fits poplar better than `database/sql` does [J]. `sqlite.OpenConn`
returns one concrete connection, so the writer would *be* an object rather than a pool
constrained to one (`writer.go:83-84` today calls `SetMaxOpenConns(1)`) [F]. Pragmas are
ordinary SQL in a `PrepareConn` callback, so the DSN-reordering bug class has no
mechanism to live in [F].

It loses on the owner's own rule, which counts maintenance only as a predictor of
correctness [F]: `go.mod:13` pins `modernc.org/sqlite v1.37.1`, roughly a year and
several SQLite point releases behind poplar's own `v1.54.0` pin, and the last CHANGELOG
entry is 2025-05-23. Adopting it means either running a year-old SQLite or maintaining a
`replace` directive that puts poplar back on modernc's engine anyway with an extra
unverified layer between [J].

### 4. `mattn/go-sqlite3` — excluded by C3, best of the cgo family

Best-maintained cgo option by a wide margin [F]: pushed 2026-07-29, v1.14.48 tagged
2026-07-13, tracking SQLite 3.53.3, the same version as poplar's modernc pin. Excluded by
C3, and the exclusion is sound (section 4). Two independent demerits even setting C3
aside [F]: FTS5 requires a `-tags sqlite_fts5` build tag, so forgetting it produces a
successful build that fails at runtime with "no such module: fts5"; and it is a
`database/sql` driver, so it inherits the identical `SetMaxOpenConns(1)` writer hack with
no API-shape gain over the incumbent.

### 5. `crawshaw.io/sqlite` — abandoned

Last substantive commit 2022-06-18, 37 open issues [F]. Not viable regardless of the
API-shape merit zombiezen inherited from it.

### Confidence, and what reading cannot settle

**Confidence in first place: moderate.** [J] The verification-posture gap is large,
one-directional, and verifiable by anyone in five minutes, which is the strongest single
fact in this audit. The counterweight is that ncruces's advantage is *testing* of a
reimplemented layer while modernc's is *provenance* of the real one, and provenance and
testing are not commensurable by argument. Four things need measurement, not reading:

1. **Crash correctness of ncruces's VFS under poplar's exact checkpoint discipline.**
   `mptest` is strong evidence, and it is not poplar's schema, poplar's FTS5 triggers, or
   poplar's manual-checkpoint policy. QA-6 already specifies the harness that settles it.
2. **Whole-process RSS against QA-5's 250 MB ceiling at a realistic corpus.** ncruces
   gives each connection its own WASM linear memory, defaulting to a 256 MB ceiling
   (`config.go`, `mem.Max = 4096` pages), reserved via `mmap(PROT_NONE)` and committed
   with `mprotect` only as SQLite grows it, and never released back within a connection's
   lifetime because WASM memory only grows [F]. With one writer and four readers
   (`read.go:15`), the interaction between that and QA-5 is unknown [J]. modernc's
   equivalent working set lives in `modernc.org/libc`'s allocator, off the Go GC heap,
   with its own unmeasured profile [F].
3. **Whether the two drivers produce identical query plans and identical file bytes.**
   Both compile with `SQLITE_ENABLE_STAT4` [F], so poplar's EXPLAIN goldens must be
   regenerated per driver and cannot be authored against the distro `sqlite3` CLI, which
   lacks STAT4 [F].
4. **Whether modernc's transpiled core still passes SQLite's own suite at 3.53.x.**
   Nobody has published a run since June 2023 [F]. This is *checkable*, and checking it is
   the highest-value single experiment available for the incumbent (section 5, task T0).

---

## 2. The case against the incumbent

Made as strongly as the evidence allows, with the incumbency given no weight. poplar
inherited this pin from the archived client at this same version, and the only
poplar-specific evidence about the driver in the tree today is a comment explaining a
workaround for one of its defects (`dsn.go:67-76`) [F].

### 2.1 Its verification apparatus was dismantled, and nothing replaced it

This is the charge that matters, and it is stronger than "the project went quiet."

- `TestTclTest`, the Go entry point that built SQLite's `testfixture` and ran the vendored
  Tcl corpus, no longer exists. What remains at `all_test.go:110` is an orphaned flag
  whose help text describes the deleted function: `oXTags = flag.String("xtags", "",
  "passed to go build of testfixture in TestTclTest")` [F].
- `internal/mptest/`, which wired SQLite's own multi-process concurrency stress harness,
  is gone. The `internal/` directory is empty. `testdata/mptest.c` sits in the tree
  unwired [F].
- There is no CI configuration in the repository. `.github/` contains exactly one file,
  `FUNDING.yml`. There is no `.gitlab-ci.yml` [F]. Whatever the maintainer runs locally,
  no release is gated by anything a user can inspect [J].
- The last commit reporting a Tcl pass rate is `2a8ff5d`, "SQLite 3.42, 28 errors out of
  839686 tests on linux/amd64", dated 2023-06-10 [F]. Since then the project has shipped
  roughly fifteen SQLite version bumps, through 3.53.3 [F].
- The vendored corpus itself never covered FTS5. `testdata/tcl/` holds 1,230 `.test`
  files spanning fts1 through fts4 and contains **zero** `fts5*.test` files [F]. So even
  the 2021-2023 evidence, at its most rigorous, never exercised the module poplar ships
  search on.

Put together [J]: poplar would build a product feature on a 250,000-line machine
translation of C whose only public correctness evidence predates every SQLite release it
now runs, and which never covered the module the feature depends on. Compare that to
section 1's matrix, and the gap is not close.

### 2.2 The DSN defect is a pattern with two incidents and a fix that made it worse

- 2022, GitLab #115: `busy_timeout` silently had no effect unless it appeared first among
  the `_pragma` keys. Users hit spurious `SQLITE_BUSY` under WAL [F].
- 2024, GitLab #198: same class, diagnosed by a contributor, fixed by **sorting** the
  parameters, pinning `busy_timeout` first and putting the rest in lexicographic order
  (`sqlite.go:380-397`, current master) [F].
- That fix is the direct cause of poplar's problem. Lexicographic ordering puts
  `journal_mode` (j) before `page_size` (p), and both `page_size` and `auto_vacuum` are
  fixed the moment a file enters WAL mode, so both silently no-op on a fresh database
  [F, and `dsn.go:67-76` documents it].
- It is still present. I read it at master `cfb9734` (2026-07-20), and the unreleased
  v1.55.0 work does not touch it [F].

The judgment [J]: the defect is not that a maintainer missed an edge case once. It is
that when the ordering problem was diagnosed, the fix chosen imposed the maintainer's
ordering over the caller's, in a code path that runs before any application test can
observe it, when preserving the caller's order was available. The competitor does exactly
that in six lines (`conn.go:125-129`) [F]. That is an API-design judgment poplar has
already paid for once and would keep paying for on every future pragma.

### 2.3 Its one severe wrong-results bug was in the layer the transpile approach adds

GitLab #42 (2021): `?NNN` positional-parameter binding bound every value to the last
argument, corrupting every row of a bulk insert. Root cause was a loop variable escaping
its scope in **Go glue**, not in transpiled SQLite [F]. It was found by a user running
TPC-H, not by the project's tests, and the issue title says so: "tpch revealed bugs the
Tcl tests do not catch" [F]. GitLab #100 (2022) is the same family: `nil []byte` stopped
binding as NULL after a botched empty-blob fix [F].

Both were fixed fast, and #42's fix was verified to byte-identical output against real
SQLite modulo version bytes, which is an excellent bar [F]. The point stands anyway [J]:
the driver's own history says the value-marshalling boundary is where its wrong-results
bugs live, that boundary is exactly what a transpile-plus-`database/sql` approach adds
over the C engine, and the Tcl suite (now unrun) was never the thing that caught them.

### 2.4 Three concurrency issues touching poplar's shape are open and stalled

- #192 (open since 2024-09-13, "waiting for info"): `busy_timeout` not honored under
  `_txlock=immediate`; `BEGIN IMMEDIATE` fails instantly with `SQLITE_BUSY` [F].
- #239 (open since 2025-12-24, stalled on repro): `wal_checkpoint(FULL)` blocks
  indefinitely [F].
- #232 (open since 2025-10-16, unresolved): frequent `SQLITE_BUSY` after migrating from
  mattn, maintainer's theory being that this driver is legitimately more
  contention-prone under that workload [F].

None reproduces poplar's exact shape [J], since poplar is file-backed, single-writer,
default `_txlock`, and checkpoints with TRUNCATE rather than FULL. They are adjacent risk
in the subsystem poplar depends on most, all open, all stalled.

### 2.5 What survives the attack

Two things, both real [F]. Version tracking is fast, usually single digit days behind an
upstream point release, and it is the one area of continuous investment. And it runs
SQLite's actual `os_unix.c`, which nothing else in the pure-Go field does. If the
benchmark's crash test (section 5, T7) finds any corruption under ncruces that modernc
does not produce, that second fact wins the argument outright and the incumbent stays.

---

## 3. Should the cross-compile and single-binary requirements stand?

They were not re-derived when they were written, and they exclude the entire cgo family,
so they deserve the check. Verdict: **C3 stands and should be reworded. The `make check`
darwin cross-compile stands but must stop being credited to QA-7.**

### C3 stands, on evidence C3's own text does not name

C3 reads "One binary, static per platform (`CGO_ENABLED=0`)" [F,
`2026-07-27-poplar-requirements.md:73-80`]. The real, load-bearing failure mode is
narrower and better than the text [F]: a cgo build on Linux dynamically links glibc by
default, which couples the shipped binary to the host's glibc version and produces the
classic "built on my distro, fails on yours" break. Making a cgo Linux binary genuinely
static requires switching the whole toolchain to musl with
`-linkmode external -extldflags -static`, which mattn's own README documents as a
separate recipe requiring `musl-cross` [F]. poplar's `CGO_ENABLED=0` binary needs none of
that and is static by construction [F].

The text should change in one respect [J]: "static per platform" is literally
unachievable on macOS by anyone, cgo or not, because Apple ships no static `libSystem.a`
and every darwin executable links the dynamic one, pure-Go binaries included since raw
syscalls are blocked by the darwin ABI [F]. Recommend rewording to name the actual
constraint, which is no cgo, no non-Go toolchain in the build, and no runtime shared-
library dependency beyond the platform's own libc.

Cost of holding C3, priced honestly [F]: cgo's one clear performance win in
`cvilsmeier/go-sqlite-bench` is bulk and complex insert, where mattn beats modernc by
roughly 2x. poplar's write path is a writer-owned, chunked, batched sync lane whose
latency is not gated by any requirement, while QA-2 and QA-3 gate reads, where the
pure-Go field is competitive and in `lbe/sqlite-read-benchmark` beats cgo outright [F].
So C3 costs poplar nothing it is measured on [J].

### The darwin cross-compile stands, with its rationale corrected

`make check`'s `check-build` target does a compile-only `GOOS=darwin GOARCH=arm64` build
and runs no tests, so it structurally cannot detect a rendering divergence [F,
`Makefile:29-37`]. The Makefile itself annotates it as serving C10, not QA-7 [F]. QA-7's
actual byte-identity signal comes from the native `darwin` job in
`.github/workflows/ci.yml`, whose own comment says exactly that, and that job runs on
real Apple hardware and would work identically with or without cgo [F]. C10 further
states that macOS "builds and passes tests but does not block the v1 gate" [F].

So [J]: keep the cross-compile, because with a pure-Go driver it costs seconds and
catches a `_darwin.go`-shaped break locally instead of on a CI round trip. Correct the
comment and any requirements text that implies it is the QA-7 gate. And do not let it
carry weight in this decision, because both finalists are `CGO_ENABLED=0` and cross-
compile cleanly from Linux today. I verified the ncruces side directly:
`GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./...` succeeds on `go-sqlite3` and
`go-sqlite3-wasm`, including `./driver/...`, `./ext/fts5/...`, and `./vfs/...`, and no
`import "C"` appears anywhere in the tree [F].

---

## 4. Benchmark plan

Purpose: settle the four open questions in section 1, not to produce a general driver
leaderboard. Run on a quiet machine with no other executor in the worktree.

### 4.0 Harness

Build a driver-parameterized fork of poplar's existing QA harness in the scratchpad, not
in the repo. It imports `0001_initial.sql` verbatim, and selects a driver by build tag,
`driver_modernc` or `driver_ncruces`. Two things must differ per driver and nothing else:

- The DSN builder. modernc requires poplar's single-script `_pragma` form. ncruces takes
  either form, and T1 below tests both.
- FTS5 registration. ncruces needs `sqlite3.AutoExtension(fts5.Register)` once before the
  first `Open`.

Reuse, do not reimplement: `storetest.Measure`, `storetest.Percentile`, and
`storetest.WriteBaseline` (`storetest/perf.go:202-253`) already produce the
`n/p50/p95/p99/max` shape this needs, and `perf_qa2_test.go` already encodes QA-2's
60/15/15/10 scripted mix.

### 4.1 Corpus, and the fix the current seeder needs

The current seeder inserts the message's own 20-word `search_text` as the body content
(`storetest/perf.go:156`), roughly 150 to 200 bytes [F]. Real bodies average several KB.
That is a ~30x understatement of the store's dominant table, and it invalidates three
things at once [J]: the reader-open operation in QA-2's mix, QA-5's RSS ceiling, and
QA-5's "store size at or under 1.6x retained body bytes" criterion, which is meaningless
when bodies are smaller than the indexes over them.

Corpus specification, fixed seed, generated once and copied per run:

| Element | Spec |
|---|---|
| Accounts / mailboxes | 1 account, 4 mailboxes, 80/20 inbox skew (matches `perfPickMailbox`) |
| Messages | 100,000, `received_at` spread over 2 years, matching QA-5's envelope |
| Body size distribution | Lognormal, median 4 KB, mean ~8 KB, p90 22 KB, p99 180 KB, hard cap 2 MB. Total body bytes ~800 MB, total file ~1.0 to 1.3 GB |
| Body content | Mail-shaped text and quoted-reply HTML built from the existing vocabulary, never random bytes, so compressibility and tokenization resemble mail |
| `search_text` | 200 to 800 words derived from the body, not 20. This is what makes the FTS5 index realistic, on the order of 150 to 400 MB rather than a few MB |
| `subject` | 4 to 12 words, common/medium/rare mix preserved from `perfRandomText` |
| Calendar | 5,000 events, ~50,000 occurrences, per QA-5 |
| Threads | `thread_key` groups of 1 to 30, skewed small, so `queryThreadAcrossFolders` sees realistic fan-out |

Record the seeded file's size, `page_count`, `freelist_count`, and the FTS5 index size
(`dbstat` over `message_fts_*`) as the corpus fingerprint. Both drivers must run against
byte-identical copies of one seeded file, so seeding runs under one driver only and T1
verifies the other reads it identically.

### 4.2 Operations to measure

**Read path** (read pool, 4 connections, `query_only(1)`; all SQL is verbatim from
`queries.go`):

| # | Operation | Iterations |
|---|---|---|
| R1 | `queryMailboxListForward`, LIMIT 50, random cursor | 5,000 |
| R2 | `queryMailboxListBackward`, LIMIT 50 | 2,000 |
| R3 | `queryMessageSummaryByID`, 50-id IN clause (one painted page) | 2,000 |
| R4 | `queryMailboxUnread`, LIMIT 50 (partial index) | 2,000 |
| R5 | `SELECT content FROM body WHERE message_id = ?`, several-KB BLOB | 5,000 |
| R6 | FTS5 `MATCH` single term, three classes (common, medium, rare), `ORDER BY rank LIMIT 50` | 2,000 each |
| R7 | FTS5 prefix `MATCH` per keystroke, `term*` at prefix lengths 2, 3, 4, 5 | 5,000 each |
| R8 | `queryThreadAcrossFolders` | 2,000 |
| R9 | `queryOccurrenceByRange` and `queryOccurrenceByLocalDate` | 2,000 each |
| R10 | QA-2's composite 500-op script, 20 replicates at different seeds, quiescent and under concurrent bulk backfill (both modes `perf_qa2_test.go` already runs) | 10,000 ops per mode |

R7's length-2 and length-3 prefixes hit the declared `prefix='2 3'` index; lengths 4 and
5 do not [F, and this is the shape most likely to blow p99].

**Write path** (single writer connection, on a fresh copy of the seeded file per
replicate, 5 replicates):

| # | Operation | Volume |
|---|---|---|
| W1 | Sync batch apply: message + `message_mailbox` + body insert per row, all three FTS triggers firing | 40 transactions x 500 rows = 20,000 appended |
| W2 | `UPDATE message SET flags = ? WHERE id = ?` (no FTS trigger) | 5,000 |
| W3 | `UPDATE message SET subject = ?, search_text = ? WHERE id = ?`, firing `trg_message_fts_update`'s delete-then-reinsert | 2,000 |
| W4 | `DELETE FROM message WHERE id = ?`, firing `trg_message_fts_delete` plus body and `message_mailbox` cascades | 5,000 |

**Maintenance and durability** (the layer where the two architectures actually differ):

| # | Operation | Iterations |
|---|---|---|
| M1 | `PRAGMA wal_checkpoint(PASSIVE)` after each W1 batch. Record the returned (busy, log, checkpointed) row | 40 |
| M2 | `PRAGMA wal_checkpoint(TRUNCATE)` at idle under `busy_timeout=50` (`checkpoint.go:63`), **with** and **without** a concurrent open read snapshot on the pool. Record duration, returned row, and WAL size after | 20 each |
| M3 | `PRAGMA incremental_vacuum(500)` after W4, repeated until the freelist drains. Record duration per call and file size | to completion |
| M4 | `PRAGMA quick_check` on the full file, cold page cache (drop caches between runs) | 3 |
| M5 | FTS5 `INSERT INTO message_fts(message_fts) VALUES('integrity-check')` after W1 through W4's trigger churn | 3 |
| M6 | FTS5 `'rebuild'` on the full index, poplar's longest single statement (`fts.go`) | 3 |
| M7 | `PRAGMA optimize`, then re-run R6 and R7. This quantifies the SQLite 3.51+ planner regression documented in modernc issue #244, which is an upstream tax both drivers inherit, not a driver difference | 3 |

**Fidelity checks** (pass/fail, not timed):

| # | Check |
|---|---|
| T0 | Restore modernc's deleted `TestTclTest` from git history, build `testfixture` against the current transpile, and run the vendored `testdata/tcl` corpus at SQLite 3.53.3. This converts "no evidence since June 2023" into evidence, and it is the single highest-value experiment available for the incumbent. Report the error count against `2a8ff5d`'s 28-of-839,686 baseline |
| T1 | Read back `page_size`, `auto_vacuum`, `journal_mode`, `synchronous`, `cache_size`, `foreign_keys`, `query_only` on a freshly created file, per driver, with pragmas spelled **both** as one script and as separate `_pragma` parameters. This settles the ordering behavior empirically rather than by source reading |
| T2 | `EXPLAIN QUERY PLAN` for all nine constants in `queries.go` plus R6 and R7, diffed between drivers and against poplar's committed goldens in `internal/store/testdata` |
| T3 | Seed an identical small corpus (10,000 messages) under an identical statement order per driver, plus once through the stock `sqlite3` CLI at 3.53.3, then compare with `sqldiff` and a page-level hash. Any divergence past the header's version bytes is a fidelity failure. This is the bar modernc's own #42 fix was verified to |
| T4 | Run poplar's QA-6 kill harness unmodified under each driver: the fixed 30-action script, SIGKILL at 200 pseudorandom points, three seeds. After each kill assert store integrity check passes, FTS5 `integrity-check` passes, and index count equals message count |

### 4.3 What to record

- **Latency**: n, p50, p95, p99, max, and the raw sample vector per operation, so
  percentiles can be recomputed without a rerun.
- **Allocation**: `-benchmem` `allocs/op` and `B/op` for every read operation, R5 and R6
  above all, since the driver boundary copy is the difference under test.
- **Memory, split three ways.** Go heap numbers alone will mislead on both drivers,
  because modernc's SQLite working set lives in `modernc.org/libc`'s allocator and
  ncruces's lives in a WASM linear mapping, and neither is on the Go GC heap [F]. Record
  per run: process peak RSS (`VmHWM` plus `VmRSS` sampled at 100 ms), and
  `runtime.MemStats` `HeapAlloc`/`HeapSys`/`Sys` sampled alongside. Take explicit RSS
  readings at four points: after pool warm-up, after R10's concurrent-backfill mode,
  immediately after M6's rebuild, and after ten minutes idle following M6. The last two
  test ncruces's never-shrinks property directly [F, `config.go` and
  `internal/sqlite3_wrap/mem_unix.go`: pages are committed by `mprotect` and never
  released within a connection's life].
- **Storage**: final file size, WAL size after each checkpoint mode, `freelist_count`,
  FTS5 index size, and the QA-5 ratio of store size to retained body bytes.
- **Build**: binary size, `go tool nm` section sizes, and cold build wall time after
  `go clean -cache`, per driver. ncruces's 4.67 MB generated `sqlite3.go` and modernc's
  58 MB `lib/` are both real compile-time costs a developer pays every day [F].
- **Environment**: Go version, driver versions and commit hashes, `GOMAXPROCS`, CPU
  governor, filesystem, and kernel version.

### 4.4 Discipline

Run five full repetitions per driver, alternating drivers between repetitions
(A/B/A/B/A/B/A/B/A/B) so thermal drift and page-cache warmth cancel rather than
accumulate on whichever ran first. Report per-operation medians across repetitions, and
run `benchstat` on the go-test benchmark lines. Pin the governor to `performance`. Drop
page caches before every cold-start measurement. The perf harness already excludes itself
from `-race` builds for exactly this reason (`perf_qa2_test.go:1`), and this suite must
too [F].

### 4.5 What result changes the ranking

Stated precisely, so the benchmark can actually decide something.

**ncruces loses first place if any one of these holds:**

- **T4 produces a single corrupt database or a failed integrity check on Linux or macOS
  where modernc does not.** This is the disqualifier. The VFS reimplementation is the
  known risk, QA-6 is a MUST, and one failure is enough.
- **T1 or T2 shows a divergence from stock SQLite that modernc does not show.** A driver
  that silently changes a query plan or a pragma outcome is a fidelity failure regardless
  of speed.
- **Peak RSS exceeds QA-5's 250 MB with one writer and four readers at the ~1 GB corpus,
  and cannot be brought under it** by `WithMaxMemory` plus read-connection recycling
  without pushing R10's p99 past 40 ms.
- **M2's TRUNCATE checkpoint under a live reader snapshot blocks past the 50 ms
  `busy_timeout`, or returns a non-conforming result row.** poplar's checkpoint policy
  depends on that bound holding exactly (`checkpoint.go:56-85`).

**modernc takes first place outright if** it wins the RSS question decisively, meaning
ncruces above 250 MB and modernc comfortably below at the same corpus. RSS is a hard
requirement and latency is not contested, so a clear RSS win outranks everything ncruces
gains elsewhere.

**modernc's position weakens further if** T0 reports a materially worse error rate at
SQLite 3.53.3 than `2a8ff5d`'s 28-of-839,686 at 3.42, or if T0 cannot be made to run at
all, which would confirm the harness is not merely unrun but rotted.

**Nothing in the latency numbers alone should move the ranking**, unless a driver misses
QA-2's 25 ms p95 or 40 ms p99 on R10 where the other holds. Both are expected to land an
order of magnitude under budget on R1 through R4, so a 3x throughput difference on point
reads is not decisive when the budget is 25 ms and the query is sub-millisecond [J].
Report the numbers, and do not rank on them.

**R7's prefix result changes the schema, not the driver**, unless one driver misses
budget on 4-and-5-character prefixes where the other holds. A shared miss is an argument
for declaring `prefix='2 3 4'` in `0001_initial.sql`.

**M7 changes neither.** Both drivers inherit the same upstream planner behavior. If
`PRAGMA optimize` moves R6 or R7 materially, that is an argument for scheduling
`PRAGMA optimize` in poplar's idle-maintenance path, which is a design task either way.
