# Human-voice audit — cross-cutting patterns

Patterns no single-package agent could catch. Each entry names the pattern, the packages where it appears, and the catalogue tell.

---

## CC1 — `"X satisfies mail.Backend."` opener (T3 / T31)

Both backends carry the chorus on every `mail.Backend` method:

- **`internal/mailjmap/`** — 15 methods in `jmap.go` + `attachments.go`, every one opens with `"X satisfies mail.Backend."` regardless of body length (1 line vs 60). `Updates` (one-line getter) and `Connect` (60-line lifecycle) get the same opener.
- **`internal/mailimap/`** — `imap.go` repeats the pattern. `AccountName satisfies mail.Backend.`, `AccountEmail satisfies mail.Backend.`, `Updates satisfies mail.Backend. Returns a nil channel before Connect succeeds.`

The opener carries no information beyond the method's presence in the type — Go's interface-satisfaction is implicit; spelling out the conformance in 30+ doc comments is mechanical. Apply in 8.9: drop the boilerplate sentence and let the second sentence (when present) carry the contract.

---

## CC2 — Reflexive `%w` wrapping (T13)

Every package uses `%w` mechanically, almost never with a downstream `errors.Is`/`errors.As` consumer:

- `cmd/poplar/root.go` — 8 `%w` sites in `runRoot`; only the two `config.Load` sentinel checks branch on a sentinel.
- `internal/config/` — 6 `"reading/parsing X config: %w"` sites; callers branch on `ErrFirstRun`/`ErrOldAccountsToml` only.
- `internal/mailjmap/jmap.go:196–224` — 5 `%w` sites in `Connect`; no caller branches.
- `internal/mailimap/realclient.go` — `store: %w`, `authenticate: %w`, `fetch headers: %w` sites where `classifyErr` already promotes the sentinels.
- `internal/tidy/api.go:56–97` — 5 `%w` sites in `CallAPI`; consumed as a string in `Result.Message`.
- `internal/filter/tohtml.go:18–19` — 3 `%w` sites; pipe consumer never inspects the error type.

Apply in 8.9: where no caller branches on a wrapped sentinel, `%v` is correct; `%w` exposes implementation types as package API for no benefit.

---

## CC3 — Gerund/operation-noun error template chorus (T10b)

Identical `"<verb-ing> <noun>: %w"` template repeats across packages:

- `internal/config/` — `reading accounts config`, `parsing accounts config`, `reading cache config`, `parsing cache config`, `reading ui config`, `parsing ui config`. Six errors across three files in identical shape.
- `cmd/poplar/config_discover_folders.go` — `reading config`, `loading accounts`, `opening backend …`, `connecting backend …`, `listing folders`, `reading existing folder keys`. Six adjacent gerunds in one closure.
- `cmd/poplar/config_discover_folders.go (writeAtomically)` — `creating temp file`, `writing temp file`, `syncing temp file`, `closing temp file`, `renaming temp file`. Five-error chorus with only the verb varying.
- `internal/cache/schema.go` — `migrate v1` through `migrate v5`. Numeric suffix is the only discriminator.

Apply in 8.9: vary verb form, drop redundant prefixes, prefer operation-keyed nouns (`"create config dir"`, `"open db"`, `"add backoff column"`) over `<gerund> <noun>` templates.

---

## CC4 — Per-test "TestX verifies that Y" docstring chorus (T9 / T24)

Test function docstrings in sentence form are pervasive:

- `internal/mailjmap/jmap_test.go` — 7 consecutive tests open with `"TestX verifies that …"`.
- `internal/mailimap/idle_test.go` — 6 tests, same opener.
- `internal/mailimap/attachments_test.go` — 4 tests with `"TestX confirms that …"`.
- `internal/cache/integration_test.go` — `TestIntegration_TriageRoundTrip` and `TestIntegration_CrashRecovery` carry multi-line prose docstrings.
- `internal/content/render_footnote_test.go` — 5 tests with multi-paragraph design-narrative docstrings.

Apply in 8.9: test function names need no docstring per the style guide. Drop the chorus.

---

## CC5 — Sentence-form `t.Run` / table case names (T9)

Subtest and table-case names written as predicates instead of noun-phrase labels:

- `internal/ui/sidebar_search_test.go` — 7 `t.Run` cases with verbs ("shows", "transitions", "emits", "cycles").
- `internal/ui/footer_test.go` — `"account context has X"` × 3 + `"responsive: X drops Y"` × 3.
- `internal/ui/account_tab_test.go` — `"title returns folder name after load"`, `"loading is true after selectionChangedCmds, before headersApplied"`.
- `internal/ui/app_test.go` — `"content height is height minus 3 chrome rows"`.
- `internal/cache/cache_test.go` — `TestQueueOp_Atomicity_FlagAppliesOptimistic`, `TestStoreBody_EvictsBySizeWhenOverCap`, `TestRecoverExecuting_IdempotentResetsToPending`.
- `internal/mail/mock_test.go` — `"flat messages have ThreadID == UID and empty InReplyTo"`, `"connect succeeds"`, `"disconnect succeeds"`.
- `internal/config/ui_test.go` — `"missing [ui] section uses defaults"`, `"below floor clamps to 2"`, `"above ceiling clamps to 30"`.
- `internal/mailjmap/jmap_test.go` — `"unknown folder returns error"`, `"happy path returns UIDs and total"`.

Apply in 8.9: noun phrases. `"idle hint row"`, `"Activate: Idle → Typing"`, `"thread invariants"`, `"size eviction"`, `"clamp below floor"`, `"unknown folder"`.

---

## CC6 — Trivial-accessor metronomic doc weight (T7)

Same-shape one-sentence godoc on every getter regardless of complexity, in every package with multiple exported types:

- `internal/cache/account.go:177–207` — eight-method `(*Account)` bank: `Name is …`, `Dir is …`, `Events returns …`, `DroppedEvents returns …`, `Close stops …`, `AccountName proxies …`, `AccountEmail proxies …`, `DB exposes …`.
- `internal/ui/sidebar.go:72–197` — six methods, six identical-weight comments.
- `internal/ui/viewer.go:68–153` — `IsOpen`, `Links`, `Attachments`, `ScrollPct` all carry single-sentence docs.
- `internal/ui/status_bar.go:54–102` — five Set/Get methods with uniform two-clause docs.
- `internal/ui/account_tab.go:98–659` — `Title`, `Backend`, `Cache`, `ViewerOpen`, `SearchState` carry identical-shape getters.
- `internal/mailjmap/jmap.go:99–131` — `AccountName`/`AccountEmail`/`Updates` getters.
- `internal/mailimap/imap.go:55–76` — same trio.

Apply in 8.9: drop docs on getters whose names + return types fully convey the contract. Keep them where there's a non-obvious invariant (`Updates returns nil before Connect`, `SetSize records dims; doesn't propagate`).

---

## CC7 — Reflexive `<thing>.go` file layout (T19)

Every multi-file package shows the same skeleton: split by category-name rather than coupling.

- `internal/config/` — 10 files for one responsibility (config parsing). `account.go` (struct only) + `accounts.go` (parsing) split with zero coupling reason. `path.go` is one 8-line function. `diderror.go` is two helpers named after the UX outcome rather than the Go concept.
- `internal/cache/` — 11 files. `conflicts.go` hosts a sub-topic too thin for a file. `folders.go` is one function, paired oddly across `syncer.go`. `attachments.go` is a noun-swap clone of `bodies.go`.
- `internal/mailimap/` — 12 files. `actions.go` (Move/Destroy/Flag/Send) groups four mutations purely because they are "writes". `tlsHint.go` (26 lines) and `errors.go` (44 lines) are skeleton files.
- `internal/mail/` — `types.go` is a catch-all by category (UID, Flag, Folder, UpdateType, ConnState, Update, Disposition, Attachment). `changes.go` houses `ErrAuth`/`ErrNotFound` despite those being Backend sentinels, not change-tracker sentinels.
- `internal/term/` — `doc.go` for a five-function package whose package doc could fit on one of the three real files.
- `internal/ui/` — `cmds.go` (617 lines) + `toast.go` + `error_banner.go` + `dim.go` + `iconwidth.go` + `top_line.go` — each helper-purpose-per-file rather than splitting by coupling.

Apply in 8.9: collapse `<thing>.go` files where the split doesn't track coupling. Move `ErrAuth`/`ErrNotFound` out of `changes.go`. Drop `internal/term/doc.go` and inline the package doc.

---

## CC8 — Pass/task label leakage in code (T5)

Project-pass identifiers ("Pass 9", "pass 3", "Tasks 10–14") appear in production godocs and error strings:

- `internal/mailjmap/jmap.go:727` — `// Send satisfies mail.Backend. Compose is planned for Pass 9.` + `errors.New("send not implemented in pass 3 — see pass 9")`.
- `internal/mailjmap/fake_test.go:30–32` — `// Tasks 10–14 use this to inject canned method responses …`.
- `internal/cache/ops.go:33–37` — `// SendArgs is reserved for Pass 9.` × 2.
- `internal/cache/conflicts.go:20–21, 84–85` — `// SendArgs/AppendArgs (Pass 9) return an error …`.

Apply in 8.9: pass identifiers belong in commit messages and ADRs, not in godocs or error strings.

---

## CC9 — Docstring-shaped exported names (T18)

Compound exported names that read like sentence summaries:

- `internal/config/` — `LookupProvider`, `DefaultCacheConfig`, `ExistingFolderKeys`, `ExpandHome`.
- `internal/ui/` — `OpenConfirmEmptyMsg`, `EmptyFolderConfirmedMsg`, `ConfirmModalYesMsg` (Msg names are *expected* to read declaratively — borderline; keep).
- `internal/cache/` — names are largely fine (`QueueOp`, `RetryOp`, `DiscardOp`, `OutboxSummary`).

Apply in 8.9: `LookupProvider` → use `Providers[key]` directly. `DefaultCacheConfig()` → make unexported (`defaultCache`) since only `LoadCache` uses it. `ExistingFolderKeys` → `FolderKeys`. `ExpandHome` is borderline; the function does pass through unchanged paths, so the name over-promises.

---

## CC10 — Hedged narrative in WHY-comments (T17)

Multi-paragraph `"this is intentional because …"` / `"we chose X over Y so that …"` blocks where one terse line would do:

- `internal/cache/account.go:87–91` — `Config` doc with `"Currently the only knobs are …; future fields cover …"` roadmap voice.
- `internal/mail/backend.go:18–21` — bubbletea-rationale sentence in interface doc.
- `internal/mail/changes.go:39–41` — second-person voice in `ErrNotFound` godoc.
- `internal/ui/app.go:294–334` — multi-paragraph case-handling commentary that duplicates ui-invariants.md.
- `internal/ui/account_tab.go:509–529` — five-clause doc on `selectionChangedCmds` with historical-alternative narration.
- `internal/config/account.go:27–61` — struct-field comments with hedged-narrative second clauses.

Apply in 8.9: terse why-line; let the function body and ADRs carry the rest.

---

## Cross-cutting tally

By tell, summed across packages:

| Tell | Count (excerpts/findings) | Dominant packages |
|------|---------------------------|-------------------|
| T2  | ~25 sites | mailjmap, mailimap, cache, config, leaves |
| T3 / T31 | 9 cross-package choruses | mailjmap+mailimap (Backend), ui (overlays), theme (15 vars), cache (Account bank), mock (Attachments pair) |
| T5  | 6 sites | mailjmap, cache, cmd-poplar |
| T7  | 7 cross-package | every multi-method package |
| T9 / T24 | 5 packages affected | mail, mailimap, mailjmap, cache, ui, content, config, filter |
| T10 / T10b / T11 | 8 cross-function choruses | config, cmd-poplar, cache, mailjmap, mailimap, tidy, filter |
| T13 | 7 packages | every error-returning package |
| T17 | 6 sites | cache, mail, ui, config |
| T18 | 4 names | config |
| T19 | 7 packages | every multi-file package |

The signature pattern of poplar-as-AI-shaped is the combination — not any single tell. The Go reads as machine-uniform because every package shows the same proportion of T2/T3/T7/T13/T19. A human-authored codebase would have far more variance in which tells dominate which packages; here every package looks like every other.
