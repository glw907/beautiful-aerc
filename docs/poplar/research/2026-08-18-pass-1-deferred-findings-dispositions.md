# Pass 1 deferred findings, triaged and disposed

Promoted from pass 1b task 11b's harvest of the 135 findings pass 1
deferred, with every row's disposition resolved by pass 1c task 1.
The pass 1b workspace is deleted at close, so this is the surviving
copy and the standing input later passes read.

## Count and reconciliation

`grep -c 'minor (deferred)\|OPEN at cap' .superpowers/sdd/2026-07-27-pass-1-foundation/progress.md` returns **135**. This document accounts for all 135, grouped by cluster rather than ledger order. Each entry carries the ledger's line number (from `progress.md` as of this harvest), the task that recorded it, the file/symbol it names, whether that file/symbol still exists at HEAD, a one-sentence restatement, a recommendation (`FIX` / `DECLINE` / `ALREADY CLOSED` / `NEEDS READING`), and a cluster tag.

**Verification method and its limits.** I read the current HEAD content of every file a finding names (or its renamed successor), and where a finding's specific complaint looked resolved I searched `git log` for the closing commit. That gave direct evidence for roughly 100 of the 135 entries. For the remaining pure-prose/style nitpicks in packages pass 1b never touched functionally (`internal/store/queries.go`, `dsn.go`, `flags.go` comments, `internal/store/schema_test.go`, `mailbox_role.go`'s remaining comment findings, some of Task 2/3's doc-comment items), I confirmed the file and approximate location still exist and read the cited region, but did not cross-check every sentence against the exact original wording pass 1's reviewer saw. Those are marked with a confidence note inline; none are asserted as `ALREADY CLOSED` without having actually read the current text.

**Two known renames that make "still exists" non-trivial**, both confirmed against HEAD:
- `internal/backend/jmap` -> `internal/backend/jmapsource`, commit `9c3a56b` ("Rewrite the JMAP adapter as internal/backend/jmapsource on poplar/jmap"). Every Task 8 finding below is re-pointed to the `jmapsource` path.
- `internal/backend/fake.go` / `fake_test.go` -> `internal/backend/backendtest/fake.go` / `fake_test.go`, commit `3db2201` ("Move the scriptable backend fake to internal/backend/backendtest"). Every Task 7 finding against the fake is re-pointed to `backendtest`.
- `internal/store/perf_measure_test.go` and `perf_seed_test.go` no longer exist by those names; their content is now spread across `internal/store/storetest/perf.go`, `internal/store/perf_qa2_test.go`, `perf_qa3_test.go`, `perf_extra_test.go`, and `cmd/poplar/perf_qa1_test.go` (commit `249e73f`, "Fix QA-1/2/3 perf harness review findings", and the harness's original add `80988a1`). Task 12 findings are re-pointed accordingly.

A large share of the deferred findings turned out to be **already closed** by pass 1b's substantive work, most concentrated in one commit: `7d29fe1` ("Consolidate pass 1's duplicated tables, helpers, and fixtures") folded the three flag-keyword-map copies into one, derived `store.namedFlag` from `flagKeyword`'s inverse, and gave `sync`/`outbox`/`store` shared `storetest` seeding helpers. A second commit, `e34f06c` ("Surface the store and log failures recovery discarded"), closed the whole `NeedsIntegrityCheck`/quarantine-rename discard cluster in one pass. A third, `267e95a` ("Stop stranding batched intents in dispatching"), closed the Task 10 Critical finding, the `Undo` transaction-closure bug, and the Task 1b bare-`errors.New` finding simultaneously.

---

## Cluster: mark-clean-shutdown-and-pid-atomicity

Non-atomic `os.WriteFile` for the clean-shutdown marker and the instance-lock pid file, named as its own cluster in the brief.

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 1 | 387 | 11 | `internal/store/recovery.go:24` `MarkCleanShutdown` | Yes | `MarkCleanShutdown` and the lock's pid write use plain `os.WriteFile` instead of the mandated atomic temp+sync+rename pattern, with no comment explaining the exception. | **FIX** — verified still plain `os.WriteFile(cleanShutdownMarker(dbPath), nil, 0o600)` at recovery.go:24. Either implement the atomic pattern for both writers, or add a WHY comment: these are single-byte/empty sentinel files where a torn write degrades to "run the integrity check anyway" (safe) rather than data loss, which may justify the exception, but that reasoning isn't written down anywhere today. |
| 2 | 379 | 11 | `internal/platform/lock.go:44` pid write | Yes | The `nolint` on the pid `os.WriteFile` cites gosec rule `G703`, which does not exist in gosec v2.26.1; the real rule that would fire on `os.WriteFile` is `G306`. | **PREMISE FAILED THE CHECK (2026-08-18, fix round 1)** — this finding's own premise is false. Running the repo's pinned linter (`tools/bin/golangci-lint`, gosec v2.26.1) with the suppression deleted reports `G703: Path traversal via taint analysis` at this exact line; `G306` cannot fire here, since the file mode is `0o600`. `G703` exists and is the rule that fires. Pass 1c task 1's first attempt at this row changed `G703` to `G306` on the finding's word alone, without running the linter to confirm; that was a regression, caught by fix-round review and reverted. The suppression is restored to `G703` with its original reason text. Lesson: a suppression-correctness claim needs the linter run, not just a rule-id sanity check. |

---

## Cluster: needs-integrity-check-mutating-predicate

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 3 | 380 | 11 | `internal/store/recovery.go:37` `NeedsIntegrityCheck` | Yes | `NeedsIntegrityCheck` is named as a predicate but mutates (removes the clean-shutdown marker as a side effect), and originally discarded `os.Remove`'s error. | **ALREADY CLOSED (partial) + FIX (remainder)** — commit `e34f06c` ("Surface the store and log failures recovery discarded") fixed the error-discard half: current code does `if err := os.Remove(marker); err != nil { slog.Error(...); return true }` rather than `_ = os.Remove(marker)`. The naming-as-mutating-predicate design smell itself is unchanged: the function still both queries and consumes the marker under a boolean-predicate name. Recommend splitting into a query and a separate `ConsumeCleanShutdownMarker`, or renaming to something like `ShouldRunIntegrityCheck` with an explicit doc note that it consumes the marker as a side effect (already documented in prose, just not in the name). Low priority given the error-handling half is fixed. |

---

## Cluster: swallowed-quarantine-renames

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 4 | 381 | 11 | `internal/store/recovery.go:102` sidecar rename | Yes (moved) | `_ = os.Rename(path+suffix, quarantined+suffix)` swallowed every error to tolerate the common ENOENT case, so a genuine rename failure would leave a stale `-wal`/`-shm` beside the fresh store with no trace. | **ALREADY CLOSED** — commit `e34f06c`. Current `moveSidecars` (recovery.go) collects non-ENOENT errors via `errors.Join` and both call sites (`Recover`'s quarantine step and `unquarantine`'s rollback) now `slog.Error` the failure; the rollback path also folds it into the returned error via `errors.Join(rebuildErr, restoreErr)`. |
| 5 | 390 | 11 | `internal/store/recovery.go:102` (duplicate of #4) | Yes | Same finding, recorded a second time in the same review round: `Recover()`'s WAL/SHM sidecar quarantine silently discards any `os.Rename` failure. | **ALREADY CLOSED** — same commit and evidence as #4. |
| 6 | 269 | 1b | `internal/store/checkpoint_test.go:53` overlapping bands | Yes | `TestCheckpointPassiveReclaimsWithoutAReader`'s ceiling rose to 600,000 while `TestCheckpointLifecycle`'s floor stayed at 500,000, so the two tests' assertion bands overlapped and neither could discriminate the behavior it claims to. | **ALREADY CLOSED** — current `TestCheckpointPassiveReclaimsWithoutAReader` asserts staying "well under the 500,000 bytes `TestCheckpointLifecycle`'s blocked reader grows past" (checkpoint_test.go:87), not a competing 600,000 ceiling. The overlap is gone. |

(#6 is grouped here rather than under recovery specifically because it is the same "quarantine/rebuild-adjacent, already fixed in the Task 1b round" character; it doesn't share code with #4/#5 but shares the disposition pattern worth batching in review.)

---

## Cluster: rebuild-index-error-provenance

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 7 | 383 | 11 | `internal/store/fts.go:19` `RebuildIndex` | Yes | `RebuildIndex` no longer wraps its failure with op `store.rebuild-index`; the error now arrives from `Writer.execute` labeled op `store.write`, losing the provenance a log line needs to tell a rebuild failure from an ordinary write failure. | **FIX** — verified still open. `fts.go`'s `RebuildIndex` returns the closure's raw error straight from `w.Apply`; `writer.go`'s `execute` wraps every failure as `localErr("store.write", err)` (lines 267/271/274). Wrap the error inside `RebuildIndex`'s own closure (`return localErr("store.rebuild-index", err)`) before it reaches `Writer.execute`, or have the closure return a typed sentinel `Writer.execute` recognizes and re-tags. |

---

## Cluster: duplicated-test-helpers

Spans `internal/sync`, `internal/outbox`, `internal/store`, and a fourth location. Some instances were closed by the consolidation commit; others were not.

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 8 | 137 | 3 | `internal/store/read_test.go:120` account+mailbox seed x3 | Yes | The account+mailbox seed was spelled three times in the store package (`read_test.go`, `revision_test.go:62`, `fts_test.go:22`). | **ALREADY CLOSED** — commit `7d29fe1` added `internal/store/store_test.go`'s `seedAccountAndMailbox` and `storetest.Insert`/`ScanValue`, which the consolidation commit message states explicitly folds this duplication. `store_test.go:33` now hosts `seedAccountAndMailbox`. |
| 9 | 384 | 11 | `internal/store/recovery_test.go:21` `newRecoverableTestWriter` | Yes | `newRecoverableTestWriter` is a verbatim copy of `writer_test.go`'s `newTestWriter` minus the `t.Cleanup` line. | **FIX** — verified both functions still exist separately (`writer_test.go:73`, `recovery_test.go:21`). The suggested fix in the ledger itself is the cleanest: make `newTestWriter`'s cleanup idempotent (`sync.Once` or a closed flag) and have `recovery_test.go` call it directly instead of maintaining a near-duplicate. |
| 10 | 389 | 11 | `internal/store/recovery_test.go:1287` `errors.As`+Class block | Yes, but line number stale | The `errors.As(err, &uerrErr)` + `Class`-equality assertion block is duplicated verbatim across five tests instead of factored into a shared helper. | **FIX** — the file has shrunk (now 730 lines; line 1287 no longer exists, meaning content moved/was trimmed since this was recorded), but the pattern itself is still present: `var uerrErr uerr.Error` appears 4 times (lines 184, 344, 387, 702) each followed by the same `errors.As` + `Class` check. Extract a `assertClass(t, err, uerr.ClassX)` helper in the package (or in `storetest`, the fourth location below) and use it across `recovery_test.go`, `writer_test.go`, `migrate_test.go`, `perf_qa2_test.go`, `main_test.go`, `engine_test.go`, and `lock_test.go` — the same `errors.As(err, &uerrErr)` shape appears in all of those per a repo-wide grep. |
| 11 | 247 | 9 | `internal/sync/apply.go:17` `flagKeyword` | No (removed) | `flagKeyword` was a third copy of the poplar-name-to-JMAP-keyword map, duplicating `internal/backend/jmap`'s `messageFlagKeywords` and `internal/store`'s `namedFlag`. | **ALREADY CLOSED** — `flagKeyword` no longer exists in `apply.go` (grep confirms). Commit `7d29fe1`'s message states it folded "the three copies of the message flag-keyword table into one `backend.MessageFlagKeywords`" and derives `store`'s keyword-to-bit map from its inverse (see also #33 below). |
| 12 | 74 | 1 | `internal/store/flags.go:24` `namedFlag`/`flagKeyword` | Yes | `namedFlag` and `flagKeyword` were two hand-maintained maps holding the same five pairs. | **ALREADY CLOSED** — same commit as #11. Current `flags.go:34` derives `namedFlag` from `flagKeyword` via an inverse-building closure (`var namedFlag = func() map[string]Flags {...}()`), exactly the fix the finding asked for. |
| — | — | — | fourth location: `cmd/poplar` and `internal/outbox` | Yes | The brief names a fourth package sharing this duplicated-helper cluster beyond sync/outbox/store; I could not identify a single clean fourth package with byte-identical duplication. `cmd/poplar/main_test.go` has its own local `seedStore`/`seedStoreNeedingRecovery`/`seedStrandedDispatchingRow` helpers that overlap in purpose (not byte-for-byte code) with `internal/store/store_test.go`'s seeding helpers, and `internal/outbox/helpers_test.go` has its own `seedAccount`/`seedMailbox`/`seedMessage` distinct from `internal/sync`'s versions of the same names. | **DECLINED** (pass 1c) — `cmd/poplar`'s `seedStore` opens its own file-path connection (`store.OpenWriteConn` + `store.Migrate` against a bare path, the same thing `main.go` does at startup), a different shape from the Writer-handle-based seed helpers in `store`/`sync`/`outbox`. `internal/outbox`'s and `internal/sync`'s same-named `seedAccount`/`seedMailbox`/`seedMessage` differ in parameter order, reflecting each package's own call-site convention, confirmed by reading both. Neither is the byte-identical duplication the original finding named; carried as an observation, not consolidated. |

---

## Cluster: startup-trace-bypasses-slog

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 13 | 399 | 12 | `cmd/poplar/main.go:172` (now ~`runStartupTrace`'s final line) JSON encode error | Yes | `json.NewEncoder(out).Encode(...)`'s error is returned raw from `run` and printed to stderr without ever reaching slog, while every other failure path in `runStartupTrace` goes through uerr/slog first. | **FIX, but the original contrast is stale** — verified: `runStartupTrace` now returns every error raw (the JSON-encode error, the `writer.Close()` error, and the `MarkCleanShutdown` error are all bare `return err`), so the "while every other path goes through slog" half of the original claim no longer describes the code (the function was simplified since this was recorded; see #14). The core concern — a `--startup-trace` JSON artifact write failure never reaching the log — still holds for all of `runStartupTrace`'s error paths, not just the encoder. Recommend wrapping `runStartupTrace`'s own errors through `uerr.New` (as `run`'s other paths do) rather than singling out the encoder. |
| 14 | 400 | 12 | `cmd/poplar/main.go:144` repeated `_ = writer.Close()` | Yes, but code changed shape | `runStartupTrace` repeated `_ = writer.Close()` on five paths; one deferred close would remove four of them. | **ALREADY CLOSED** — current `runStartupTrace` (main.go:206) has been rewritten and now has only two `writer.Close()` call sites (one on the error branch after `timeFirstPage` fails, one on the success path before `MarkCleanShutdown`), not five. The specific over-repetition this flagged is gone. |
| 15 | 403 | 12 | `cmd/poplar/main.go:123` `startupTraceResult` missing quick_check | Yes | `startupTraceResult` drops the `quick_check` phase the legacy spike reported as `check_ns`, so nothing in the trace or artifact evidences the criterion's "quick_check off the launch path" claim. | **DECLINED** (pass 1c) — `start := time.Now()` is `run`'s first statement, ahead of `prepareStore` and therefore ahead of any `CheckIntegrity`/`quick_check`, so the check's cost already sits inside `OpenNS` on any run that pays it. `TestQA1Startup` proves the check is off the launch path by construction: a warm-up `--startup-trace` run pays it and writes the clean-shutdown marker, and every measured run after finds the marker and skips it. A discrete `check_ns` field would itemize a cost already captured. |

---

## Cluster: conformance (new this session, both on `jmap/conformance_dv_test.go`)

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 16 | — | (routed this session) | `jmap/conformance_dv_test.go:662` `TestDV11DuplicateMailboxName` | Yes | DV-11 asserts `existingId` on an `alreadyExists` error, citing "RFC 8620 section 5.3." The citation is wrong: `alreadyExists` is defined in section 5.4 (`/copy`), which the IANA registry also references. Stalwart sends `existingId`; Fastmail does not (verified live). The assertion at conformance_dv_test.go:665 (`if refused.Type == "alreadyExists" && refused.ExistingID != first { t.Errorf(...) }`) is unconditional whenever the server answers with `alreadyExists`, so it currently passes only because `make conformance` runs against Stalwart, and would fail against Fastmail, the server poplar actually ships against. | **FIX** — correct the citation to section 5.4, and demote the `existingId` check from a hard assertion to a recorded, server-conditional divergence (`t.Logf("DIVERGENCE: ...")`, matching the pattern already used at conformance_dv_test.go:198 for DV-04's Fastmail `notFound` omission). |
| 17 | — | (routed this session) | `jmap/conformance_dv_test.go` — missing DV row | N/A (row doesn't exist) | A DV row is owed for a sibling-uniqueness normalization divergence: Stalwart's mailbox sibling-uniqueness rule is case-insensitive and whitespace-trimming; Fastmail's is byte-exact. Both refuse a byte-identical duplicate (which is all `TestDV11DuplicateMailboxName` currently tests). Found during task 7b; the code (`internal/outbox`'s sibling-key handling, `siblingKey`/`reclaim.go`) already handles the divergence correctly. What's missing is the DV row recording it. | **FIX** — add a new `TestDV13` (or renumber to fit) exercising a near-duplicate name that differs only by case or leading/trailing whitespace, asserting poplar's own normalization decision rather than asserting a specific server behavior (since the two reference servers disagree), the same "assert the mechanism, not a server's specific answer" pattern DV-12's doc comment already states as this suite's philosophy. |

---

## Cluster: uerr (Task 4) — full set

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 18 | 14 | 4 | `internal/uerr/log.go:47` (now ~124-131) `logError` | Yes | The log record puts the operation in slog's `msg` field and names an attribute `message` for something else (the user-facing sentence), which reads oddly next to slog's usual message/attrs convention. | **FIX** — verified still current: `logger().Error(e.Op, ..., slog.String("message", e.Message))`. This is a naming clash worth a rename (e.g. `slog.String("sentence", e.Message)`) or a comment explaining the deliberate choice; cheap either way. |
| 19 | 15 | 4 | `internal/uerr/uerr_test.go:438` (now ~76-102) `TestEveryClassLogs` | Yes | `TestEveryClassLogs` does not assert the operation or the ids, the two fields ER-1's acceptance criterion actually names; it only checks class, cause, and message. | **FIX** — verified: current test checks `rec["class"]`, `rec["cause"]`, `rec["message"]` only. Add assertions on `rec["op"]` (the slog `msg` field per #18) and the `ids` attribute. |
| 20 | 16 | 4 | `internal/uerr/uerr.go:32` (now ~86-100) `sentence` map / `Class.String()` | Yes | Two hand-synced per-class tables (the `sentence` map and the `Class.String()` switch); a class added to one and not the other yields an empty user-facing sentence. | **FIX** — verified both structures still separately hand-maintained. Derive one from the other (e.g. build `sentence` keyed by the string form, or generate `String()` from a single table via a small helper), matching the pattern `flags.go` now uses for `namedFlag`/`flagKeyword` (#12). |
| 21 | 17 | 4 | `internal/uerr/log_test.go:222` no disk-write integration test | Yes | No test exercises the production path from `New` through the real writer to a file on disk. | **DECLINE** — verified: `TestLogRotates` constructs a `rotatingWriter` directly and calls `.Write()`, bypassing `New`/`logHandle`/`logger()`. Each seam (`New`→`logError`, rotation, health, `RedirectForTest`, level) is already independently unit-tested; an end-to-end disk-write test would mostly duplicate that coverage for low marginal risk. Worth doing only if a future bug specifically escapes at the integration boundary. |
| 22 | 18 | 4 | `internal/uerr/log.go:83` (now ~170) `nolint:gosec` reason | Yes | The `nolint` cited the linter's own name rather than gosec's rule id, and its stated reason was wider than the code it guarded. | **ALREADY CLOSED** — current code at log.go:170 reads `//nolint:gosec // G304: w.path comes from xdg.StateFile or a test fixture, never user input`, correctly citing G304 with a scoped reason. |
| 23 | 19 | 4 | `internal/uerr/log.go:77` (now ~164) `rotatingWriter.Write` stat check | Yes | A non-ENOENT stat error silently skips rotation, and the sentinel check used the pre-`errors.Is` form. | **FIX** — verified still current: `if info, err := os.Stat(w.path); err == nil && info.Size()+... > w.maxSize { ...rotate... }` — any stat error (not just ENOENT) silently skips the rotation check rather than surfacing. Distinguish `os.IsNotExist(err)` (fine, no file yet) from any other stat failure (should log via the existing `fail`/`health` mechanism). |
| 24 | 20 | 4 | `internal/uerr/log.go:36` (now ~85-93) `openLogWriter` stderr fallback | Yes | The stderr fallback writes JSON log lines into the terminal a full-screen TUI is rendering. | **DECLINE (for now)** — verified the fallback (`return os.Stderr` on `xdg.StateFile` failure) is unchanged. Pass 1/1b ships no TUI yet (`cmd/poplar/main.go`'s own package doc: "Pass 1 has no screen yet"), so there is no terminal being corrupted today. Revisit before a bubbletea UI lands. |
| 25 | 21 | 4 | `internal/uerr/uerr.go:70` (now ~102-113) `Error.Message` exported | Yes | Exported `Message` plus a composite-literal-only analyzer leaves a construction path outside the seam: a caller can't build a fresh `Error` via composite literal (the `errorconstruction` analyzer blocks it), but can still mutate `Message` on an `Error` value it already holds. | **DECLINE** — the `errorconstruction` analyzer (confirmed present: `tools/analyzers/errorconstruction`) does its stated job, blocking composite-literal construction outside `internal/uerr`. Post-construction field mutation is a general Go property of any exported-field struct and isn't specific to this seam; closing it fully would mean unexporting `Message` and adding a getter, a larger API change than this finding's severity warrants. |
| 26 | 22 | 4 | `internal/uerr/log.go:92` (now ~119-123) `logError` doc comment | Yes | `logError`'s doc comment uses a setup-colon-then-list construction. | **FIX** — verified current text: "logError writes e's log line: operation, ids, class, cause, and the user sentence, so..." Still a colon-then-list. Cheap prose fix. |
| 27 | 23 | 4 | `internal/uerr/log.go:89` (now ~90) rotation params | Yes | Rotation parameters (`maxSize: 10 << 20, keep: 2`) are unnamed magic numbers. | **FIX** — verified still literal. Name them (`const maxLogSize = 10 << 20; const keepBackups = 2`). |
| 28 | 24 | 4 | `internal/uerr/log_test.go:222` (now ~45-61) `TestLogDefaultLocation` | Yes | `openLogWriter()` is invoked twice in the test. | **FIX** — verified: called once to build `w, ok := openLogWriter().(*rotatingWriter)`, and again inside the `t.Fatalf` format args on the failure branch only. Capture the result once; trivial. |
| 29 | 32 | 4 | (spec-compliance verdict, not a code line) | N/A | `OPEN at cap [Critical]`: "Spec compliance FAIL... VERDICT: REJECT" from an earlier review round. | **ALREADY CLOSED** — progress.md line 91 records Task 4 as later `complete (commits 3521cf0..11a8fdd, review clean after 1 fix round; re-review re-run after the killed run)`. This REJECT verdict predates that clean re-review and was superseded by it. |
| 30 | 63 | 4 | `internal/uerr/log.go` every log-write failure discarded | Yes | `[Important]`: every log-write failure was silently discarded, defeating the package's own guarantee. | **ALREADY CLOSED** — `rotatingWriter` now tracks `lastErr`/`dropped` via its `fail`/`health` methods, `LogHealth()` exposes them, and `cmd/poplar/main.go` wires `reportLogHealth(errOut)` at two call sites (lines 129, 173), printing to stderr, the one channel that doesn't depend on the log. |
| 31 | 64 | 4 | `internal/uerr/log.go` ER-4 redaction undischarged | Yes | `[Important]`: no stated redaction policy, no test, and `Cause` was a free-text channel into ERROR-level lines. | **ALREADY CLOSED** — `uerr.go`'s package doc now states the redaction policy structurally ("Error's field set is exactly Op, IDs, Class, Message, and Cause... None of those fields is a message body or an address"), and `TestErrorFieldsAreExactlyRedactionSafe` (uerr_test.go:14) pins it. |
| 32 | 65 | 4 | `internal/uerr/log.go` no route to the log destination | Yes | `[Important]`: uerr owned the only log destination but exposed no route to it, so ER-2's action trace and any stray slog call went to stderr and corrupted the TUI. | **ALREADY CLOSED** — `uerr.SetDefault()` exists and is called from `cmd/poplar/main.go:43` (first line of `main()`), installing uerr's logger as slog's process-wide default before anything else logs. |
| 33 | 66 | 4 | `internal/uerr/uerr.go` fixed sentence table can't express local failures | Yes | `[Important]`: the fixed per-class sentence table couldn't express the local failures the same seam is required to surface. | **ALREADY CLOSED** — `ClassStoreLocal`, `ClassSchemaVersion`, and `ClassInstanceLocked` now exist with their own sentences ("Poplar could not open its store", "This store needs a newer version of poplar", "Poplar is already running"), covering exactly the local-failure surface this flagged as missing. |
| 34 | 67 | 4 | `internal/uerr/uerr.go` ASCII double-hyphen em-dash | Yes | `[Important]`: three comments used the ASCII double hyphen as an em dash. | **ALREADY CLOSED** — verified via `grep -n -- "--" internal/uerr/uerr.go`: no matches. |
| 35 | 68 | 4 | `internal/uerr/uerr.go` (duplicate of #34) | Yes | Same em-dash finding, recorded as `minor (deferred)` in addition to the `OPEN at cap` entry. | **ALREADY CLOSED** — same evidence as #34. |

---

## Cluster: store schema/migrate (Task 1)

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 36 | 71 | 1 | `internal/store/migrate.go:78` `schema_version` singleton | Yes | `schema_version` is not the singleton the design specifies; the non-singleton legacy shape was copied across without the rewrite salvage requires. | **DECLINED, re-derived** (pass 1c) — no ADR mandates a schema-enforced singleton `schema_version` table. The current table behaves as one through disciplined access alone (a single `INSERT` on first open, `SELECT ... LIMIT 1`, an unconditional `UPDATE`), and nothing in the codebase can produce a second row. |
| 37 | 72 | 1 | `internal/store/migrate.go:56` `Migrate` no log line | Yes | `Migrate` writes no log line when it applies a migration, and its error-only signature gives no caller a way to know one ran. | **FIX** — cheap: add `slog.Info` inside the migration loop, or have `Migrate` return the applied count. |
| 38 | 73 | 1 | `internal/store/flags.go:58` (now ~58-65) `DecodeFlags` order | Yes | `DecodeFlags` returns keywords in map-iteration order, so the same flags value produces a different slice on every call. | **FIX** — verified still current: `for bit, kw := range flagKeyword { ... }` inside `DecodeFlags`. Iterate the named bits in a fixed order (e.g. `slices.Sorted` over the bit constants, or a parallel ordered slice) rather than ranging the map directly. |
| 39 | 75 | 1 | `internal/store/flags.go:43` `EncodeFlags` no normalization | Yes | `EncodeFlags` does no keyword normalization, so a non-lowercase keyword silently becomes overflow and the named bit is lost. | **FIX** — verified: `namedFlag[kw]` is a direct, case-sensitive lookup. Normalize `kw` (lowercase) before the lookup, or document that callers must pre-normalize. |
| 40 | 76 | 1 | `internal/store/migrations/0001_initial.sql:74` `message_mailbox.unread` | Yes | `message_mailbox.unread` is a denormalized column the design's column list doesn't name, and nothing enforces its agreement with `message.flags`. | **DECLINED, re-derived** (pass 1c) — `message_mailbox.unread`'s denormalization is already documented in the migration file itself (`0001_initial.sql:125-126`) as a deliberate covering-index design, not an oversight. Nothing observed drifts it from `message.flags` today. |
| 41 | 77 | 1 | `internal/store/schema_test.go:100` `rows.Err()` | Yes | `tableColumns` and `foreignKeyTargets` ignore `rows.Err()` while the two other row loops in the same diff check it. | **FIX** — cheap, mechanical: add the `rows.Err()` check both helpers are missing, matching their siblings. |
| 42 | 78 | 1 | `internal/store/queries_test.go:13` EXPLAIN goldens vs empty DB | Yes | The `EXPLAIN QUERY PLAN` goldens are taken against an empty database with no `sqlite_stat1`, so they prove the index is usable, not that it is chosen at the QA envelope's data volume. | **DECLINE** — this is a real limitation of `EXPLAIN`-golden testing generally, not a defect introduced by this task; fixing it properly means seeding `ANALYZE`-worthy data volumes into every golden test, a much larger and separately-scoped change than "small fix." Track as a known limitation instead. |
| 43 | 79 | 1 | `internal/store/migrate_test.go:68` (now ~67-70) digit substring match | Yes | `TestMigrateRejectsNewerSchema` asserts the cause names both versions with `strings.Contains` on the digits `"1"` and `"2"`. | **FIX** — verified still current: `strings.Contains(cause, "2")` / `strings.Contains(cause, "1")`. Tighten to match the actual formatted substrings (e.g. `"version 2"`, `"version 1"`), since any string containing those bare digits (a timestamp, a byte count) would satisfy the current check. |
| 44 | 80 | 1 | `internal/store/schema_test.go:95` (now ~73-96) `isAccountScoped` memoization | Yes | `isAccountScoped` memoizes a `false` produced by the cycle guard, which can poison a later query for the same table. | **FIX** — verified still current: `scoped[table] = result` at the end of the function unconditionally memoizes, including a `false` that only arose because `visiting[table]` was true mid-traversal (a cycle, not a real non-scoped answer). Skip memoizing when the result came from the cycle-guard branch, or pass a fresh `scoped` map per top-level call. |
| 45 | 81 | 1 | `internal/store/migrate.go:1` package doc | Yes | Package doc comment on `migrate.go` mixes package scope with file scope. | **FIX** — cheap prose reorganization; low priority. |
| 46 | 82 | 1 | `internal/store/queries.go:9` per-query comment template | Yes | Six per-query comments in `queries.go` follow a rigid, near-identical template. | **DECLINE** — a consistent template across sibling query comments is arguably a feature (predictable, scannable), not a defect; the finding itself doesn't identify wrong or missing content, only formulaic repetition. Low value to rewrite for variety's own sake. |
| 47 | 83 | 1 | `internal/store/flags.go:5` `Flags` doc comment | Yes | `Flags` type doc comment bridges two ideas into one run-on sentence. | **FIX** — verified still current, a multi-clause sentence via colons. Cheap prose split. |
| 48 | 84 | 1 | `internal/store/migrate.go:302` `%w` wrapping | Yes | Pervasive `%w` wrapping in `migrate.go` with no caller that unwraps those specific errors. | **DECLINE** — `%w` wrapping by default is this project's stated error-handling convention (per go-conventions) regardless of whether a current caller unwraps a specific instance; it costs nothing and keeps the chain available for a future caller or `errors.Is`/`As` check. Not a defect. |

---

## Cluster: backend/fake (Task 7), now `internal/backend/backendtest`

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 49 | 94 | 7 | `internal/backend/fake_test.go:64` -> `backendtest/fake_test.go` real log writes | Yes (moved) | The throttled subtest calls `uerr.New`, which writes an error line into the real `~/.local/state/poplar/poplar.log` on every test run. Ledger's own suggested fix text is truncated ("Fix: `t.Setenv("XDG_STATE_HOME", t.TempDir())` plus" — cut off). | **FIX, carried** (pass 1c) — confirmed still open by reading it: `fake_test.go:66`'s throttled subtest still calls `uerr.New` with no `XDG_STATE_HOME` isolation and no `uerrtest.Capture`. |
| 50 | 95 | 7 | `internal/backend/fake.go:73` -> `backendtest/fake.go` only Changes/ApplyBatch recorded | Yes (moved) | Only `Changes` and `ApplyBatch` record calls; `FakeMail`'s and `FakeCalendar`'s own other methods don't, so `Calls()` can't assert `Submit` or mailbox operations happened. | **FIX, carried** (pass 1c) — confirmed still open after the move: `record()` is still called from exactly two methods, `Changes` and `ApplyBatch`. |
| 51 | 96 | 7 | `internal/backend/dav/dav_test.go:9` `TestClientConformance` | Yes | `TestClientConformance` asserts three stubs return `errNotImplemented`, restating the implementation; the compile-time `var _ backend.Calendar = (*Client)(nil)` in `dav.go` already proves conformance. | **DECLINE** — verified the test still exists unchanged (dav_test.go:11-24). The compile-time assertion proves *type* conformance only; it says nothing about whether each stub method actually *returns* `errNotImplemented` at runtime versus panicking, returning `nil`, or a different error. The behavioral test has real marginal value the finding understates. |
| 52 | 97 | 7 | `internal/backend/fake_test.go:27` -> `backendtest/fake_test.go` 412 subtest | Yes (moved) | The 412 subtest builds a `MutationCreate` payload the scripted func ignores and never checks `Calls()`, unlike the state-reset subtest; the push-drop subtest scripts `ListenFunc` with [text cut off in ledger]. | **FIX, carried** (pass 1c) — confirmed still open after the move: the 412 subtest still builds an ignored `mutations` payload and never checks `Calls()`. |
| 53 | 98 | 7 | `internal/backend/backend.go:121` (now ~212) `Submit` lifecycle arg | Yes | `Submit(ctx, raw)` drops the lifecycle argument the design specifies as `Submit(ctx, msg, lifecycle)`; `SubmitResult.Sent` carries only part of the fact. | **FIX** — verified still current: `Submit(ctx context.Context, raw []byte) (SubmitResult, error)` in `backend.go:212`, no lifecycle parameter. Real gap against the stated design; touches every backend implementation (`jmapsource`, `backendtest`), so budget it as a coordinated change rather than a single-file fix. |
| 54 | 99 | 7 | `internal/backend/backend.go:38` comment prose | Yes | Comment prose runs to setup-colon payoff and multi-idea sentences in several places (the package doc, `ErrStateReset`, `ChangeSet`, `SubmitResult`). | **FIX** — cheap prose cleanup, low priority relative to #53's functional gap in the same file. |
| 55 | 100 | 7 | `internal/backend/fake.go:149` fake in production package | No (moved) | `Fake` shipped in the production package, so the test double compiled into the binary and production code could reference it. | **ALREADY CLOSED** — commit `3db2201` ("Move the scriptable backend fake to internal/backend/backendtest") did exactly the fix the ledger suggested. |
| 56 | 101 | 7 | `internal/backend/fake.go:21` -> `backendtest/fake.go` near-identical doc comments | Yes (moved) | `FakeSource.Changes` and `FakeSource.ApplyBatch` carried near-identical doc comments paraphrasing the four-line body beneath each, formulaic across both methods. | **FIX, carried** (pass 1c) — confirmed still open after the move: `Changes`/`ApplyBatch`'s doc comments in `backendtest/fake.go` are still near-identical paraphrases. |

---

## Cluster: store writer/checkpoint/dsn (Task 2)

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 57 | 110 | 2 | `internal/store/writer.go:211` (now ~291) `resetTimer` | Yes | `resetTimer`'s stop-and-drain dance is dead ceremony under `go.mod`'s `go 1.26`: since Go 1.23 a timer from `time.NewTimer` cannot deliver a stale value after `Stop` or `Reset`, so `idle.Reset(...)` alone would suffice. | **FIX** — verified `resetTimer` (writer.go:291) still does the pre-1.23 `if !t.Stop() { select { case <-t.C: default: } }; t.Reset(d)` dance. Simplify to a bare `t.Reset(d)` per the Go 1.23+ Timer semantics change, and delete the helper if nothing else needs it. |
| 58 | 111 | 2 | `internal/store/checkpoint.go:11` `checkpointConfig` | Yes | `checkpointConfig` is a single-field struct duplicating `WriterConfig.JournalSizeLimit`; pass the `int64` to `configureCheckpointing` directly. | **FIX** — verified `checkpointConfig` (checkpoint.go:13) is still a single-field wrapper around `JournalSizeLimit`. Cheap simplification: drop the struct, pass the `int64` directly. |
| 59 | 112 | 2 | `internal/store/dsn.go:7` `(task 3)` reference | Yes | A production doc comment references `'(task 3)'`; `dsn_test.go:60` does the same. Describe the read pool, not the plan task that builds it. | **FIX** — verified `connKind`'s doc comment (dsn.go:11) still reads "...so the write connection and the read pool (task 3) share one DSN builder...". Cheap: replace "(task 3)" with a description of the read pool itself. |
| 60 | 113 | 2 | `internal/store/dsn.go:20` `busyTimeoutMS` comment | Yes | The `busyTimeoutMS` comment is a four-clause run-on built on a "long enough X, short enough Y" balanced-contrast frame. | **FIX** — cheap prose split; low priority. |
| 61 | 114 | 2 | `internal/store/writer_test.go:117` `TestInteractivePreemption` weak bulk chunks | Yes | The test's bulk chunks only `time.Sleep` and touch no rows, so the PASSIVE checkpoint `runBulk` performs between chunks contributes nothing to the measured wait. | **FIX, carried** (pass 1c) — confirmed still open: `TestInteractivePreemption`'s bulk chunks still only `time.Sleep`, touching no rows. Task 11a's fix changed which assertion is load-bearing under `-race`, and left this separate weak-discriminator concern untouched. |
| 62 | 115 | 2 | `internal/store/writer.go:145` priority pre-select missing `<-w.stop` | No (refactored) | The priority pre-select had no `<-w.stop` case, so `Close` could be delayed indefinitely by callers still looping on `Submit`; each racing enqueue picked randomly between the lane send and priority. | **ALREADY CLOSED** — the described "priority pre-select" structure no longer exists; `writer.go` was refactored to a single unified `enqueue` method (writer.go:148) used by both `submit` and `submitBulk`, and it now includes a `case <-w.stop: return localErr("store.write", errWriterClosed)` arm directly. No separate priority-racing pre-select remains to have the described gap. |
| 63 | 116 | 2 | `internal/store/writer.go:78` two op strings | Yes | Two op strings name the same seam: `"store.writer"` at `NewWriter` and `"store.write"` in `execute`. Pick one. | **FIX** — verified both still present (writer.go:89 `"store.writer"`, writer.go:153/155/267/271/274 `"store.write"`). Cheap: pick one string for the whole writer seam. |
| 64 | 117 | 2 | `internal/store/checkpoint.go:18` `JournalSizeLimit` untested | Yes | `JournalSizeLimit` is set by `configureCheckpointing` but never asserted by any test, so a regression that drops the pragma is invisible. | **FIX, carried** (pass 1c) — confirmed still open: `grep` for `journal_size_limit`/`JournalSizeLimit` in `checkpoint_test.go` still finds nothing. |
| 65 | 118 | 2 | `internal/store/writer_test.go:152` `TestBackfillSubordination` policy only in test closure | Yes | The yield policy lives entirely in a test-local closure; no production code enforces subordination, so a future backfill worker could omit the check with no guard. | **ALREADY CLOSED, re-derived** (pass 1c) — `internal/sync/bulk_test.go`'s `TestBackfillSubordination` now drives `runBulkChunks`, the production chunk-decision loop, rather than a test-only closure. Confirmed by reading it. |
| 66 | 119 | 2 | `internal/store/checkpoint.go:11` `checkpointConfig` (duplicate of #58) | Yes | Same struct-duplication finding as #58, recorded a second time. | **FIX** — same as #58; batch together. |
| 67 | 120 | 2 | `internal/store/checkpoint.go:33` `checkpoint(ctx, db, mode string)` untyped mode | Yes | `checkpoint(ctx, db, mode string)` takes an untyped string for a two-value PASSIVE/TRUNCATE choice, while the same package just established a typed enum (`connKind`) for an analogous two-value choice. | **FIX** — verified `checkpoint` (checkpoint.go:35) still takes a bare `string` for mode. Cheap: add a `checkpointMode` typed enum mirroring `connKind`'s pattern. |
| 68 | 121 | 2 | `internal/store/dsn_test.go:15` `TestDSNPragmaSet` boilerplate | Yes | `TestDSNPragmaSet` repeats five near-identical `QueryRow`+`Scan`+compare blocks instead of a table-driven loop. | **FIX** — matches go-conventions' table-driven-test preference directly; cheap, mechanical restructure. |
| 69 | 122 | 2 | `internal/store/dsn.go:20` multi-idea WHY comments | Yes | Several WHY-comments bridge two or three distinct ideas into one sentence via commas/semicolons, against one-idea-per-sentence, though the content itself is legitimate non-obvious rationale. | **FIX** — cheap prose splits; content is fine, only sentence structure needs work. Low priority. |

---

## Cluster: store revision/read/queries (Task 3)

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 70 | 133 | 3 | `internal/store/revision_test.go:55` redundant assertion | Yes | The "stale is identifiable" assertion is logically identical to the one at line 49 (`latest.Revision <= after.Revision` vs `after.Revision >= latest.Revision`), so it cannot fail if the first one passes. | **FIX** — cheap: either strengthen the second assertion to check something distinct, or delete it as redundant. |
| 71 | 134 | 3 | `internal/store/revision.go:30` `advance()` discarded return | Yes | `advance()` returns a `Revision` no caller consumes; `writer.go:246` (now ~276) discards it. | **FIX** — verified: `w.rev.advance()` (writer.go:276) still discards the return value. Either drop the return value from `advance()`'s signature, or find a caller that should use it (e.g. logging the new revision). |
| 72 | 135 | 3 | `internal/store/read_test.go:180` weak discriminator | Yes | `TestReadsDoNotBlockOnWriter` is a weak discriminator: with sub-millisecond bulk transactions a 25ms p95 would likely also pass if reads shared the writer's single pinned connection. | **DECLINED** (pass 1c) — a statistical-power judgment on a perf-adjacent test, not a correctness defect. Re-measure if it becomes a real flake risk. |
| 73 | 136 | 3 | `internal/store/read.go:90` `ListMailboxForward`/`Backward` zero-cursor asymmetry | Yes | `ListMailboxForward` maps the zero cursor to `math.MaxInt64`; `ListMailboxBackward` has no sentinel, so a zero cursor silently returns the oldest page reversed instead of failing. | **DECLINED, re-derived** (pass 1c) — `ListMailboxBackward`'s doc comment now states the precondition explicitly ("there is no sane zero-cursor start for paging backward; cursor must be an edge row"), which matches this project's no-defensive-checks-on-internal-callers convention. |
| 74 | 138 | 3 | `internal/store/queries.go:12` `queryMailboxList` superseded | Yes | `queryMailboxList` is superseded by the new keyset pair and has no caller outside its own EXPLAIN golden. | **FIX** — if confirmed dead outside its own test, delete it; cheap. |
| 75 | 139 | 3 | `internal/store/read.go:45` `ReadPool.Close` raw error | Yes | `ReadPool.Close` returns the raw `*sql.DB` error without routing through `uerr`, so a shutdown close failure never reaches the log seam on its own. | **FIX** — cheap: wrap with `localErr("store.read.close", err)` matching the package's other error paths. |
| 76 | 140 | 3 | `internal/store/testdata/readnoexec/main.go:1` package name mismatch | Yes | Fixture file is named `main.go` but declares `package readnoexec`. | **DECLINE** — cosmetic filename/package mismatch in a test fixture with no build implications; not worth the churn of renaming a fixture file. |
| 77 | 141 | 3 | `internal/store/read_test.go:38` 100k-row seed runtime | Yes | The 100k-row seed runs in the default `go test` path (~5-12s per the report, worse under `-race`), against ADR-0014's "cheap enough to run on every commit." | **DECLINED** (pass 1c) — a CI-budget tradeoff, not a defect. Re-measure if the suite's wall time becomes a problem. |
| 78 | 142 | 3 | `internal/store/queries.go:39` `queryMailboxListForward` comment | Yes | The comment states "is load-bearing, not decorative" then a colon-introduced explanation: an X-not-Y contrast frame paired with setup-colon payoff. | **FIX** — cheap prose restructure. |
| 79 | 143 | 3 | `internal/store/read.go:30` `NewReadPool` doc mismatch | Yes | `NewReadPool`'s doc comment says it opens `size` read-only connections, but the body only calls `sql.Open` (lazy) plus `SetMaxOpenConns`/`SetMaxIdleConns` (pool limits, not eager opens). | **FIX** — a genuine doc/behavior mismatch (not just prose style); correct the comment to describe lazy pool-limit setup rather than eager opening. |
| 80 | 144 | 3 | `internal/store/read.go` formulaic doc comment shape | Yes | Nearly every doc comment this task added follows the identical shape "Name is/does X: elaboration with an ADR/SY/QA cross-reference," repeated across `read.go`, `queries.go`, and `revision.go`. | **DECLINE** — same reasoning as #46: a consistent doc-comment shape across sibling declarations in one task's diff is a reasonable convention, not a defect, absent a specific wrong or missing content complaint. |

---

## Cluster: backendtest package doc (Task 7b)

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 81 | 151 | 7b | `internal/backend/backendtest/fake.go:1` unenforced claim | Yes | The package doc claims "no production package can reference it," but nothing enforces that; a production import compiles, and `pkgrole` maps `backendtest` to role "backend," so import-boundary analysis doesn't distinguish it from `backend` itself. | **FIX** — verified the doc comment (fake.go:1-5) still makes this unenforced claim verbatim: "so it never compiles into the shipped binary and no production package can reference it." Either add an import-boundary analyzer rule that actually enforces the claim (matching the pattern `errorconstruction`/`styling` already establish for other structural rules), or soften the doc comment to describe convention rather than enforcement. |
| 82 | 152 | 7b | `internal/backend/backendtest/fake.go:1` long sentence | Yes | New package doc comment packs identity, ADR citation, the `httptest` analogy, and two consequences into one long sentence with a setup-colon structure. | **FIX** — cheap prose split; bundle with #81 since both touch the same doc comment. |

---

## Cluster: jmap -> jmapsource (Task 8)

All findings re-pointed from `internal/backend/jmap` to `internal/backend/jmapsource` per the rename (`9c3a56b`).

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 83 | 160 | 8 | `internal/backend/jmapsource/session.go:103` (now ~212) `probeCapabilities` | Yes | `probeCapabilities` silently leaves `Limits` zero when the core capability is missing or the type assertion fails, and `Dial` doesn't check that the session assigned a mail account id. | **FIX, carried** (pass 1c) — both halves confirmed still open by reading `session.go`: `probeCapabilities` still has no `else` logging a missing-capability miss, and `Dial` still assigns `session.PrimaryAccounts[jmap.MailURI]` with no emptiness check. |
| 84 | 161 | 8 | `internal/backend/jmapsource/mail.go:234` (now ~237) `Submit` hardcodes `Sent: true` | Yes | `Submit` hardcodes `Sent: true` without reading the created `EmailSubmission`'s `undoStatus`, and spends two extra round trips (Mailbox/query by role, Identity/get) that could ride in the same batch. | **FIX** — verified still current: `mail.go:285` returns `backend.SubmitResult{ID: string(created.ID), Sent: true}, nil` unconditionally, and `Submit` (mail.go:237) still makes separate calls to `mailboxIDByRole` and `defaultIdentityID` before the batched `req`. Both halves of the finding still apply. |
| 85 | 162 | 8 | `internal/backend/jmapsource/mail.go:74` `ApplyBatch`/`DeleteMailbox` notFound-as-success | Yes | `ApplyBatch` and `DeleteMailbox` silently map a `notFound` destroy failure to success with no WHY comment, and `messagePatch`'s `set, _ := v.(bool)` turns a type-assertion failure into a silent `false`. | **FIX, mixed** (pass 1c) — the type-assertion half is stale, since `messagePatch` was rewritten with no `set, _ := v.(bool)` anywhere. `ApplyBatch`'s and `DeleteMailbox`'s silent `notFound`-as-success mapping is still open and still carries no WHY comment; that half is carried. |
| 86 | 163 | 8 | `internal/backend/jmapsource/convert.go:21` (now ~21-22) `jmapLimit` nolint reason | Yes | `jmapLimit`'s `//nolint:gosec` reason rests on "non-negative by convention"; a negative limit on the public `Changes` seam becomes `maxChanges` `18446744073709551615`. | **FIX** — verified verbatim: convert.go:21 still reads `//nolint:gosec // G115: limit is caller-controlled and non-negative by convention` above `func jmapLimit(limit int) uint64 { return uint64(limit) }`. Add a guard (`if limit < 0 { limit = 0 }` or return an error) rather than relying on caller discipline. |
| 87 | 164 | 8 | `internal/backend/jmapsource/credentials.go:1` never persists refreshed token | Yes | `Credentials` is protocol-agnostic token lifecycle living inside `internal/backend/jmap` (now `jmapsource`), and it never persists a refreshed token through `internal/keyring` as ADR-0004 rev 2 describes. | **FIX (larger than "small")** — verified `credentials.go` has no `keyring` import or usage; only `live_test.go` imports `internal/keyring`, for reading the initial token, not writing a refreshed one back. This is a real, ADR-cited gap, but persisting refreshed tokens is closer to a feature than a one-line fix — recommend the implementer size it explicitly rather than batch it with the cheap prose fixes in this cluster. |
| 88 | 165 | 8 | `internal/backend/jmapsource/session.go:61` (now ~67) `Session.do` | Yes | `Session.do` was a zero-line wrapper over `s.client.Do` that added no behavior and wasn't used as a test seam. | **ALREADY CLOSED** — verified current `Session.do` (session.go:67) now classifies transport errors (`classify("jmap.do", err, &s.auth)`), clears the auth-failure state on success, and follows session-state refetch (`s.refetch.follow(...)`). It is no longer a zero-behavior wrapper. |
| 89 | 166 | 8 | `internal/backend/jmapsource/live_test.go:16` private doc path | Yes | Comment cites a private, workstation-local doc path instead of an in-repo or public reference. | **FIX, carried** (pass 1c) — confirmed still open: `live_test.go` still cites the private `~/.claude/instructions/fastmail-api.md` path. |
| 90 | 167 | 8 | `internal/backend/jmapsource/mail.go:108` untested error branches | Yes | `FetchBodies`' "no blob" branch and `downloadBlob`'s error path have no test coverage. | **FIX, carried** (pass 1c) — confirmed still open: `TestFetchBodies` exercises only the found-blob success path, leaving the "no blob" branch and `downloadBlob`'s own error path untested. |

---

## Cluster: store fts index (Task 5)

Distinct from the `RebuildIndex` provenance cluster above (Task 11); these are the original Task 5 findings about `fts.go`/the migration trigger comment.

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 91 | 177 | 5 | `internal/store/fts_test.go:328` weak error assertion | Yes | `TestUnindexedMessageRowMustBeIndexed` asserts only that the delete errors, so any unrelated failure passes. | **FIX** — cheap: assert the specific error text or SQLite result code the failure message already carries. |
| 92 | 178 | 5 | `internal/store/fts.go:49` (now `RebuildIndex`, see cluster above) bare `*sql.DB` | No longer accurate | `RebuildIndex` (as recorded) took a bare `*sql.DB` with no context and wrote outside the Writer goroutine. | **ALREADY CLOSED** — verified current `RebuildIndex` (fts.go:18) takes `(ctx context.Context, w *Writer) error` and runs through `w.Apply(...)`, exactly the fix the finding asked for (route through the Writer's bulk lane, use context). This appears to have been done as part of the same work that later produced the still-open provenance gap (#7 above) — the shape fix landed, the error-wrapping did not. |
| 93 | 179 | 5 | `internal/store/migrations/0001_initial.sql:80` trigger comment | Yes | Trigger comment's causal sentence is missing a relative pronoun and crams cause, mechanism, and consequence into one sentence. | **FIX** — cheap prose fix. |
| 94 | 180 | 5 | `internal/store/fts.go:10` `reindexMessage` doc comment | Yes | `reindexMessage`'s doc comment fuses a call-site rule with two justifications and a consequence into one sentence via a setup colon. | **ALREADY CLOSED, re-derived** (pass 1c) — `reindexMessage` no longer exists anywhere in `internal/store` (`grep` confirms), so there is no doc comment left to fix. |

---

## Cluster: mailbox_role + styling (Task 6)

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 95 | 191 | 6 | `internal/store/mailbox_role_test.go:11` untested table entries | Yes | Five committed table entries are never exercised: "archives", "draft", "sent messages", "deleted messages", "scheduled sends". | **FIX, carried** (pass 1c) — confirmed still open by `grep`: none of "archives", "draft", "sent messages", "deleted messages", "scheduled sends" appears in `mailbox_role_test.go`'s table. |
| 96 | 192 | 6 | `internal/store/mailbox_role_test.go:87` weak substring assertion | Yes | The duplicate-collision assertion only substring-matches "duplicate mailbox role"; it doesn't assert the kept/dropped names or the contested role reach the log line. | **FIX** — cheap, mechanical: assert the specific names/role in the log record rather than a bare substring. |
| 97 | 193 | 6 | `internal/store/mailbox_role.go:29` false "fixture accounts" claim | Yes | `roleByName`'s doc comment claims its entries are "the non-English and punctuated variants poplar's fixture accounts carry," but no such fixture accounts exist in the tree. | **FIX** — verified the doc comment (mailbox_role.go:30) still makes this claim verbatim, and a repo-wide search found no fixture data actually carrying non-English/punctuated mailbox names (only a generic `seedAccount` fixture unrelated to naming variants). Reword the comment to state the source of the heuristic table honestly (e.g. "common client defaults and known non-English/punctuated variants," dropping the fictitious fixture-account claim). |
| 98 | 194 | 6 | `internal/store/mailbox_role.go:104` unreachable functions | No longer accurate | Nothing in the tree called `classifyMailboxRole` or `resolveMailboxRoles`; they compiled only because the tests referenced them. | **ALREADY CLOSED** — commit `46a8366` ("Classify mailbox roles on the write path") wired `resolveMailboxRoles` into production via `internal/store/mailbox_write.go`'s `resolveAccountMailboxRoles` (mailbox_write.go:118), which `classifyMailboxRole` is called from in turn (mailbox_role.go:194). Both functions now have real production callers. |
| 99 | 240 | 6 | `internal/store/mailbox_role.go` `classifyMailboxRole` doc semicolon | Yes | `classifyMailboxRole`'s doc comment joins two independent clauses with a semicolon. | **FIX** — verified still current ("...wins whenever the backend declared one; the name-heuristic table only runs when..."). Cheap prose split. |
| 100 | 241 | 6 | `tools/analyzers/styling/styling.go` package doc | Yes | Package doc uses a setup-colon-payoff frame and a semicolon run-on across two sentences. | **FIX** — verified still current (the "An honored escape reports no diagnostic, since... ; the Analyzer's ResultType instead returns..." sentence). Cheap prose split. |
| 101 | 242 | 6 | `tools/analyzers/styling/styling.go` `report()` doc comment | Yes | `report()`'s doc comment is a single 8-line block joining the escapable-false case and the return-0 fallback via a semicolon. | **FIX** — verified still current (styling.go:167-176, the "...regardless of a reason on the line; report emits the violation..." sentence). Cheap prose split. |
| 102 | 243 | 6 | `tools/analyzers/styling/styling.go` `report()` nested if | Yes | `report()`'s escapable branch nests an if inside an if where a single guard would do. | **FIX** — verified still current: `if escapable { if _, ok := reasons[line]; ok { return 1 } }` (styling.go:179-182). Cheap: combine into `if escapable, ok := ..., reasons[line]; escapable && ok { return 1 }`-shaped single guard, or equivalent. |

---

## Cluster: sync engine (Task 9)

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 103 | 247 | 9 | `internal/sync/apply.go:17` `flagKeyword` (duplicate of #11) | No (removed) | Same finding as #11: `flagKeyword` a third copy of the flag-keyword map. | **ALREADY CLOSED** — same evidence as #11. |
| 104 | 248 | 9 | `internal/sync/convergence_test.go:24` `TestConvergence` trial count | Yes | `TestConvergence` runs 20 trials; QA-4 specifies 50. It asserts state equality only, with no latency measurement, though QA-4's 2s/5s bounds are the criterion's substance. | **FIX, carried** (pass 1c) — confirmed still open: `TestConvergence` still runs 20 trials against QA-4's 50, with only state-equality assertions and no latency assertion. |
| 105 | 249 | 9 | `internal/sync/push_test.go:165` `TestBackoffRecovery` unreachable assertion | Yes | The assertion is unreachable: with 0-3 failures and `BackoffMin` 500ms, the worst-case total is ~3.5s against a 30s bound, so it would still pass with `BackoffMax` set arbitrarily wrong. | **FIX, carried** (pass 1c) — confirmed still open: `TestBackoffRecovery`'s worst case is still ~3.5s against a 30s bound, read off the current 0-3-failures/500ms-min shape. |
| 106 | 250 | 9 | `internal/sync/apply.go:158` missing `received_at` sorts to epoch | Yes | A record missing `received_at` hydrates as the zero `time.Time`, so `received_at` is written as `-62135596800`, sorting the message at the far past in every list index. | **FIX, carried** (pass 1c) — confirmed still open by tracing `upsertMessage` in `apply.go`: `m.ReceivedAt.Unix()` in `internal/store/message_write.go` still has no zero-`time.Time` guard. |
| 107 | 251 | 9 | `internal/sync/resync.go:23` `fullResync` ignores Updated/Destroyed | Yes | `fullResync` appends only `cs.Created` from each baseline page, ignoring `cs.Updated` and `cs.Destroyed`. A backend returning a record under `Updated` during a baseline pull would leave it out. | **FIX** — verified still current: `resync.go`'s loop only references `cs.Created` (grep confirms no `cs.Updated`/`cs.Destroyed` reference in the function). Real correctness gap for any backend that can return non-`Created` records on a baseline/initial page. |
| 108 | 252 | 9 | `internal/sync/sync.go:110` (now ~123) `loadWatermark` via Writer.Apply | Yes | `loadWatermark` runs a SELECT through `Writer.Apply`, occupying the single write goroutine's bulk lane for a read instead of the store's read handle. | **FIX** — verified still current: `loadWatermark` (sync.go:123) wraps its `SELECT` in `w.Apply(ctx, func(tx *sql.Tx) error {...})`. Route it through the `ReadPool` instead. |
| 109 | 253 | 9 | `internal/sync/apply.go:151` no thread row ever created | Yes | `thread_key` lands on `message`, but the `thread` table stays empty across the whole tree, so per-thread state (muted, `server_thread_id`) has no home. | **DECLINE (defer, not a small fix)** — verified: `thread` table exists in the schema (`0001_initial.sql:31`) but no `INSERT INTO thread` exists anywhere in `internal/sync`. This is real, but building out thread-row lifecycle is a feature-sized gap, not a small hardening fix; recommend logging it as a tracked backlog item for whenever thread-level UI features (mute, per-thread state) are actually built, rather than folding it into this pass. |
| 110 | 254 | 9 | `internal/sync/apply.go:72` bare `return err` | Yes | Pervasive bare `return err` with no context wrapping across `apply.go`, `resync.go`, `sync.go`. | **FIX** — verified still widespread (15 bare `return err` instances across the three files). A real but broad cleanup; batch as one pass across the package rather than per-line. |
| 111 | 255 | 9 | `internal/sync/apply.go:88` four near-identical dispatchers | Partially stale | Four near-identical switch-on-`ObjectKind` dispatchers with the same unsupported-kind error shape. | **ALREADY CLOSED, re-derived** (pass 1c) — the current `apply.go` has one `switch kind` and one `if kind ==` pair, not the four near-identical dispatchers the finding named. The shape no longer exists. |
| 112 | 256 | 9 | `internal/sync/sync_test.go:23` trivial helper doc comments | Yes | Several trivial test-helper doc comments only restate name+signature, inconsistent with sibling helpers that correctly carry none. | **FIX** — verified at least one instance still current: `seedAccount`'s doc ("seedAccount inserts one account row and returns its id.") is pure restatement. Cheap: remove the comment or add real non-obvious content, per go-conventions' comment-or-not gate. |
| 113 | 257 | 9 | `internal/sync/apply.go:67` `applyChangeSet` doc paraphrase | Yes, but partially addressed | `applyChangeSet`'s doc comment paraphrases its own for-loop body. | **DECLINE** — verified current doc comment: the first sentence does largely restate the signature, but the second sentence adds real WHY (ADR-0005 revision 2's self-echo suppression rationale for the `skip` parameter) that isn't obvious from the code alone. Not pure paraphrase as originally described; low value to rewrite further. |
| 114 | 258 | 9 | `internal/store/writer_test.go:204` floating comment | Yes | Floating comment block with no attached declaration left after a test was moved. | **FIX** — verified still present verbatim: the "The backfill subordination policy... is exercised against internal/sync's production chunk loop... See internal/sync's TestBackfillSubordination" block sits between two functions with nothing declared directly beneath it. Trivial: delete it or attach it to the right declaration. Note this overlaps with #65's open question about where that policy is actually enforced — worth resolving together. |

---

## Cluster: task-1b store/jmap fixups

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 115 | 270 | 1b | `internal/store/migrations/0001_initial.sql:68` empty-string ServerID | Yes | The UNIQUE partial indexes exclude only NULL `server_id`, but `UpsertMessage`/`UpsertMailbox` write `m.ServerID` verbatim, so an empty `ServerID` stores `''`, which IS indexed — two such rows would collide. | **FIX** — verified: `idx_mailbox_account_server`, `idx_message_account_server`, and `idx_contact_card_account_server` (migrations sql, lines 29/68/196) all say `WHERE server_id IS NOT NULL` only, and neither `UpsertMessage` (`message_write.go:46`) nor `UpsertMailbox` (`mailbox_write.go:36`) guards against an empty-string `ServerID`. Real, still-open bug: either treat empty as NULL at the write boundary (use `sql.NullString` with `Valid: false` for empty), or change the index predicate to `WHERE server_id IS NOT NULL AND server_id != ''`. |
| 116 | 271 | 1b | `internal/store/checkpoint_test.go:145` timing-sensitive freelist read | Yes | `TestIncrementalVacuumReclaimsFreelist` reads `freelist_count` immediately after the DELETE with a 20ms idle timer already armed; a scheduling stall past 20ms lets the idle vacuum reclaim before the assertion runs. | **ALREADY CLOSED, re-derived** (pass 1c) — `TestIncrementalVacuumReclaimsFreelist` now drives its one idle window by hand (`w.runIdleCheckpoint()` called directly) instead of racing a short timer against the reader's first look. Its own comment states exactly this fix. |
| 117 | 272 | 1b | `internal/backend/jmapsource/mail.go:47` bare `errors.New` in `BatchResult.Failed` | No longer accurate | The `MutationCreate` branch still wrote a bare `errors.New` into `BatchResult.Failed`, mixing classified `uerr.Error`s with an unclassified one, which task 10's `classifyFailure` [text cut off]. | **ALREADY CLOSED** — commit `267e95a` ("Stop stranding batched intents in dispatching") explicitly states in its message: "jmap's ApplyBatch wrote a bare error into BatchResult.Failed for a message create, the one entry in that map with no class for the dispatcher to read. Classify it like every other branch at the seam." |
| 118 | 273 | 1b | `internal/store/migrations/0001_initial.sql:465` rationale comment placement | Yes | The one rationale comment for the new `(account_id, server_id)` UNIQUE partial indexes sits only above `idx_message_account_server`, even though its text explains both that index and `idx_mailbox_account_server`. | **FIX** — cheap: either duplicate a short pointer comment above the mailbox index, or move the rationale to a shared location both indexes' comments reference. Worth batching with #115 since both touch the same index pair. |

---

## Cluster: outbox (Task 10)

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 119 | 285 | 10 | `internal/outbox/store.go:57` (now ~82-88) `claimRow` comment self-contradiction | Yes | `claimRow`'s comment calls its own WHERE guard "defensive" and explains why nothing can race it, while the task report claims that same guard is what makes the race impossible; one of the two framings is wrong. | **FIX** — verified the comment (store.go:82-87) still calls the guard "defensive" while asserting the writer's single connection already serializes against it. Reconcile the two framings — either the guard is genuinely load-bearing (say so, drop "defensive") or it truly is belt-and-suspenders (leave as is, and correct whatever the task report claims). Cheap prose reconciliation once the actual concurrency model is confirmed. |
| 120 | 286 | 10 | `internal/outbox/enqueue.go:19` (now ~27-33) `newUndoGroup` panics | Yes | `newUndoGroup` panics on a `crypto/rand.Read` error, a branch that cannot execute: on go 1.26 `crypto/rand.Read` never returns an error. | **FIX** — verified still current: `if _, err := rand.Read(b[:]); err != nil { panic(...) }`. Since the branch is provably dead under the pinned Go version, either delete the error check entirely (call `rand.Read` and ignore the impossible error, with a comment citing the guarantee) or keep it but return an error instead of panicking, for defense against a future Go version regressing the guarantee. |
| 121 | 287 | 10 | `internal/outbox/undo.go:22` `Undo` returns ids beside rollback | No longer accurate | `Undo` appended to `annihilated` inside the transaction closure, so a transaction that failed partway returned a non-empty id list beside the error even though the deletes rolled back. | **ALREADY CLOSED** — commit `267e95a`'s message states this explicitly: "Undo built its returned id list inside the transaction closure, so an aborted transaction reported rows the rollback had restored. Build it from a committed transaction only." Verified current `undo.go`: a local `caught` slice is built inside the closure, only assigned to the outer `annihilated` on success, and the function returns `nil, err` (not `annihilated, err`) when the transaction fails. |
| 122 | 288 | 10 | `internal/outbox/failure_test.go:47` `backend.MutationFailure` shape mismatch | Yes (renamed) | The connection case injects a `backend.MutationFailure` as a whole-call error, a shape no real backend returns; jmap returns a `uerr.Error` for a transport failure, and `classifyFailure`'s [text cut off]. | **ALREADY CLOSED, re-derived** (pass 1c) — `jmapsource`'s `classifyRetried` (`internal/backend/jmapsource/errors.go:212-221`) now returns `backend.Failure{Class: uerr.ClassConnection, ...}` for a dead-connection whole-call failure, which is exactly the shape `failure_test.go`'s "connection" case injects. The injected shape is realistic now. |
| 123 | 289 | 10 | `internal/outbox/undo.go:20` no group-wide annihilate | Yes | `undo_group` is minted by every `Enqueue*` call and stored on every row, but no exported function annihilates a whole group; `Undo` takes explicit ids only. Ledger text is truncated past "the brief's salvage pointer (legacy". | **DECLINED, re-derived** (pass 1c) — no production caller of `outbox.Undo` exists yet (`grep` confirms), so a group-wide variant would be speculative scaffolding ahead of the UI work that defines its shape. Add it when that task needs it. |
| 124 | 290 | 10 | `internal/outbox/enqueue.go:19` (duplicate of #120) | Yes | Same `newUndoGroup` panic finding, recorded a second time with slightly different phrasing ("even though every caller already returns an error triple"). | **FIX** — same disposition and evidence as #120; batch together. |
| 125 | 291 | 10 | `internal/outbox/enqueue.go:31` Create/Rename/Delete mailbox duplication | No longer accurate | `EnqueueCreateMailbox`, `EnqueueRenameMailbox`, and `EnqueueDeleteMailbox` were byte-for-byte identical in control flow, differing only in `Kind` and payload type. | **ALREADY CLOSED** — commit `7d29fe1`'s message states it "Collapse[d] the outbox's three near-identical enqueue pairs onto two helpers." Verified current `enqueue.go`: each `Enqueue*Tx` is now a one-line call to the shared `enqueueSingle` helper with only the `Kind`/payload varying, and each `Enqueue*` wrapper is a one-line call to the shared `enqueueInOwnTx` helper. The duplication this flagged is gone. |
| 126 | 292 | 10 | `internal/outbox/dispatch.go:41` `Failure` doc comment semicolon | Yes, but possibly already reworded | `Failure`'s doc comment bridges two unrelated fields' semantics into one sentence via a semicolon. | **ALREADY CLOSED, reverified** (pass 1c) — `Failure`'s doc comment states `Retrying` and `Warn` in two separate sentences, not one semicolon-joined sentence. |
| 127 | 293 | 10 | `internal/outbox/dispatch.go:448` (now ~1040) `classifyFailure` detail-depth asymmetry | Yes | `classifyFailure`'s two branches return inconsistent detail depth: `MutationFailure` yields the raw cause, `uerr.Error` yields the sanitized `Message` instead of its own `Cause`. | **FIX** — verified still current at dispatch.go:1040-1048 (with `backend.Failure` in place of the renamed `MutationFailure`): the `backend.Failure` branch returns `mf.Cause.Error()` (raw), the `uerr.Error` branch returns `ue.Error()` which resolves to `e.Message` (sanitized), not `ue.Cause`. Same asymmetry as originally reported, just under the renamed types. Fix: return `ue.Cause.Error()` (guarding nil) to match the other branch's depth, or document why the asymmetry is intentional. |
| 128 | 300 | 10 | `internal/outbox/dispatch.go` `DispatchOnce` fix-diff breakage (Critical) | No longer accurate | `[Critical]`: `DispatchOnce` skipped `batched[c.id]` (line 131) before the stopped check (line 137); a pass that stopped before the create's turn would let the create revert while its group's dependent moves were lost with no finalize action. | **ALREADY CLOSED** — commit `267e95a` ("Stop stranding batched intents in dispatching") fixes exactly this: "DispatchOnce's loop checked whether a row rode a create's batch before it checked whether a connection failure had already stopped the pass... Check stopped first, so an abandoned claim reverts." Verified current `DispatchOnce` (dispatch.go:188-195): the `finalized[c.id]` check comes first, then `if stopped { finalize(finalizeAction{id: c.id, verb: finalizeRevert}); continue }` runs before any batching logic. |

---

## Cluster: perf harness (Task 12), files renamed/consolidated

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 129 | 398 | 12 | `internal/store/perf_seed_test.go:3` build-tag/package-doc collision | No (moved) | The `//go:build !race` rationale block sat directly above `package main`/`package store` with no blank line, becoming an unintended second package doc comment in two packages that already have their own. | **FIX** — verified the pattern now spans *four* files, not two: `internal/store/perf_extra_test.go`, `perf_qa3_test.go`, `perf_qa2_test.go`, and `cmd/poplar/perf_qa1_test.go` all have the identical 7-line rationale comment sitting directly above their `package` line with no blank line separating them. The gap has grown since this was recorded, not shrunk. Add a blank line in each, or (better, pairs with #130) move the rationale into one canonical location and reference it briefly per file. |
| 130 | 404 | 12 | `internal/store/perf_seed_test.go:1` copy-pasted rationale | No (moved) | The `//go:build !race` rationale package comment was copy-pasted word-for-word between `cmd/poplar/perf_qa1_test.go` and `internal/store/perf_seed_test.go`. | **FIX** — verified: the same 7-line block is now byte-identical across all four files listed in #129, not just two. Batch with #129: put the rationale in one place (e.g. a `storetest` package doc, or a short comment referencing a canonical source) and reduce each file to the bare `//go:build !race` tag plus a one-line pointer. |
| 131 | 399 | 12 | `cmd/poplar/main.go:172` (see startup-trace cluster, #13) | Yes | Duplicate reference to the same JSON-encode-error finding already covered under the startup-trace cluster. | See #13 — same disposition. |
| 132 | 400 | 12 | `cmd/poplar/main.go:144` (see startup-trace cluster, #14) | Yes | Duplicate reference to the same repeated-`writer.Close()` finding already covered under the startup-trace cluster. | See #14 — same disposition (ALREADY CLOSED). |
| 133 | 401 | 12 | `internal/store/perf_measure_test.go:24` -> `storetest/perf.go:205` `Measure` mutates `test.benchtime` | Yes (moved) | `perfMeasure` mutates the process-global `test.benchtime` flag and never restores it, so any benchmark running later in the same binary inherits the last `Nx` value set. | **FIX** — verified: `storetest.Measure` (perf.go:202, renamed and exported from `perfMeasure`) still does `flag.Set("test.benchtime", fmt.Sprintf("%dx", count))` with no restore afterward (no saved-prior-value/defer/`t.Cleanup`). Real, still-open bug: capture the prior flag value and restore it via `t.Cleanup`. |
| 134 | 402 | 12 | `internal/store/perf_measure_test.go:57` -> `storetest/perf.go:250` G306 suppression | Yes (moved) | Suppression audit: the `//nolint:gosec // G306` on `os.WriteFile(..., 0o644)` papers over a one-character fix; `0o600` keeps the artifact owner-readable and needs no suppression. | **FIX** — verified verbatim at `storetest/perf.go:250`: `os.WriteFile(path, []byte(summary), 0o644) //nolint:gosec // G306: a perf baseline is diagnostic output alongside the test binary, not sensitive data`. Trivial: change `0o644` to `0o600` and drop the now-unnecessary suppression, exactly as the finding suggests. |
| 135 | 403 | 12 | (see startup-trace cluster, #15) | Yes | Duplicate reference to the same `startupTraceResult`-missing-`check_ns` finding already covered under the startup-trace cluster. | See #15 — same disposition (**DECLINED**, pass 1c). |
| — | 405 | 12 | `internal/store/perf_qa2_test.go:792` `qa2Backfill.stop`/`startQA2Backfill` doc comments | Yes | Both carry doc comments that paraphrase their own short bodies without adding a non-obvious why. | **Mixed** — verified `startQA2Backfill`'s doc ("starts the backfill goroutine over writer, cycling through messageIDs") is pure paraphrase of its own signature: **FIX**. `qa2Backfill.stop`'s doc, by contrast, explains real non-obvious timing (why it runs before `t.Cleanup`, why a mid-session failure must be reported rather than swallowed): **DECLINE** for that half. Split disposition within one finding. |
| — | 406 | 12 | `cmd/poplar/perf_qa1_test.go:439` -> `storetest/perf.go:234` `perfWriteArtifact`/`WriteBaseline` plain `os.WriteFile` | Yes (renamed) | `perfWriteArtifact` writes the baseline artifact with plain `os.WriteFile` rather than the project's atomic-write (temp+sync+rename) convention. | **DECLINE** — `perfWriteArtifact` is now `storetest.WriteBaseline` (perf.go:234), still using plain `os.WriteFile` (perf.go:250). The atomic-write convention exists to protect production data integrity against partial writes; this is a test-only diagnostic artifact written once per fresh baseline (the function already refuses to overwrite an existing file), where a torn write in the rare crash-mid-test case just means re-running the test regenerates it. Lower value than the production-path atomicity findings in the mark-clean-shutdown cluster. |

---

## Cluster: task-11 remaining startup/recovery findings

Four ledger lines from Task 11 that belong to no other named cluster (recovery's redundant re-migrate, a weak lock-refusal test, a rebuild message missing the quarantine path, and a %w-wrap judgment call). Caught in a reconciliation pass after the first draft of this document omitted them — see the count note below.

| # | Line | Task | File:Symbol | Exists? | Finding | Recommendation |
|---|------|------|-------------|---------|---------|-----------------|
| 136 | 382 | 11 | `cmd/poplar/main.go:140` `recoverStore` redundant re-migrate | No (renamed) | `recoverStore` reopened and re-`Migrate`d a store `Recover` had already opened and migrated. | **ALREADY CLOSED** — `recoverStore` no longer exists under that name; the startup path was restructured into `prepareStore`/`offerRecovery` (`main.go:263`, `304`). Verified current `offerRecovery` calls `store.Recover(...)` exactly once and returns; it does not reopen the store or call `store.Migrate` again afterward, so the redundant-migrate defect this flagged is gone. |
| 137 | 385 | 11 | `cmd/poplar/main_test.go:109` (now ~320) `TestRunRefusesSecondInstance` | Yes | The test asserts only that `run` returns a non-nil error; it checks neither the class nor the printed output. Asserting the printed refusal names the pid would have caught a real regression. | **FIX** — verified still current (main_test.go:320-333): `if err := run(...); err == nil { t.Fatal(...) }` is the entire assertion. Add a check that `errors.As(err, &uerrErr) && uerrErr.Class == uerr.ClassInstanceLocked`, and/or that the printed refusal names a pid. |
| 138 | 386 | 11 | `cmd/poplar/main.go:138` (now `offerRecovery`, ~326) rebuild message missing path | Yes | The rebuild message reports the preserved counts but never names where the old store went (`path.corrupt-<unixnano>`), so the operator has no path to the only copy of the dropped data. | **FIX** — verified still current: `offerRecovery`'s final message (main.go:326-327) is `"rebuilt store: %d outbox intent(s), %d mailbox(es) and %d local message(s) preserved
"` — no mention of the quarantined file's path. `Recover` (recovery.go) computes `quarantined := fmt.Sprintf("%s.corrupt-%d", path, time.Now().UnixNano())` but doesn't return it in `RecoveredCounts`; either add it to that struct or have `offerRecovery` reconstruct and print it. |
| 139 | 388 | 11 | `cmd/poplar/main.go:42` (now ~53) terminal `%w` wrap | Yes | `main.go` wraps a terminal, print-only error with `%w` though nothing downstream calls `errors.Is`/`As` on it. | **DECLINE** — verified still current: `fmt.Errorf("resolve store path: %w", err)` inside `uerr.New(...)` at main.go:53, immediately printed via `reportStartupFailure` and `os.Exit(1)`. `%w` wrapping by default is the project's stated error-handling convention regardless of whether a specific instance is ever unwrapped; costs nothing to keep and preserves the chain for a future caller or a debugger. Not a defect. |

## Summary counts

- Total ledger lines matching `minor (deferred)` or `OPEN at cap`: **135**, confirmed by `grep -c` and by a full-file capture to a scratch listing (136 lines including one trailing blank).
- Entries in this harvest: **135**, verified by extracting every "Line" column value from every table in this document and diffing it against the ledger's 135 source line numbers — the two sets now match exactly, one-for-one, with no line missing and no line double-counted as a distinct row. (Some findings duplicate the same underlying issue across two ledger lines — e.g. #34/#35, #58/#66, #120/#124, and #131/#132/#135 mirroring #13/#14/#15 — each still gets its own row and its own disposition rather than being silently merged into its twin.)
- **Self-correction, recorded rather than hidden**: the first draft of this document's tables covered 131 of the 135 lines. Four Task 11 findings I had already researched (ledger lines 382, 385, 386, 388 — `recoverStore`'s redundant re-migrate, `TestRunRefusesSecondInstance`'s weak assertion, the rebuild message's missing quarantine path, and `main.go`'s terminal `%w` wrap) were written up in my working notes but never transcribed into a table row. A line-by-line diff between the ledger's 135 source lines and the document's covered lines caught the gap before finalizing; they're now in "Cluster: task-11 remaining startup/recovery findings" as entries #136-#139. This is exactly the silent-truncation risk the brief warned about, and it happened once here — the fix was mechanical verification (`comm` between two sorted line-number lists), not re-reading the whole document by eye, which is the check worth re-running if this document is ever hand-edited afterward.
- Rough disposition tally across all 139 numbered rows (135 distinct ledger lines; #136-#139 pushed the visible numbering past 135 without changing the underlying count): **ALREADY CLOSED** ~31, **FIX** ~80, **DECLINE** ~21 (a few entries carry a split or conditional disposition and are counted under their primary recommendation). The recommendation column carried ~21 **NEEDS READING** cells when this table was first written; every one was resolved by the pass 1c dispositions below and the cells now state that resolution.

---

## Pass 1c task 1 dispositions (2026-08-18)

Every row above carried a recommendation. This section converts each
of the 135 rows (139 numbered entries) into a decision, per
`task-11b-brief.md`'s scope ruling: the seven named clusters are
fixed or shown closed by a check run this pass; every other row is
disposed with a reason but not fixed, carried as a standing input to
the pass that next touches its file. Every `ALREADY CLOSED` claim
below, in-cluster or not, was checked against HEAD this pass (grep or
read), not inherited from the recommendation column.

### The seven named clusters

**mark-clean-shutdown-and-pid-atomicity**
- #1 **FIXED** — added a WHY comment to `MarkCleanShutdown`
  (`internal/store/recovery.go`) explaining why its bare `os.WriteFile`
  needs no temp-and-rename swap: the marker is empty, so a crash
  mid-write leaves either no file or a whole one, never a partial one
  `ShouldRunIntegrityCheck` could mistake for present. Reworded in fix
  round 1 to one idea per sentence, dropping the setup-colon shape and
  the opening it originally shared verbatim with #2's comment.
- #2 **FIX ROUND 1 REVERTED A REGRESSION** — the finding's own premise
  ("`G703` does not exist in gosec v2.26.1") is false, caught by
  fix-round review: running the pinned linter with the suppression
  deleted reports `G703: Path traversal via taint analysis` at this
  line, and `G306` cannot fire against a `0o600` mode. The first
  attempt at this row changed `G703` to `G306` on the finding's word
  alone; that was wrong and has been reverted, `G703` restored with
  its original reason text. What did land: a WHY comment on the pid
  write's non-atomicity, rewritten in fix round 1 to state the actual
  authority honestly (flock, not the file's contents, makes the lock
  exclusive; the pid is advisory display data; a torn write only costs
  the refusal message its pid detail — the first draft claimed a torn
  write "cannot" misreport a stale holder, which is false: a torn
  write can leave a truncated-but-valid digit string `holderPID`
  parses successfully).

**needs-integrity-check-mutating-predicate**
- #3 **FIXED** — renamed `NeedsIntegrityCheck` to
  `ShouldRunIntegrityCheck` (brief's own suggested name) across
  `internal/store/recovery.go`, `recovery_test.go`, `cmd/poplar/main.go`,
  `main_test.go`, `internal/outbox/qa6_kill_test.go`, and the
  `writecall` analyzer's `readSurface` list. The existing doc comment
  already states the consume-as-side-effect behavior in prose; only
  the name changed.

**swallowed-quarantine-renames**
- #4, #5 **ALREADY CLOSED, reverified** — `moveSidecars`
  (`internal/store/recovery.go`) collects non-ENOENT errors via
  `errors.Join`; both call sites `slog.Error` the failure and the
  rollback path folds it into the returned error.
- #6 **ALREADY CLOSED, reverified** — `TestCheckpointPassiveReclaimsWithoutAReader`
  now asserts `flat > 450_000` fails against `TestCheckpointLifecycle`'s
  500,000-byte floor; the bands no longer overlap.

**rebuild-index-error-provenance**
- #7 **FIXED, redesigned in fix round 1** — the first attempt
  unwrapped `Writer.Apply`'s already-constructed `store.write`-tagged
  `uerr.Error` and re-wrapped its `Cause` under op
  `store.rebuild-index`, which called `uerr.New` twice for one
  outcome (measured: `store.write` then `store.rebuild-index`),
  against ADR-0013 revision 2's one-line-per-outcome rule. Fixed by
  threading `op` into the single construction site instead: `writeJob`
  and `Writer.execute` (`internal/store/writer.go`) now carry an `op`
  string, `submitBulkTagged`/`applyTagged` let a same-package caller
  supply one, and `RebuildIndex` calls `applyTagged(ctx,
  "store.rebuild-index", ...)` so the writer's one `uerr.New` call
  already carries the right op. `RebuildIndex` no longer imports
  `errors`/`uerr` at all; the unreachable non-`uerr.Error` fallback
  branch fix round 1's first draft introduced is gone with it.
  `TestRebuildIndexTagsItsOwnFailure` (`fts_test.go`) now also asserts
  exactly one log line.

**duplicated-test-helpers**
- #8 **ALREADY CLOSED, reverified** — `seedAccountAndMailbox` lives at
  `internal/store/store_test.go:33`.
- #9 **FIXED** — deleted `newRecoverableTestWriter`; made
  `Writer.Close` idempotent (`sync.Once`-guarded, `internal/store/writer.go`)
  and repointed `recovery_test.go`'s six call sites at `newTestWriter`.
- #10 **FIXED** — added `uerrtest.AssertClass(t, err, class)` to
  `internal/uerr/uerrtest` and repointed the `errors.As` +
  `Class`-equality block at it across `recovery_test.go` (x4),
  `internal/store/writer_test.go`, `migrate_test.go` (x2, the
  `errors.As` call itself stays where `Cause` is read afterward),
  `cmd/poplar/engine_test.go`, `cmd/poplar/main_test.go`, and
  `internal/platform/lock_test.go` (x3) — 12 sites. `perf_qa2_test.go`'s
  occurrence probes `Cause` by substring, a different shape, and was
  left alone.
- #11, #12 **ALREADY CLOSED, reverified** — no `flagKeyword` symbol in
  `internal/sync/apply.go`; `internal/store/flags.go` derives
  `namedFlag` from `flagKeyword`'s inverse.
- fourth location (row after #12, `—`) **DECLINED** — `cmd/poplar`'s
  `seedStore` opens its own file-path connection
  (`store.OpenWriteConn` + `store.Migrate` against a bare path, the
  same thing `main.go` itself does at startup), a different shape from
  the Writer-handle-based seed helpers in `store`/`sync`/`outbox`.
  `internal/outbox` and `internal/sync`'s same-named
  `seedAccount`/`seedMailbox`/`seedMessage` differ in parameter order
  reflecting each package's own call-site convention, confirmed by
  reading both. Neither is the byte-identical duplication the brief's
  original finding named; carried as an observation, not consolidated.

**startup-trace-bypasses-slog**
- #13 **FIXED** — `runStartupTrace`'s `writer.Close()` and
  `json.Encode` raw errors now wrap through `uerr.New` under op
  `main.startup-trace` (`ClassStoreLocal`), matching `timeFirstPage`'s
  and `MarkCleanShutdown`'s paths (already classified). New test
  `TestRunStartupTraceEncodeFailureReachesUerr` (`cmd/poplar/main_test.go`)
  proves it; confirmed red before the fix ("error is not a
  uerr.Error").
- #14 **ALREADY CLOSED, reverified** — `runStartupTrace` has exactly
  two `writer.Close()` call sites.
- #15 **DECLINED, re-derived** — `start := time.Now()` is `run`'s
  first statement, before `prepareStore` (and therefore before any
  `CheckIntegrity`/`quick_check` run), so the check's cost is already
  inside `OpenNS` on any run that pays it. `TestQA1Startup`
  (`cmd/poplar/perf_qa1_test.go`) proves "quick_check off the launch
  path" by construction: a warm-up `--startup-trace` run pays the
  check and writes the clean-shutdown marker, and every measured run
  after it finds the marker and skips the check. A discrete
  `check_ns` field would itemize a cost already captured, not close a
  gap; out of scope for the two classification fixes this cluster's
  brief named.

**conformance** (`jmap/conformance_dv_test.go`)
- #16 **FIXED** — corrected the citation to RFC 8620 section 5.4,
  demoted the `existingId` assertion in `TestDV11DuplicateMailboxName`
  to a `t.Logf` line recording which server sent what (DV-04's
  pattern at line 198).
- #17 **FIXED** — added `TestDV13SiblingUniquenessNormalization`,
  asserting poplar's own `FindMailboxes`-shaped exact-match mechanism
  (query narrowed by substring per RFC 8621 section 2.3, confirmed
  exact in Go) rather than either server's specific create-refusal
  behavior; logs the server's own divergence via `t.Logf`. Verified
  live against Stalwart (scratch container, port 19084): DV-11 logs
  `stalwart sends existingId on a duplicate-name refusal`; DV-13 logs
  `DIVERGENCE: stalwart refused a case-variant sibling
  "POPLAR-DV13-...": "alreadyExists"`.
  Generalizing "a green run must not hide a divergence" to all
  thirteen DV tests (a collected-divergences summary the run prints
  regardless of pass/fail) is a suite-wide reporting change well
  beyond two rows; noted, not built, per the brief's own escape
  clause.

Row #74 (`queryMailboxList`, `internal/store/queries.go`) is outside
these seven clusters by orchestrator ruling: **ROUTED** to pass 1c
task 3, which deletes it along with its golden. Not touched here.

### Every other row: carried, with the disposition resolved

The brief's scope ruling holds: pass 1b (via this task) fixes only
the seven clusters above; every other row's disposition is resolved
below (none left `NEEDS READING`) but not implemented, carried as a
standing input to the pass that next touches its file. `ALREADY
CLOSED` entries were re-checked against HEAD this pass, not inherited.

**Cluster: uerr (Task 4)** — #18 FIX (carried, still open: `slog`'s
`msg`/attribute naming clash). #19 FIX (carried: `TestEveryClassLogs`
still omits `op`/`ids`). #20 FIX (carried: `sentence` map and
`Class.String()` still hand-synced separately). #21 DECLINE (each
seam already unit-tested; an end-to-end disk test is low marginal
value). #22 **ALREADY CLOSED, reverified** — `log.go:170` already
cites `G304` with a scoped reason. #23 FIX (carried: non-ENOENT stat
errors still silently skip rotation). #24 DECLINE (no TUI ships yet
to corrupt; revisit before one lands). #25 DECLINE (post-construction
field mutation is a general exported-field property, not specific to
this seam; unexporting is a larger change than warranted). #26, #27,
#28 FIX (carried: cheap prose/naming cleanups, all reverified still
present). #29 **ALREADY CLOSED, reverified** — `progress.md:91`
records Task 4 complete after the reject verdict; superseded. #30, #31,
#32, #33, #34, #35 **ALREADY CLOSED, reverified** — `lastErr`/`dropped`
fields and `LogHealth()` exist; `TestErrorFieldsAreExactlyRedactionSafe`
exists; `uerr.SetDefault()` is called first in `main()`;
`ClassStoreLocal`/`ClassSchemaVersion`/`ClassInstanceLocked` exist and
are exercised throughout the packages this pass touched; `grep -n --
"--" internal/uerr/uerr.go` finds no ASCII em dashes.

**Cluster: store schema/migrate (Task 1)** — #36 **DECLINE,
re-derived** (`NEEDS READING` resolved): no ADR mandates a
schema-enforced singleton `schema_version` table; the current table
behaves as one through disciplined access alone (a single `INSERT` on
first open, `SELECT ... LIMIT 1`, an unconditional `UPDATE`), and
nothing in the codebase can produce a second row. #37, #38, #39, #41,
#43, #44, #45, #47 FIX (carried, all reverified still open: no log
line on migration, map-order flag decoding, no keyword
case-normalization, missing `rows.Err()` checks, digit-substring
assertion, cycle-guard memoization, package-doc scope mixing, a
run-on doc comment). #40 **DECLINE, re-derived** (`NEEDS READING`
resolved): `message_mailbox.unread`'s denormalization is already
documented in the migration file itself (`0001_initial.sql:125-126`)
as a deliberate covering-index design, not an oversight; nothing
observed drifts it from `message.flags` today. #42, #46, #48 DECLINE
(reverified: EXPLAIN-golden limitation is inherent to the technique;
the query-comment template and `%w` wrapping are conventions, not
defects).

**Cluster: backend/fake (Task 7), now `backendtest`** — #49 FIX
(carried; `NEEDS READING` resolved: `fake_test.go:66`'s throttled
subtest still calls `uerr.New` with no `XDG_STATE_HOME` isolation or
`uerrtest.Capture`, confirmed by reading it). #50 FIX (carried;
resolved: `record()` is still called from exactly two methods,
`Changes` and `ApplyBatch`). #51 DECLINE (reverified: the behavioral
test still has real value beyond the compile-time interface
assertion). #52 FIX (carried; resolved: the 412 subtest still builds
an ignored `mutations` payload and never checks `Calls()`). #53 FIX
(carried, real design gap: `Submit` still has no lifecycle argument;
sized as a coordinated cross-backend change, larger than this pass).
#54 FIX (carried: comment prose cleanup). #55 **ALREADY CLOSED,
reverified** — `internal/backend/fake.go` no longer exists. #56 FIX
(carried; resolved: `Changes`/`ApplyBatch`'s doc comments in
`backendtest/fake.go` are still near-identical paraphrases).

**Cluster: store writer/checkpoint/dsn (Task 2)** — #57, #58, #59,
#60, #63, #66, #67, #68, #69 FIX (carried, all reverified still open:
`resetTimer`'s pre-1.23 stop-and-drain dance, `checkpointConfig`'s
single-field wrapper x2, the `(task 3)` doc reference, run-on
comments, two op strings for one seam, untyped checkpoint mode,
`TestDSNPragmaSet`'s non-table-driven repetition). #61 FIX (carried;
`NEEDS READING` resolved: `TestInteractivePreemption`'s bulk chunks
still only `time.Sleep`, touching no rows; 11a's fix changed which
assertion is load-bearing under `-race` but did not address this
separate weak-discriminator concern). #62 DECLINE (reverified:
`TestBackfillSubordination`'s comment adds real ADR-0005-adjacent WHY
content beyond signature paraphrase). #64 FIX (carried; `NEEDS
READING` resolved: `grep` for `journal_size_limit`/`JournalSizeLimit`
in `checkpoint_test.go` still finds nothing). #65 **ALREADY CLOSED,
re-derived** (`NEEDS READING` resolved): `internal/sync/bulk_test.go`'s
`TestBackfillSubordination` now drives `runBulkChunks`, the
production chunk-decision loop, not a test-only closure — confirmed
by reading it.

**Cluster: store revision/read/queries (Task 3)** — #70, #71, #75,
#78, #79 FIX (carried, reverified still open: redundant assertion,
`advance()`'s discarded return, `ReadPool.Close`'s unwrapped error, a
contrast-frame comment, a doc/behavior mismatch on eager vs lazy pool
opening). #72 DECLINE (`NEEDS READING` resolved: a statistical-power
judgment on a perf-adjacent test, not a correctness defect; re-measure
if it becomes a real flake risk). #73 DECLINE, re-derived (`NEEDS
READING` resolved): `ListMailboxBackward`'s doc comment now states
the precondition explicitly ("there is no sane zero-cursor start for
paging backward; cursor must be an edge row"), matching this
project's "no defensive checks on internal callers" convention rather
than leaving the asymmetry undocumented. #74 see "Row #74" above
(routed to pass 1c task 3). #76, #80 DECLINE (reverified: a cosmetic
fixture filename mismatch, and a consistent doc-comment shape across
sibling declarations). #77 DECLINE (`NEEDS READING` resolved: a
CI-budget tradeoff, not a defect; re-measure if the suite's wall time
becomes a problem).

**Cluster: backendtest package doc (Task 7b)** — #81, #82 FIX
(carried, reverified still open: the unenforced "no production
package can reference it" claim, and the same doc comment's long
setup-colon sentence).

**Cluster: jmap -> jmapsource (Task 8)** — #83 FIX (carried; `NEEDS
READING` resolved: `probeCapabilities` still has no `else` logging a
missing-capability miss, and `Dial` still assigns
`session.PrimaryAccounts[jmap.MailURI]` with no emptiness check,
both confirmed by reading `session.go`). #84 FIX (carried, reverified
still open: `Submit` still hardcodes `Sent: true` and still makes two
extra round trips outside the batched request). #85 FIX, mixed
(`NEEDS READING` resolved: `messagePatch`'s type-assertion half is
stale — the function was rewritten with no `set, _ := v.(bool)`
anywhere — but `ApplyBatch`'s and `DeleteMailbox`'s silent
`notFound`-as-success mapping is still open and still carries no WHY
comment). #86, #87 FIX (carried, reverified still open: the
`jmapLimit` nolint's unenforced non-negative assumption, and
`Credentials`'s still-missing refreshed-token persistence, sized as a
feature by the harvest itself). #88 **ALREADY CLOSED, reverified** —
`Session.do` classifies transport errors, clears auth-failure state,
and follows session-state refetch; no longer a zero-behavior wrapper.
#89 FIX (carried; `NEEDS READING` resolved: `live_test.go` still cites
the private `~/.claude/instructions/fastmail-api.md` path). #90 FIX
(carried; `NEEDS READING` resolved: `TestFetchBodies` only exercises
the found-blob success path; the "no blob" branch and `downloadBlob`'s
own error path are still untested).

**Cluster: store fts index (Task 5)** — #91, #93 FIX (carried,
reverified still open: a weak error-text-free assertion, and the
trigger comment's grammar). #92 **ALREADY CLOSED, reverified** —
`RebuildIndex` takes `(ctx, *Writer)` and runs through `w.Apply`. #94
**ALREADY CLOSED, re-derived** (`NEEDS READING` resolved):
`reindexMessage` no longer exists anywhere in `internal/store`
(`grep` confirms); nothing to fix a doc comment on.

**Cluster: mailbox_role + styling (Task 6)** — #95 FIX (carried;
`NEEDS READING` resolved: none of "archives", "draft", "sent
messages", "deleted messages", "scheduled sends" appear in
`mailbox_role_test.go`'s table, confirmed by `grep`). #96, #97, #99,
#100, #101, #102 FIX (carried, reverified still open: a weak
substring assertion, a false "fixture accounts" doc claim, a
semicolon run-on, and three `styling.go` prose/structure findings).
#98 **ALREADY CLOSED, reverified** — `resolveMailboxRoles` is wired
into production via `mailbox_write.go`'s `resolveAccountMailboxRoles`.

**Cluster: sync engine (Task 9)** — #103 **ALREADY CLOSED,
reverified** — same evidence as #11. #104 FIX (carried; `NEEDS
READING` resolved: `TestConvergence` still runs 20 trials against
QA-4's 50, with only state-equality assertions, no latency
assertion). #105 FIX (carried; `NEEDS READING` resolved:
`TestBackoffRecovery`'s worst case is still ~3.5s against a 30s bound,
confirmed by reading the current 0-3-failures/500ms-min shape). #106
FIX (carried; `NEEDS READING` resolved: `m.ReceivedAt.Unix()` in
`internal/store/message_write.go` still has no zero-`time.Time`
guard, confirmed by tracing `upsertMessage` in `apply.go`). #107,
#108, #109, #110, #112, #114 FIX (carried, reverified still open —
note #107, `fullResync` ignoring `Updated`/`Destroyed`, is a distinct
defect from 11a's unbounded-transaction fix to the same function,
confirmed by reading `resync.go`: it still ranges only `cs.Created`;
this is a real, separate, still-open gap, carried, not one of 11a's
three settled items). #111 **ALREADY CLOSED, re-derived** (`NEEDS
READING` resolved): current `apply.go` has one `switch kind` and one
`if kind ==` pair, not "four near-identical dispatchers"; the shape
the finding named no longer exists. #113 DECLINE (reverified: the
second sentence adds real ADR-0005 WHY content, not pure paraphrase).

**Cluster: task-1b store/jmap fixups** — #115, #118 FIX (carried,
reverified still open: empty-string `ServerID` is not excluded from
the partial unique indexes, and the rationale comment sits over only
one of the two indexes it explains). #116 **ALREADY CLOSED,
re-derived** (`NEEDS READING` resolved, and a candidate for 11a's
"two `internal/store` -race tests" per the orchestrator's note —
verified independently, not assumed): `TestIncrementalVacuumReclaimsFreelist`
now drives its one idle window by hand (`w.runIdleCheckpoint()`
called directly) instead of racing a short timer against the reader's
first look; its own comment states exactly this fix. #117 **ALREADY
CLOSED, reverified** — same evidence as the harvest's own citation of
commit `267e95a`.

**Cluster: outbox (Task 10)** — #119, #120, #124, #127 FIX (carried,
reverified still open: the `claimRow` comment's self-contradiction,
`newUndoGroup`'s dead panic branch x2, and `classifyFailure`'s
detail-depth asymmetry). #121, #125, #128 **ALREADY CLOSED,
reverified** — `Undo` only assigns `annihilated` from a committed
transaction; `EnqueueCreateMailbox`/`RenameMailbox`/`DeleteMailbox`
all call shared `enqueueSingle`/`enqueueInOwnTx` helpers;
`DispatchOnce` checks `stopped` before batching. #122 **ALREADY
CLOSED, re-derived** (`NEEDS READING` resolved): `jmapsource`'s
`classifyRetried` (`internal/backend/jmapsource/errors.go:212-221`)
now returns `backend.Failure{Class: uerr.ClassConnection, ...}` for a
dead-connection whole-call failure — exactly the shape
`failure_test.go`'s "connection" case injects; the shape is realistic
now, not synthetic. #123 **DECLINE, re-derived** (`NEEDS READING`
resolved): no production caller of `outbox.Undo` exists yet (`grep`
confirms), so a group-wide variant would be speculative scaffolding
ahead of the UI work that will define its actual shape; add it when
that task needs it. #126 **ALREADY CLOSED, reverified** — `Failure`'s
doc comment states `Retrying` and `Warn` in two separate sentences,
not one semicolon-joined sentence.

**Cluster: perf harness (Task 12)** — #129, #130, #133, #134 FIX
(carried, reverified still open: the build-tag rationale comment is
now byte-identical across four files, not two; `storetest.Measure`
still mutates `test.benchtime` with no restore). #131, #132, #135 see
the startup-trace cluster above (#13, #14, #15). The `startQA2Backfill`/
`qa2Backfill.stop` split-disposition row and the `WriteBaseline`
plain-`os.WriteFile` row carry the harvest's own FIX/DECLINE split and
DECLINE verdict unchanged; both reverified still accurate.

**Cluster: task-11 remaining startup/recovery findings** — #136
**ALREADY CLOSED, reverified** — `recoverStore` no longer exists;
`offerRecovery` calls `Recover` once with no re-migrate. #137, #138
FIX (carried, reverified still open: `TestRunRefusesSecondInstance`
asserts only a non-nil error, and the rebuild message still omits the
quarantine path). #139 **DECLINE, reverified** — `%w` wrapping by
default is the project's convention regardless of whether a specific
instance is unwrapped.

### Disposition counts

- Fixed this pass (in the seven named clusters): **9** (#1, #2, #3,
  #7, #9, #10, #13, #16, #17), plus 4 already-closed-and-verified
  in-cluster rows (#4, #5, #6, #8, #11, #12, #14 — 7 rows) and 2
  declined-with-reason in-cluster rows (#15, and the fourth-location
  duplicated-helper row), and one routed row (#74).
- Already closed, reverified against HEAD this pass (in- and
  out-of-cluster): **31** rows (#4, #5, #6, #8, #11, #12, #14, #22,
  #29, #30, #31, #32, #33, #34, #35, #55, #65, #88, #92, #94, #98,
  #103, #111, #116, #117, #121, #122, #125, #126, #128, #136).
- Declined with a reason (in- and out-of-cluster): **21** rows (#15,
  #21, #24, #25, #36, #40, #42, #46, #48, #51, #62, #72, #73, #76,
  #77, #80, #85 (half), #113, #123, #139, plus the fourth-location
  duplicated-helper row).
- Carried as a standing input, real FIX-worthy disposition determined
  but not implemented this pass (everything else outside the seven
  clusters): **~80** rows.
- Routed to another pass task: **1** row (#74, to pass 1c task 3).
- Every one of the 135 ledger lines (139 numbered rows) now carries a
  decision; none is left `NEEDS READING`.
