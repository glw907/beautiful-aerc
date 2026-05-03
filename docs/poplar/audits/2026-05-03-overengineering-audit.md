# Pass 8.5 Overengineering Audit — Triage

Aggregate of Phase B (eight per-package agents) and Phase C
(cross-cutting re-read). Each finding has an apply/skip decision.

## Skip-rationale guard

Reused verbatim from /simplify (CLAUDE.md pre-beta posture). The
only valid skip rationales are:
1. Speculative future consumer with a named, scheduled pass on
   STATUS.md (not "Pass N might want this").
2. Upstream-blocked.
3. Premature optimization without measurement (efficiency only).

Forbidden skip rationales: "cross-package," "schema change,"
"would require interface change," "churn cost," "out of scope,"
"non-trivial refactor."

## Findings

### internal/theme/ + internal/term/ + internal/backoff/

| File:line | Finding | Action | Rationale |
|---|---|---|---|
| theme/palette.go:17,38,74,144 + themes.go | dead `FgBrightest` slot | apply | delete (3) |
| theme/palette.go:27,41,82,83 + themes.go | dead `ColorInfo`, `ColorSpecial` slots | apply | delete (3) |
| theme/palette.go:126-166 | `PaletteHex` has only test callers | apply | delete; rewrite tests against direct fields |
| term/probe.go:35-41 | `measureSPUACells` one-call wrapper | apply | inline into `MeasureSPUACells` |
| term/probe_test.go:34 | `(*fakeTerminal).run` unused `t` param | apply | drop param |
| term/probe_test.go:73-90 | `intToStr` one-call helper | apply | replace with `strconv.Itoa` |
| backoff/* | nothing found | n/a | three genuine consumers, 30 LOC |

### internal/mailjmap/

| File:line | Finding | Action | Rationale |
|---|---|---|---|
| jmap.go:45 | `current` field write-only | apply | delete |
| jmap.go:590 | `firstInReplyTo` one-call helper | apply | inline |
| errors.go:40 | `wrapAuth` one-call helper | apply | inline |
| errors.go:41 | `wrapNotFound` one-call helper | apply | inline |
| jmap.go:864 | `checkEmailSetCreated` one-call helper | apply | inline into `Copy` |
| jmap.go:750 | `checkEmailSetDestroyed` one-call helper | apply | inline into `Destroy` |
| jmap.go:635 | stale "Pass 6" comment | apply | delete comment |

### internal/mailimap/

| File:line | Finding | Action | Rationale |
|---|---|---|---|
| auth.go:32 | `dialCommand` one-line wrapper | apply | inline |
| auth.go:38 | `dialIdle` one-line wrapper | apply | inline |
| idle.go:143 | `handleUnilateral` one-call method | apply | inline; pass `b.emit` directly |
| changes.go:57 | `cmdClient` one-call accessor | apply | inline |
| client.go:62 | `listEntry.HasChildren` write-only | apply | delete |
| imap.go:111 | `finishConnect(ctx)` discards ctx | apply | thread caller's ctx into idleLoop OR drop param; choose the threading variant for correctness |
| imap.go:48 | `capSet.XGM` write-only | apply | inline check; delete field |
| fake_test.go:118 | `stringReader` duplicates `strings.NewReader` | apply | replace with stdlib |

### internal/mail/

| File:line | Finding | Action | Rationale |
|---|---|---|---|
| types.go:14 | `FlagRecent` no production consumers | apply | delete; rewrite jmap test using a different unsupported flag |
| mock.go:29 | `MockMoveCall` Lazy Element | apply | inline as anonymous struct |
| mock.go:35 | `MockFlagCall` Lazy Element | apply | inline as anonymous struct |
| mock_test.go:165 | `equalUIDs` one-call helper | apply | inline |
| backend.go:44 | `Search` vestigial on interface | apply | remove from interface; backends keep concrete method (delete if also unused) |
| backend.go:46 | `Copy` vestigial on interface | apply | remove from interface; mailimap keeps internal `copyUIDs`; delete from mailjmap if unused |

### internal/cache/

| File:line | Finding | Action | Rationale |
|---|---|---|---|
| account.go:28-36 | `Cache` + `NewCache` unused aggregator | apply | delete both; drops `mu` field too |
| account.go:279 | `errClosed` unused | apply | delete |
| ops.go:34-43 | `SendArgs`/`AppendArgs` reserved sum | skip | (1) named scheduled pass — Pass 8.4c per STATUS.md |
| drainer.go:260-263 | `backoffFor` one-call wrapper | apply | inline |
| account.go:179-192 | `AccountName`/`AccountEmail` Middle Man | keep | nil-backend CLI introspection path is load-bearing |
| drainer.go:197-200 | `mark` one-call helper | apply | inline UPDATE |
| account.go:243-255 | `expandHome` one-call helper | apply | inline into `DBPath` |

### internal/config/

| File:line | Finding | Action | Rationale |
|---|---|---|---|
| account.go:22-25 | `Headers`, `HeadersExclude`, `CheckMail` always zero | apply | delete |
| account.go:29 | `Aliases` always zero | apply | delete |
| accounts.go:41-47 | `ParseAccounts` test-only path wrapper | apply | delete; rewrite tests with `os.ReadFile` + `ParseAccountsFromBytes` |
| providers.go:9 | `Provider.Name` redundant with map key | apply | delete field |
| providers.go:17-18 | `Provider.AuthHint`, `HelpURL` no production reads | apply | delete fields; remove from preset table |
| loader.go:14-19 | `Source` exported enum, callers discard | apply | refactor to internal bool; drop second return of `Resolve` |

### internal/ui/

| File:line | Finding | Action | Rationale |
|---|---|---|---|
| help_popover.go:181 | `lipgloss.NewStyle()` outside permitted files | apply | move to `Styles.HelpBoxBorder` |
| cmds.go:278 | `looksLikeRFC822` one-call helper | apply | inline |
| cmds.go:228 | `extractDisplayText` one-call helper | apply | inline |
| confirm_modal.go:153 | `confirmWrap` one-call helper | apply | inline |
| app.go:416 | `translateConnState` one-call helper | apply | inline |
| help_popover.go:224 | `renderTopEdge` one-call method | apply | inline |
| viewer.go:360 | `viewerViewportKeymap` one-call helper | apply | inline |
| linkpicker.go:275 | `parseLinkKey` one-call helper | apply | inline |

### cmd/poplar/

| File:line | Finding | Action | Rationale |
|---|---|---|---|
| cache.go:72 | `gatherStats` one-call helper | apply | inline into RunE |
| config_discover_folders.go:85 | `openBackendForDiscoverFolders` one-call wrapper | apply | inline |
| diagnose.go:28 | `runDiagnose` one-call helper | apply | inline into RunE |
| cache.go:335 | `fileSize` returns discarded error | apply | inline `os.Stat(...).Size()` with zero-fallback |
| config_cmd.go:23,54,72 | `--config` flag ignored by 3 subcommands | apply | thread flag through |
| config_discover_folders.go:16 | `configDiscoverFoldersFlags` single-field struct | apply | replace with bare bool var |

### Cross-cutting

| Cite | Finding | Action | Rationale |
|---|---|---|---|
| internal/config/loader.go ↔ cmd/poplar/* | `Source` cascade into cmd consumers | apply | applied with config Source refactor |
| internal/theme/palette.go ↔ themes.go | palette deletions cascade across 15 themes | apply | applied with theme deletions |
| internal/mail/backend.go ↔ mailjmap/mailimap/mock | interface trim cascade | apply | applied with mail interface refactor |

## Summary

- Total findings: 47 actionable + 2 keep + 1 nothing-found
- Apply: 45
- Skip (speculative consumer with named pass): 1 (cache OpArgs)
- Skip (upstream-blocked): 0
- Skip (no-measurement): 0
- Keep (informational, real consumer cited): 1 (cache AccountName/AccountEmail nil-guard)
