# Human-voice audit — aggregated findings

Phase 1 output. Read the per-package files for verbatim excerpts and
diagnoses; this file is the index + tally + by-category roll-up that
drives Phase 2 triage.

## Source files

| File | Findings |
|------|---------:|
| [`2026-05-04-pkg-cmd-poplar.md`](2026-05-04-pkg-cmd-poplar.md) | 12 |
| [`2026-05-04-pkg-cache.md`](2026-05-04-pkg-cache.md)           | 20 |
| [`2026-05-04-pkg-mail.md`](2026-05-04-pkg-mail.md)             | 19 |
| [`2026-05-04-pkg-mailjmap.md`](2026-05-04-pkg-mailjmap.md)     | 22 |
| [`2026-05-04-pkg-mailimap.md`](2026-05-04-pkg-mailimap.md)     | 18 |
| [`2026-05-04-pkg-config.md`](2026-05-04-pkg-config.md)         | 17 |
| [`2026-05-04-pkg-leaves.md`](2026-05-04-pkg-leaves.md)         | 22 |
| [`2026-05-04-pkg-ui.md`](2026-05-04-pkg-ui.md)                 | 23 |
| [`2026-05-04-cross-cutting.md`](2026-05-04-cross-cutting.md)   | 10 |
| **Total** | **163** |

## Aggregate tally by category

| Category | Tells | Findings |
|----------|-------|---------:|
| C1 — Comment rot                | T1 / T2 / T5     | 39 |
| C2 — Defensive cruft            | T22 / T23        | 3  |
| C3 — Premature abstraction      | T20 / T21        | 0  |
| C4 — Uniform verbosity          | T3 / T7 / T17 / T31 | 39 |
| C5 — Naming tells               | T14 / T15 / T18  | 4  |
| C6 — Test boilerplate           | T9 / T24         | 32 |
| C7 — Error phrasing             | T10 / T10b / T11 / T13 | 26 |
| C8 — Structural symmetry        | T19              | 10 |
| **Cross-cutting** (overlap with above) | — | 10 |

C3 came up empty: every interface-shaped seam has a real second
implementation (`mail.Backend`, `mail.ChangeTracker`,
`internal/ui/URLOpener` test fake, `internal/ui/Editor` planned
compose seam, `imapClient` real+fake). Calibration confirms.

## Routing for Phase 2 → Phase 3

Per the plan's routing rule:

- **Pass 8.8 Phase 3a (string-only):** C1 (T1/T2/T5), C7 (T10/T10b/T11/T13), C4-prose subset (T3/T7/T17/T31 where the fix is comment shape only, not function shape).
- **Pass 8.9 Phase 3b (structural):** C2 (T22/T23), C5 (T14/T15/T18), C6 (T9/T24 — renames + structure changes), C8 (T19), C4-structural (T31 cross-file file-layout choruses, `<thing>.go` collapses).

C4 splits along the prose-vs-structure line; Phase 2 triage marks each
finding with its target pass.

## By-category roll-up

For each category below, the (package · tell · finding-count) breakdown
points back into the per-package files where the verbatim excerpts live.

### C1 — Comment rot (39 findings)

- **cmd-poplar** (T2): `loadAccounts` / `formatThousands` / `newConfigCmd` / 3× `newXxxCmd` constructors.
- **cache** (T2 + T5): `Name`/`Dir`/`Events` accessors restate signature; `migrations` slice doc tautological; `SendArgs`/`AppendArgs` "reserved for Pass 9"; `AccountName` architectural narration.
- **mail** (T1 + T2): `MockBackend.ListFolders`/`OpenFolder`/`Updates` restate body.
- **mailjmap** (T1 + T2 + T5): four translation helpers (`translateEmail`, `translateKeywords`, `idsToUIDs`, `keywordForFlag`); `formatFromList` doc; `Connect`'s six `--- Phase N ---` banner labels; `push.go` line-restating comments; `Send` "Pass 9" / "pass 3" labels; `fake_test.go` "Tasks 10–14" reference.
- **mailimap** (T1 + T2 + T5): `imapUID`, `mailUIDsToSet`, `attrsToStrings` signature restatement; `client.go` interface preamble describing development workflow.
- **config** (T2 + T5): `knownProvidersList`, `isShellName`, `suggestProvider`; `Template` golden-file framing.
- **leaves** (T2): `theme.Themes` map doc; `theme.ThemeNames` doc; `term.fcListFamilies` first-two-sentences; `content.markerFor`; `content.metadataPrefixWidth`; `tidy.countChangedLines`; `filter.layoutTablePlugin.Init`.
- **ui** (T2 + T5): `cacheEventMsg` doc; `updateTab` doc; `renderBlankLine`; `deriveChromeFromAcct` third-sentence task framing; `openFolderCmd`/`refreshFolderCmd` "used by …" clauses.

### C2 — Defensive cruft (3 findings)

- **cmd-poplar** (T22): redundant `db.Ping()` after `cache.OpenDB` in `statsForAccount` and `runVacuum`.
- **ui** (T22): `account_tab.go:593–596` `page != nil` check on internally-maintained map; `account_tab.go:628–630` redundant `!ok || page == nil` double-guard.

### C3 — Premature abstraction (0 findings)

Every potential single-impl interface has a real second consumer or an ADR'd seam. No applies; no tasks.

### C4 — Uniform verbosity (39 findings)

- **cmd-poplar** (T3 / T7 / T11): three `newXxxCmd` chorus; five `newXxxCmd`/`runXxx` chorus in `cache.go`; six gerund errors in `discover-folders` RunE; five gerund errors in `writeAtomically`.
- **cache** (T3 / T7 / T17 / T31): `ops.go` five-type chorus; `schema.go` five-`migrate vN` meta-structure chorus; `(*Account)` eight-method getter bank; `Config` roadmap-voice doc; `bodies.go`/`attachments.go` cross-file noun-swap clones.
- **mail** (T3 / T7 / T17 / T31): `Attachments`/`FetchAttachment` identical-stub pair; `FetchBody`/`mockBodies` content duplication; commented-vs-uncommented mock method asymmetry; `Backend` interface bubbletea-rationale narration; `ConfigKey`/`Attachments` `should` hedges; `ErrNotFound` second-person voice.
- **mailjmap** (T3 / T7 / T31): 15-method `"X satisfies mail.Backend."` opener chorus; `push.go` unexported function bank uniform density; `Updates` getter overweight godoc.
- **mailimap** (T3 / T7 / T17 / T31): three-function `FetchBody`/`FetchBodyStructure`/`FetchBodyPart` doc + body chorus; `(*Backend)` getter bank uniform comment weight; `imapClient` interface 12-method same-shape comments; `idle_test`/`attachments_test` per-test docstring chorus.
- **config** (T3 / T7 / T17 / T31): `resolveEnv`/`parseSize`/`LoadUI` uniform doc weight across complexity tiers; `LookupProvider`/`DefaultCacheConfig`/`ErrFirstRun` SVO opener chorus; `PasswordCmd`/`Email` field hedging; `TrashRetentionDays`/`SpamRetentionDays` identical-shape pair.
- **leaves** (T7 / T31): `theme.themes.go` 15-var chorus (single most visually obvious AI fingerprint).
- **ui** (T3 / T7 / T17 / T31): `sidebar.go` six-method getter bank; `viewer.go`, `status_bar.go`, `account_tab.go` accessor banks; `outbox_overlay.go`/`conflict_overlay.go`/`confirm_modal.go` six-method lifecycle chorus; `sidebar_column.go` accessor-vs-`SetSize` density inversion; `app.go` ErrorMsg/folderLoadedMsg multi-paragraph narration; `selectionChangedCmds` historical-alternative narration.

### C5 — Naming tells (4 findings)

- **cmd-poplar** (T18): `parseEvictDuration` (caller context already supplies `evict`).
- **config** (T18): `LookupProvider`, `DefaultCacheConfig`, `ExistingFolderKeys`, `ExpandHome`.

Routing: 3b. Renames ripple; tmux render after.

### C6 — Test boilerplate (32 findings)

- **cmd-poplar** (T9 / T24): predicate-form test names; `TestCacheStats_OutputFormat` four-sentence in-test-body docstring.
- **cache** (T9): pervasive verbal-predicate suffixes (`_ResetsAttemptsAndStatus`, `_EvictsBySizeWhenOverCap`, `_IdempotentResetsToPending`, `_ReAnchorPromotesOutboxToConflict`, `_SyncerSkipsUIFlagsOnPendingOp`); `TestQueueOp_Atomicity_FlagAppliesOptimistic`; `integration_test.go` two multi-line docstrings.
- **mail** (T9 / T24): five sentence-form `t.Run` names; uniform `"expected at least one X"` assertion template.
- **mailjmap** (T9 / T24): seven "TestX verifies that…" docstrings; `"unknown folder returns error"` / `"happy path returns UIDs and total"` table cases.
- **mailimap** (T9 / T24): six idle-test docstrings + four attachment-test docstrings; uniform `"got = X, want Y"` assertion template across six test files (~35 sites).
- **config** (T9 / T24): `TestLoadUI` sentence-form names; `TestLoadUI_UndoSeconds` predicate-form cases; uniform `"expected error containing %q, got nil"` template across four test functions.
- **leaves** (T9 / T24): five `render_footnote_test.go` test docstrings; five filter-test files share `t.Errorf("got %q, want %q", …)`.
- **ui** (T9): four test files (`sidebar_search_test`, `footer_test`, `account_tab_test`, `app_test`) with sentence/predicate-form `t.Run` cases.

Routing: 3b. Test renames + boilerplate trimming.

### C7 — Error phrasing (26 findings)

- **cmd-poplar** (T10b / T13): `runRoot` 9-error ladder; three `%w` sites whose callers don't branch.
- **cache** (T10 / T10b / T11): `migrate vN` numeric-discriminator chorus; `retry: read status / retry: update` adjacent pair; identical `"expand cache dir"` strings for distinct failures; `fetch headers` / `backend fetch headers` adjacent template.
- **mailjmap** (T10b / T11 / T13): two sibling functions emit identical `"no Email/set response"`; `email/changes:` cross-file chorus across push and changes; identical `move:`/`destroy:`/`set keyword` template for transport-vs-checker errors; five `Connect` `%w` sites with no caller branching.
- **mailimap** (T10 / T10b / T11 / T13): three-function FetchBody/Structure/Part error trio; `actions.go` copy-pasted `store deleted`/`uid expunge` pair across two functions; `password-cmd failed` adjacent pair; reflexive `%w` in internal adapter methods.
- **config** (T10b / T13): six `"reading/parsing X config: %w"` sites; same six errors with `%w` where callers branch only on sentinels.
- **leaves** (T10 / T13): three-error `%w` chorus in `filter.ToHTML`; five-error `%w` chorus in `tidy.CallAPI`.

Routing: 3a (string-only).

### C8 — Structural symmetry (10 findings)

- **cache** (T19): 11 files; `conflicts.go` thin sub-topic; `folders.go` one-function paired oddly with `syncer.go`; `attachments.go` noun-swap clone of `bodies.go`.
- **mailimap** (T19): 12 files; `actions.go` groups four mutations by category; `tlsHint.go` and `errors.go` are skeleton files.
- **mail** (T19 × 2): `types.go` is a catch-all by category; `changes.go` houses `ErrAuth`/`ErrNotFound` despite those being Backend sentinels.
- **config** (T19): 10 files; `account.go` (struct only) + `accounts.go` (parsing) split with no coupling reason; `path.go` 8-line file; `diderror.go` named after UX outcome.
- **term** (T19): dedicated `doc.go` for a five-function package.
- **ui** (T19): `cmds.go` (617 lines) + scattered helper-purpose-per-file (`toast.go`, `error_banner.go`, `dim.go`, `iconwidth.go`, `top_line.go`).

Routing: 3b. File renames/collapses.

### Cross-cutting (10 findings)

See [`2026-05-04-cross-cutting.md`](2026-05-04-cross-cutting.md). The
ten cross-package patterns subsume individual findings but are tracked
separately so they can be applied as coordinated batches in Phase 3.

## Phase 2 next step

Triage walks each finding in this index and the per-package files,
marking `apply 3a` / `apply 3b` / `keep` / `taste-call`. Frozen at end
of Phase 2; Phase 3 lands the apply set.
