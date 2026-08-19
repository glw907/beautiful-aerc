# poplar SQLite driver benchmark

Date 2026-08-19. Executes the benchmark plan
`docs/poplar/research/2026-07-28-sqlite-driver-audit.md` section 4
(4.0-4.5) laid out: modernc.org/sqlite v1.54.0 versus
github.com/ncruces/go-sqlite3 v0.35.3, over poplar's real query and
write shape at the 100,000-message QA-5 envelope. The ruling this
benchmark produced lives in ADR-0001 revision 3; this document is the
method, the raw numbers, and the caveats behind that ruling.

Raw artifacts (every JSON repetition, T0-T4 outputs, kill-harness
logs, build-cost measurements) are committed alongside this document
at
`docs/poplar/research/captures/2026-08-19-sqlite-driver-benchmark-results.tar.xz`;
section 16 describes its contents and schema.

## 1. Method

A driver-parameterized fork of poplar's own QA harness, built in a
scratchpad module and never merged into the repo. It imports
`0001_initial.sql` verbatim and selects a driver by build tag. Exactly
two things differ per driver, matching audit 4.0: the DSN builder
(modernc needs poplar's single-script `_pragma` form; ncruces accepts
either form) and FTS5 registration (ncruces needs
`sqlite3.AutoExtension(fts5.Register)` once before the first `Open`).
Every other package in the harness (schema, corpus generation, the
query constants, percentile math, the operations under test, JSON
reporting, RSS sampling, environment capture) compiles unmodified into
both binaries.

The corpus is audit 4.1's specification: one account, four mailboxes
at an 80/20 inbox skew, 100,000 messages spread over two years,
lognormal body sizes (4 KB median, ~13.5 KB mean, capped at 2 MB),
mail-shaped text rather than random bytes, `search_text` drawn from
200 to 800 words of the body, 5,000 calendar events and ~50,000
occurrences, and threads of 1 to 30 messages. Both drivers ran against
byte-identical copies of one seeded file per repetition.

Five full repetitions per driver, alternating (A/B/A/B/A/B/A/B/A/B,
ten runs total) so thermal drift and page-cache warmth cancel rather
than accumulate on whichever driver ran first, per audit 4.4.
Per-operation figures below are the median across the five
repetitions' own p50/p95/p99/max, reported as `min..median..max` so
rep-to-rep noise stays visible. Every artifact's raw nanosecond vector
was independently recomputed (nearest-rank percentile) and checked
against the stored p50/p95/p99/max: zero mismatches across 210 checks.

Environment, held constant across all ten repetitions except where
section 15 notes drift: Go 1.26.3, `GOMAXPROCS=8`, ext4, one Linux
kernel, `modernc.org/sqlite v1.54.0` and
`github.com/ncruces/go-sqlite3 v0.35.3` as pinned in `go.mod`.

## 2. T0: modernc's Tcl conformance suite is rotted, not merely unrun

Audit 4.2's T0 asked to restore modernc's deleted `TestTclTest` from
git history, build `testfixture` against the current transpile, and
run the vendored `testdata/tcl` corpus at SQLite 3.53.3, reporting an
error count against the `2a8ff5d` commit's baseline of 28 errors out
of 839,686 individual assertions (2023-06-10, SQLite 3.42).

**Verdict: rotted.** `TestTclTest`, its generator (`generator.go`,
887 lines), and the platform-specific generated fixtures
(`internal/testfixture/testfixture_<os>_<arch>.go`, 65,000-67,800
lines each) were deleted in one commit, `11df50c` ("upgrade supported
targets to SQLite 3.45.1"), released as v1.29.0 on 2024-02-13, roughly
eight months and one intermediate release past the `2a8ff5d` baseline
and fifteen releases before poplar's own v1.54.0 pin. The vendored
`testdata/tcl/` corpus itself is still present (1,230 files, 87 MB),
confirming only the machinery that could run it was removed, not the
corpus.

Restoring `tcl_test.go` from its last pre-deletion commit and working
the build forward hits a second, independent break before the first:
`modernc.org/tcl@v1.15.3`, the one still-published piece of the old
toolchain, fails to compile with nine type errors, all inside its own
transpiled source, none in anything touched during restoration. Its
`libc.Xioctl`/`libc.Xpthread_join` call signatures assume an older
`modernc.org/libc` than the `v1.74.1` that `modernc.org/sqlite v1.54.0`
itself pins, a drift `modernc.org/sqlite`'s own changelog says a
downstream consumer "must" avoid.

Past that, the actual stop point: `internal/testfixture` was an
internal package of `modernc.org/sqlite`, never published standalone,
and the only tool that ever generated it was deleted in the same
commit. Nothing on the module proxy substitutes for it
(`modernc.org/testfixture` and `modernc.org/libtestfixture` both
resolve to "not found"). The maintainer's current production pipeline
(`Makefile`'s `vendor` target, `vendor_libs/main.go`) was restructured
around a separate published module,
`modernc.org/libsqlite3`, with no analogous step for test-only C
sources anywhere in the current tree. Reviving testfixture generation
at 3.53.3 would mean reviving a frozen major version of `ccgo`
(`v3.12.22`, distinct from the `ccgo v4` the live pipeline uses),
hand-configuring a SQLite 3.53.3 source tree, and transpiling roughly
85 C files nobody, maintainer included, has ever transpiled against
this SQLite version, with no CI and no cross-check. That is new
engineering to reconstruct a deliberately dismantled internal build
step, not a restoration, and it would produce a testfixture binary
whose fidelity to the real one is itself unverified, making any
resulting error count uninterpretable against the `2a8ff5d` baseline.

Per audit 4.5, "T0 ... being impossible to run at all (harness rotted)"
is itself the decisive evidence T0 was designed to produce, not a
failed dispatch: the harness was actively maintained for roughly eight
months past its last reported pass rate, then deliberately deleted
alongside a SQLite version bump, and has stayed deleted through
fifteen-plus subsequent releases with no replacement, no partial
substitute, and no changelog entry in that span mentioning a
Tcl/testfixture run.

## 3. T1: pragma read-back, both DSN forms

Read back `page_size`, `auto_vacuum`, `journal_mode`, `synchronous`,
`cache_size`, `foreign_keys`, and `query_only` on a freshly created
file, per driver, with pragmas spelled both as one script and as
separate `_pragma` parameters.

| driver | form | page_size | auto_vacuum | journal_mode |
|---|---|---|---|---|
| modernc | single_script | 8192 | 2 | wal |
| modernc | separate_params | **4096** | 2 | wal |
| ncruces | single_script | 8192 | 2 | wal |
| ncruces | separate_params | 8192 | 2 | wal |

modernc's separate-`_pragma`-parameter form silently drops `page_size`
(reverting to SQLite's 4096 default): the driver re-sorts separate
parameters alphabetically, running `journal_mode` ahead of `page_size`
and `auto_vacuum`, both of which are fixed the instant a file enters
WAL mode. poplar's own DSN builder already used the single-script form
that avoids this, so the defect never reached production traffic, but
T1 confirms the reason that discipline exists. ncruces shows no
equivalent trap: it holds the correct pragmas under either DSN
spelling, matching the driver's own documentation, which states that
`_pragma` values run in query-string order.

## 4. T2: EXPLAIN QUERY PLAN, both drivers versus the repo's goldens

`internal/store/queries_test.go`'s `TestExplainQueryPlan` carries
eight committed golden cases. The harness's cross-driver diff probed
eight queries too, but not the same eight: six of the golden cases,
plus `SearchRanked` (R6) and `SearchPrefix` (R7), which that test does
not cover. The two golden cases the cross-driver diff omitted are the
outbox queries, `outbox_dispatch` and `outbox_eligibility_probe`.

**Zero difference between modernc and ncruces across the six golden
queries plus R6/R7 that the harness's cross-driver diff probed,** for
example `SEARCH message_mailbox USING COVERING INDEX
idx_message_mailbox_list (mailbox_id=? AND received_at<?)`. Separately,
the repository's own `TestExplainQueryPlan` proves all eight committed
goldens, including the two outbox plans the cross-driver diff never
touched, hold under ncruces: 8/8 pass in-repo. Those two were verified
against ncruces alone, not cross-diffed against modernc's plan. No T2
disqualifier.

## 5. T3: identical-statement-order corpus fidelity

One `corpus.Generate(10,000)` call (fixed seed) produced one in-memory
row set, replayed three ways: modernc via `database/sql` prepared
statements, ncruces via the same prepared-statement path under its own
driver, and the stock `sqlite3` CLI (3.50.1, the newest precompiled
build with FTS5 available at run time) via a literal SQL script in the
identical statement order.

All three page hashes are identical after zeroing the header's
file-change-counter (offset 24-28) and SQLite-version fields (offset
92-100):

```
modernc  c7b1176a7d81edf42fd2c5e2f17969c57e52c6e2fda6232ac80917dd37711ed
ncruces  c7b1176a7d81edf42fd2c5e2f17969c57e52c6e2fda6232ac80917dd37711ed
```

`sqldiff` between modernc-versus-stock, ncruces-versus-stock, and (run
separately) modernc-versus-ncruces all report zero rows of difference.
No T3 disqualifier.

## 6. T4: the QA-6 kill harness

poplar's own kill harness (the fixed 30-action script, SIGKILL at 200
pseudorandom points, three seeds) ran unmodified under both drivers:
the committed master build (modernc) and a driver-swapped worktree
(ncruces), 600 SIGKILL trials each. Every trial reported `ok`: no
corruption, no failed integrity check, on either driver. No T4
disqualifier, the audit's stated one-failure-is-enough bar.

## 7. Read path (R1-R10), medians across 5 reps, `min..median..max` ms

| op | stat | modernc | ncruces | ratio (n/m) | reps |
|---|---|---|---|---|---|
| R1 mailbox_list_forward | p50 | 0.049..0.147..0.150 | 0.067..0.205..0.206 | 1.40 | overlap |
| | p95 | 0.065..0.188..0.194 | 0.084..0.259..0.260 | 1.38 | overlap |
| | p99 | 0.081..0.233..0.249 | 0.104..0.299..0.343 | 1.29 | overlap |
| R2 mailbox_list_backward | p50 | 0.049..0.146..0.146 | 0.068..0.207..0.216 | 1.42 | overlap |
| R3 message_summary_by_id | p50 | 0.191..0.612..0.617 | 0.218..0.694..0.701 | 1.13 | overlap |
| R4 mailbox_unread | p50 | 0.032..0.085..0.087 | 0.040..0.121..0.126 | 1.41 | overlap |
| R5 body_read | p50 | 0.013..0.036..0.038 | 0.011..0.034..0.035 | 0.94 | overlap |
| | p95 | 0.050..0.139..0.150 | 0.025..0.068..0.072 | 0.49 | overlap |
| | p99 | 0.181..0.428..0.457 | 0.106..0.245..0.262 | 0.57 | overlap |
| R6 search_common | p50 | 274.0..529.5..538.1 | 319.6..688.2..691.0 | 1.30 | overlap |
| R6 search_medium | p50 | 16.9..33.3..33.5 | 20.2..43.7..43.9 | 1.31 | overlap |
| R6 search_rare | p50 | 1.90..3.79..3.82 | 2.28..4.91..4.98 | 1.30 | overlap |
| R7 prefix_len2 | p50 | 0.289..0.574..0.585 | 0.325..0.733..0.736 | 1.28 | overlap |
| R7 prefix_len3 | p50 | 0.291..0.572..0.589 | 0.328..0.734..0.736 | 1.28 | overlap |
| R7 prefix_len4 | p50 | 0.286..0.587..0.591 | 0.332..0.739..0.743 | 1.26 | overlap |
| R7 prefix_len5 (n=100, section 15) | p50 | 73.6..144.7..146.2 | 100.6..215.1..217.7 | 1.49 | overlap |
| | p95 | 218.9..452.1..459.2 | 317.0..687.1..716.9 | 1.52 | overlap |
| | p99 | 252.4..507.3..509.7 | 343.1..740.8..760.6 | 1.46 | overlap |
| R8 thread_across_folders | p50 | 0.031..0.065..0.067 | 0.038..0.087..0.089 | 1.35 | overlap |
| R9 occurrence_by_local_date | p50 | 0.064..0.132..0.136 | 0.085..0.188..0.190 | 1.43 | overlap |
| R9 occurrence_by_range | p50 | 0.233..0.497..0.506 | 0.283..0.623..0.627 | 1.25 | overlap |
| R10 qa2_composite | p95 | 0.309..0.640..0.645 | 0.363..0.802..0.810 | 1.25 | overlap |
| | p99 | 0.346..0.704..0.736 | 0.388..0.880..0.899 | 1.25 | overlap |
| R10 qa2_composite_under_write | p95 | 0.399..0.784..0.789 | 0.466..0.901..0.907 | 1.15 | overlap |
| | p99 | 0.572..1.063..1.168 | 0.550..1.023..1.029 | 0.96 | overlap |

The read-path separation is real. modernc runs roughly 1.2-1.4x faster
than ncruces on every row-shaped read except R5, and the ratio holds in
the same direction in all five repetitions. Rep-ranges overlap only
because the governor drift (section 15) moved both drivers together
across reps, which is exactly what the paired design controls for: the
comparison that matters is modernc against ncruces within a rep, not
one driver's range against the other's. The separation is nevertheless
non-decisive, because every budgeted operation clears its budget with
15-70x headroom on both drivers (section 13). That matches audit 4.5's
own prediction that nothing in sub-millisecond point-read latency alone
should move the ranking. R5 (body BLOB read)
is the one qualitatively different row: ncruces's p95/p99 run roughly
half of modernc's. Allocs/op are close (18-19 versus 17) but bytes/op
diverge sharply, modernc 29,889-29,890 B/op versus ncruces
15,296-15,297 B/op, essentially 2x, for the same several-KB BLOB read
(section 15 flags this as unexplained and not gating).

Allocs/op and bytes/op, median across reps (zero rep-to-rep alloc
noise on every op except R1 and R5):

| op | modernc allocs/bytes | ncruces allocs/bytes |
|---|---|---|
| R1 | 117 / 1534-1535 | 117 / 1605 |
| R2 | 118 / 1549-1552 | 117 / 1605-1608 |
| R3 | 423 / 11255 | 573 / 14845 |
| R4 | 63 / 880 | 63 / 960 |
| R5 | 18-19 / 29889-29890 | 17 / 15296-15297 |
| R6 (all 3 classes) | 65-66 / 910-912 | 64-70 / 982-1143 |
| R7 (len2-5) | 17 / 512-570 | 16 / 584-593 |
| R8 | 39 / 1112 | 39 / 1292 |
| R9 (both forms) | 74 / 988, 258 / 2472 | 73 / 1052, 258 / 2552 |

ncruces allocates marginally fewer or equal objects per op everywhere
except R3, but consistently copies more bytes per op (roughly 5-15%
more) on every row-shaped read except R5, where it copies roughly
half.

## 8. Write path (W1-W4), medians across 5 reps, `min..median..max` ms

| op | stat | modernc | ncruces | ratio (n/m) |
|---|---|---|---|---|
| W1 sync_batch_apply (n=200, 40 tx x 5 reps) | p50 | 45.1..141.9..143.9 | 56.5..180.1..181.7 | 1.27 |
| | p95 | 83.5..227.9..245.0 | 107.0..313.8..320.5 | 1.38 |
| | p99 | 362.2..1836.8..3461.7 | 417.5..424.0..2781.1 | 0.23 |
| | max | 2589.8..3662.2..5003.0 | 611.7..1423.3..2993.1 | 0.39 |
| W2 flag_update (n=25000) | p50 | 0.019..0.058..0.059 | 0.020..0.063..0.064 | 1.08 |
| W3 subject_search_text_update (n=10000) | p50 | 0.279..0.816..0.823 | 0.357..1.057..1.066 | 1.30 |
| | p99 | 24.98..42.60..45.10 | 29.99..31.20..39.30 | 0.73 |
| W4 delete (n=25000) | p50 | 0.334..0.939..0.942 | 0.449..1.233..1.240 | 1.31 |
| | p99 | 25.66..33.50..35.52 | 29.24..31.07..41.52 | 0.93 |

W1's p99/max show the widest, noisiest spread of anything in this
suite (modernc's own rep range spans 362ms to 3.5s at p99), driven by
checkpoint/fsync interaction under the batch writer, not a clean
driver signal: the ratio flips sign between p50 (ncruces slower) and
p99/max (ncruces faster) inside the same operation. Both drivers show
the same pattern on W3/W4: ncruces slower at p50 (1.3x) but at or
below modernc at p99 (0.73x-0.93x), meaning modernc's p50-to-p99 tail
is proportionally worse than ncruces's on both trigger-firing
UPDATE/DELETE paths.

## 9. Maintenance path

### M1, PASSIVE checkpoint (paired per W1 batch, 200/rep, 1000 total/driver)

| | modernc | ncruces |
|---|---|---|
| busy != 0 | 0 / 1000 | 0 / 1000 |
| checkpointed < log | 0 / 1000 | 0 / 1000 |
| duration ms min/med/max | 2.27 / 7.21 / 12863.87 | 1.64 / 4.35 / 13172.68 |
| log frames min/med/max | 66 / 79 / 539 | 65 / 79 / 538 |

Every PASSIVE checkpoint on both drivers reports `busy=0` and fully
drains what it reports. The extreme max durations (~13s on both) are
single outlier rows, not a driver pattern; median duration is
sub-10ms on both.

### M2, TRUNCATE checkpoint, with and without a live reader snapshot (20/rep/mode, 100 total/driver/mode)

| mode | driver | duration ms min/med/max | busy | log | checkpointed | wal bytes after | rows > 50ms |
|---|---|---|---|---|---|---|---|
| without reader | modernc | 0.009 / 0.030 / 4926.19 | {0} | {0} | {0} | 0 | 5/100 |
| without reader | ncruces | 0.009 / 0.032 / 8587.87 | {0} | {0} | {0} | 0 | 5/100 |
| with reader | modernc | 49.79 / 51.10 / 52.20 | {1} | {1} | {1} | 8248 | 98/100 |
| with reader | ncruces | 0.009 / 0.029 / 1.43 | {0} | {0} | {0} | 0 | 0/100 |

**The with-reader contrast in this table is a harness artifact, and
this document's earlier reading of it was wrong.** The two arms did not
measure the same thing. modernc's arm ran against a WAL holding one
8KB frame, and ncruces's arm ran against an empty WAL: the raw rows
record `(busy, log, checkpointed)` as `(1, 1, 1)` with
`wal_bytes_after=8248` on modernc, and `(0, 0, 0)` with
`wal_bytes_after=0` on all 100 ncruces samples across all 5 reps. A
TRUNCATE checkpoint with no frames to copy returns immediately on any
driver, so ncruces's sub-millisecond arm records an empty WAL rather
than a fast checkpoint. Nothing here distinguishes the drivers.

The answer condition 4 needs comes from a controlled in-tree
measurement instead, run at commit `64260cd` against poplar's own
store. Under a live reader snapshot with a non-empty WAL, ncruces
blocks 50.5-51.6ms and returns `busy=1`, the same profile modernc
showed in this table. Both drivers block roughly the full 50ms
`busy_timeout` and return a structurally well-formed row (`busy` in
{0,1}, `checkpointed <= log`). The roughly 1ms overshoot is symmetric
across the two drivers, not a property of either one.

### M3, incremental_vacuum(500) to the step cap (24 steps/rep, both drivers)

| | modernc | ncruces |
|---|---|---|
| steps/rep | [24,24,24,24,24] | [24,24,24,24,24] |
| total duration s min/med/max | 0.37 / 0.68 / 0.78 | 0.40 / 0.52 / 0.61 |
| final file size (median) | 2161.2 MB | 2161.2 MB |

Both drivers hit the harness's 24-step cap in every rep, not a
freelist-0 early exit; final file sizes match to the MB. No driver
signal.

### M4, quick_check (3/rep, whole-file)

Not tabulated as a clean cold-cache figure: the orchestrator's
cache-drop preflight step is manual and this analysis cannot verify it
ran for every one of the ten repetitions, so a reported duration would
imply a cache state that isn't certified.

### M5, FTS5 integrity-check (whole-index, 3/rep)

| | modernc | ncruces |
|---|---|---|
| p50 (median across reps) | 18.98s (range 9.30-19.12s) | 23.45s (range 10.91-23.68s) |
| max | 19.14s (range 9.35-19.18s) | 23.60s (range 10.96-23.71s) |

### M6, FTS5 rebuild (whole-index, poplar's longest statement, 3/rep)

| | modernc | ncruces |
|---|---|---|
| p50 (median across reps) | 85.40s (range 63.80-86.02s) | 100.24s (range 44.91-100.54s) |
| max | 85.51s (range 66.07-86.94s) | 101.27s (range 45.39-101.86s) |

Both M5 and M6 show wide rep-to-rep spread, most likely the paired
governor drift (section 15): the fast outlier rep in each row is rep
1, the only performance-governor rep. Medians favor modernc by roughly
1.15-1.25x on both whole-index operations.

### M7, PRAGMA optimize (3/rep) plus R6/R7 rerun

`PRAGMA optimize` itself is noise-dominated at these magnitudes: p50
across the 5 reps is 21.3, 30.3, 32.4, 36.6, 30.1 microseconds
(modernc) versus 11.9, 37.2, 35.6, 36.2, 59.6 microseconds (ncruces),
no material difference, both effectively instantaneous.

R6/R7 medians after `optimize` move by low single-digit percent on
both drivers (for example R6 search_common p50: modernc 529.5ms to
498.7ms, ncruces 688.2ms to 656.97ms; R7 prefix_len5 p50: modernc
144.7ms to 137.2ms, ncruces 215.1ms to 206.3ms). Both drivers move
together and by comparable proportions, the upstream planner tax
audit 4.2 predicted (SQLite 3.51+ regression, modernc issue #244),
not a driver difference.

## 10. Memory (RSS), four checkpoints, `min..median..max` MB across 5 reps

| checkpoint | modernc VmHWM | modernc VmRSS | ncruces VmHWM | ncruces VmRSS |
|---|---|---|---|---|
| after_pool_warmup | 10.41..10.54..10.62 | 10.41..10.54..10.62 | 9.89..9.92..10.05 | 9.89..9.92..10.05 |
| after_r10_concurrent_backfill | 23.40..23.58..24.40 | 17.16..17.43..18.91 | 34.67..35.04..35.27 | 32.50..32.66..33.38 |
| immediately_after_m6_rebuild | 23.40..23.58..24.40 | 18.05..18.12..18.79 | 34.67..35.04..35.27 | 19.52..19.76..19.96 |
| after_idle_following_m6 (real 10-min idle wait, full mode) | 28.06..28.25..28.62 | 17.14..17.27..17.86 | 34.67..35.09..35.27 | 18.59..18.95..19.05 |

Both drivers finish an order of magnitude under QA-5's 250MB ceiling
at this ~2.25GB corpus scale, with one writer and four readers
exercised through R10's concurrent backfill and the M6 whole-index
rebuild: modernc's peak VmHWM is 28.62MB (at least 8.7x under budget),
ncruces's is 35.27MB (at least 7.1x under budget). ncruces's peak
VmHWM never shrinks across the three post-warmup checkpoints, matching
the never-shrinks-within-a-connection prediction for its WASM linear
memory, while modernc's peak VmHWM also grows monotonically
(24.4MB to 28.6MB), as expected of `VmHWM` by definition on either
driver. The useful contrast is VmRSS, which for both drivers stays
essentially flat (within ~2MB) from `after_r10_concurrent_backfill`
onward, showing no unbounded growth on either driver at this scale.

## 11. Storage snapshots, median across 5 reps

| phase | driver | file | WAL | freelist | FTS5 index |
|---|---|---|---|---|---|
| after_reads_phase | modernc | 2247.3 MB | 23.14 MB | 3227 | 352.18 MB |
| after_reads_phase | ncruces | 2247.3 MB | 17.41 MB | 3924 | unavailable (`dbstat` gap, section 15) |
| after_writes_phase | modernc | 2259.1 MB | 10.89 MB | 12631 | 353.48 MB |
| after_writes_phase | ncruces | 2259.1 MB | 10.89 MB | 12631 | unavailable |
| after_maintenance | modernc | 2151.3 MB | 346.86 MB | 2400 | 335.02 MB |
| after_maintenance | ncruces | 2151.3 MB | 345.86 MB | 2397 | unavailable |

File sizes match to the MB between drivers at every phase, as expected
of an identical schema and corpus. WAL sizes and freelist counts track
closely except `after_reads_phase`, where ncruces's freelist count
(3924) runs higher than modernc's (3227) purely from checkpoint-timing
differences during the read-only phase, reflecting residual state
carried from seeding rather than a read-path effect. modernc's FTS5
index tracks at roughly 335-353MB across the three phases, about
15-16% of file size; ncruces's is unmeasurable through `dbstat` on
this harness build (section 15).

## 12. Build cost (single measurement per driver, harness's `cmd/bench` binary, not `cmd/poplar`)

| | modernc | ncruces |
|---|---|---|
| binary size | 10.60 MB | 18.13 MB (1.71x) |
| cold build (`go clean -cache` then build) | 33.9s | 44.8s (1.32x) |
| `nm` text (T) | 3.45 MB | 5.39 MB (1.56x) |
| `nm` data (D) | 33.94 MB | 33.83 MB (~equal) |

ncruces's embedded, machine-translated SQLite source accounts for the
larger text section and binary size; modernc's larger `D` section
reflects its own C-transpiled tables. Cold build time is a single
one-shot measurement on each side, not a median, and the gap (roughly
11 seconds on this machine) is not a material tiebreaker either way.

## 13. Section 4.5 conditions, answered with numbers

**Condition 1, T4: any corrupt database or failed integrity check on
either driver.** No. All 600 kill trials per driver report `ok`.
Section 6. No disqualifier.

**Condition 2, T1/T2 divergence from stock where modernc shows none.**
ncruces shows none; the one divergence runs the other direction.
Section 3's `page_size` defect is modernc's. Section 4's EXPLAIN plans
show zero diff across all eight queries probed. No disqualifier for
ncruces.

**Condition 3, peak RSS versus QA-5's 250MB, both drivers, all four
checkpoints.** Neither driver comes close: modernc's peak VmHWM across
all four checkpoints and all 5 reps is 28.62MB, ncruces's is 35.27MB
(section 10). Neither the ncruces-disqualifying condition nor
modernc's outright-win condition (ncruces above 250MB, modernc
comfortably below) fires.

**Condition 4, M2 TRUNCATE under a live reader: blocks past the 50ms
`busy_timeout`, or a non-conforming row.** This condition is answered
by the in-tree controlled measurement, not by the benchmark. The
benchmark's ncruces arm ran against an empty WAL and so measured
nothing about a blocked checkpoint (section 9). Under a live reader
snapshot with a non-empty WAL, both drivers block roughly the full
50ms budget and return `busy=1`: modernc at 49.8-52.2ms in the
benchmark, ncruces at 50.5-51.6ms in the in-tree measurement at
`64260cd`. Neither driver returns a structurally non-conforming row.
Whether a roughly 1ms overshoot on a timeout a driver is, by design,
supposed to return from around 50ms counts as "blocking past" the
bound in the audit's disqualifying sense is a judgment call this
document does not resolve. It is symmetric across both drivers either
way, so it fires no disqualifier against either one.

**QA-2 budget on R10 (25ms p95 / 40ms p99): does either driver miss
where the other holds?** Neither misses. R10 quiescent: modernc p95
0.31-0.65ms / p99 0.35-0.74ms; ncruces p95 0.36-0.81ms / p99
0.39-0.90ms. R10 under concurrent write: modernc p95 0.40-0.79ms / p99
0.57-1.17ms; ncruces p95 0.47-0.91ms / p99 0.55-1.03ms. Both land
roughly 30-70x under budget in every rep. No latency disqualifier
either direction.

**R7 length-4/length-5 budget: does either driver miss where the other
holds?** Both miss, at length 5 only, a shared miss rather than a
driver difference. Length 4 (n=5000, hits no declared prefix index but
stays fast): both comfortably under budget. Length 5 (n=100, section
15) blows the budget on both drivers by roughly 10-20x: modernc p95
218.9-459.2ms / p99 252.4-509.7ms; ncruces p95 317.0-716.9ms / p99
343.1-760.6ms. Per audit 4.5, a shared miss at length 5 is an argument
for extending the FTS5 prefix index declaration (`prefix='2 3'` to
`'2 3 4'`) in `0001_initial.sql`, not a driver-selection signal.

**M7 optimize effect on R6/R7 (upstream tax, report only).** Both
drivers move together after `PRAGMA optimize`, roughly 4-8% on both.
Section 9. Consistent with the audit's framing: SQLite's own planner
behavior, inherited identically by both drivers.

**Build costs.** Binary size 1.71x, cold build 1.32x, both ncruces
higher. Section 12. Single one-shot measurements, not medians.

**T0.** Rotted rather than merely unrun, per section 2. Per audit 4.5,
this weakens the incumbent's position rather than leaving it neutral.

## 14. What the criteria leave undecided

- **M2's ~51ms-versus-50ms overshoot is measured but not adjudicated.**
  The audit's condition 4 text does not specify a tolerance for "blocks
  past" the bound. modernc's 98/100 benchmark samples exceed 50.000ms
  by amounts up to roughly 2.2ms (52.20ms max), and the in-tree ncruces
  measurement lands in the same band at 50.5-51.6ms. Whether that
  counts as the disqualifying case the audit intends is outside this
  document's scope. It is symmetric across the two drivers, so it does
  not separate them. Flagged in ADR-0001 revision 3 as a
  `checkpoint.go` policy follow-up regardless of which driver won.
- **R5's roughly 2x bytes/op gap** (modernc ~29.9KB versus ncruces
  ~15.3KB for the same BLOB read) is measured but not explained by
  this artifact set; it may reflect a genuine copy-count difference in
  the two drivers' `database/sql` BLOB-scanning paths, or an artifact
  of how each driver's `Rows.Scan` allocates for a `[]byte`
  destination. Not gating (audit 4.5 does not name R5, and both
  drivers clear R10's budget regardless); worth a targeted look if R5
  ever becomes gating.
- **Why R7 length-5, but not length-4, blows the FTS5 budget by
  10-20x on both drivers** is observed, not explained: plausibly a
  term-cardinality effect of the corpus vocabulary at 5-character
  prefixes rather than a fixed schema property, but this benchmark
  captured only one representative `r7_search_prefix` plan, not one
  per prefix length.

## 15. Measurement caveats

**CPU governor drifted off `performance` for 4 of 5 repetitions.**
Only repetition 1, both drivers, ran under `governor=performance`;
repetitions 2 through 5, both drivers, ran under `governor=powersave`.
The drift is paired: for any given repetition number, both drivers ran
under the same governor state, so the modernc-versus-ncruces ratios
throughout this document are not invalidated by it, since both sides
took the same thermal/frequency hit in the same rep. Absolute latency
numbers are not clean: the cross-rep medians mix one
performance-governor sample with four powersave-governor samples per
operation, over-weighting powersave conditions 4-to-1 relative to the
audit's own preflight intent (governor pinned to `performance`
throughout). The QA-2 budget checks in section 13 still clear by wide
margins even under this handicap, so it does not change those
verdicts; a re-run with the governor correctly held at `performance`
for all five reps would likely show tighter and somewhat lower
absolute latencies for every operation on both drivers.

**R7 length-5 ran at 100 iterations, not the audit's specified 5,000.**
An orchestrator ruling made after the realistic corpus put a single
length-5 query at 0.6-1s in a preliminary probe ahead of the full
benchmark, an estimated roughly 80 minutes per repetition at the full
count on this one non-decisive operation. The benchmark's own measured
p50 at n=100 came in at 144.7ms (modernc) and 215.1ms (ncruces), lower
than the pre-ruling probe's per-query estimate, consistent with that
early figure being a rough single-query observation rather than a
percentile. 100 samples still resolve p95/p99; lengths 2 through 4
stayed at the audit's 5,000.

**`dbstat` is unavailable on ncruces's default build.** `SELECT ...
FROM dbstat` fails `no such table: dbstat` under ncruces, confirmed
with a standalone repro beyond this harness. This affects only test
and perf tooling today, not production store code: poplar's own
`storetest/corpus.go` fingerprint now sums `message_fts_data`'s stored
block bytes rather than reading `dbstat`, documented at the query site
and in ADR-0001 revision 3.

**M2's with-reader arm did not measure ncruces at all.** The harness
left ncruces's WAL empty on all 100 with-reader samples
(`wal_bytes_after=0`, `log=0`), while modernc's held one 8KB frame, so
the two arms are not comparable and the sub-millisecond ncruces figure
records an empty WAL rather than a fast checkpoint. This document's
first version read that contrast as a real divergence on the
incumbent's arm, and that reading was wrong. Section 9 carries the raw
rows and the correction; condition 4's answer comes from the in-tree
controlled measurement at `64260cd`, where both drivers block roughly
the full `busy_timeout` and return `busy=1`. The remaining question is
whether a 50ms bound is the right policy at all, flagged as a
`checkpoint.go` follow-up independent of the driver decision.

## 16. Raw artifacts

`docs/poplar/research/captures/2026-08-19-sqlite-driver-benchmark-results.tar.xz`
(3.5MB) holds every artifact this document's numbers are drawn from,
34 files, flat inside the archive:

- **Ten full-repetition JSON files**, `<driver>-rep<N>-full-<unixnano>.json`
  (`modernc-rep1` through `rep5`, `ncruces-rep1` through `rep5`). Each
  is one complete run's report: `driver`, `repetition`, `mode`,
  `started_at`/`finished_at`, an `env` block (`go_version`, `driver`,
  `driver_module`, `driver_version`, `driver_sum`, `gomaxprocs`,
  `cpu_governor`, `filesystem`, `kernel`, `hostname`), a `reads` array
  (17 entries, one per read operation/class: `op`, `n`, `p50_ns`,
  `p95_ns`, `p99_ns`, `max_ns`, `allocs_per_op`, `bytes_per_op`, and
  the full `nanos` sample vector), a `writes` array (4 entries, same
  latency shape, no alloc fields), a `maintenance` object keyed by
  operation (`m1_passive` through `m7_rerun_r7`, each holding that
  operation's own result rows), an `rss` array (the four checkpoints
  of section 10, each with `label`, `at`, `vm_hwm_kb`, `vm_rss_kb`,
  and `runtime.MemStats` fields), and a `storage` array (the three
  phases of section 11).
- **Two smoke-mode JSON files**, `<driver>-rep1-smoke-<unixnano>.json`:
  disposable proof-of-execution artifacts at 1/50th iteration counts,
  not analysis input (harness report section 6).
- **Two build-cost JSON files**, `buildcost-<driver>.json`: the
  section 12 figures.
- **Six fidelity JSON files**, `t1-<driver>.json`, `t2-<driver>.json`,
  `t3-<driver>.json`: the raw T1/T2/T3 results sections 3-5 summarize.
- **Ten repetition log files**, `rep<N>-<driver>.log`: the harness's
  own stderr/stdout for each of the ten full repetitions (preflight
  notices, timing, exit status).
- **Six T4 kill-harness log files**,
  `t4-wt-task4-<modernc|ncruces-swap>-seed<1,2,3>.log`: the 200
  kill-point, per-seed `go test` output for both driver arms, section
  6.

No corpus database files are included; the 2.25GB seeded corpus this
benchmark ran against is regenerable from
`internal/store/storetest/corpus.go`'s generator (fixed seed) and was
never itself an input worth archiving.
