---
title: Audit G — test assertion meaningfulness
status: accepted
date: 2026-05-14
---

## Context

Pass 40 ran Audit G per `docs/poplar/audit-plan.md` §"Phase G":
walk every `*_test.go` in `internal/` and `cmd/poplar/` against
the lens — "for each test, what would have to be wrong about the
code under test for the test to fail? If the answer is nothing
realistic, the test is theatre." The bias the lens targets, per
the audit-plan: LLMs reliably write tests that pass syntax but
verify nothing. Coverage rises; correctness doesn't.

Trigger: Audit F remediation (Pass 39.1, ADR-0229) shipped.

Walk dispatched in four parallel batches by package cluster
matching Audit F's shape: mail stack; storage & content; UI
layer; config / CLI / utilities. Five focuses per audit-plan:
trivially-passing assertions, self-derived expectations, unread
goldens, silent-success fakes, missing error-branch coverage.
Per-batch findings live in
`docs/superpowers/archive/plans/2026-05-14-audit-g.md`.

## Decision

Aggregate over the four batches: **0 P0, 21 P1, 17 P2** (38
total findings across 22 packages and ~185 test files).

| Batch | P0 | P1 | P2 |
|-------|----|----|----|
| 1 — mail stack | 0 | 6 | 3 |
| 2 — storage & content | 0 | 4 | 4 |
| 3 — UI layer | 0 | 7 | 8 |
| 4 — config / CLI | 0 | 4 | 2 |
| **total** | **0** | **21** | **17** |

Pass 40.1 lands the 21 P1 items. P2s land in BACKLOG; the golden-
correctness item (G-batch3-8) is structural and gets a one-line
BACKLOG entry rather than a remediation row.

The cluster shape is **silent-success fakes** dominating Focus 4:
`MockBackend.Send`/`Append`, `mailimap.fakeClient` mutation
methods, `mailjmap.fakeClient.Do` default branch,
`blockingBackend` in `account/cmds_test.go`,
`fakeCache.CreateDraft`/`UpdateDraft`,
`fakeContactsWriter.Multiget` — all return `nil` unconditionally,
making every error-path assertion in the suite trivially pass.
Secondary: **trivially-true lipgloss assertions** (Focus 1) —
`style.Render("test") != ""` is true for the zero-value
`lipgloss.Style{}`, so style-smoke tests pass with no styling
applied. Tertiary: **skipped placeholder tests** —
`internal/contacts/sync_test.go` is four `t.Skip` stubs, leaving
the entire 210-line sync engine with zero passing coverage.

**P1 set — queue Pass 40.1 (21 items):**

- **G-batch1-1** — `mailimap.fakeClient.Copy`/`Move`/`UIDExpunge`
  always return nil. Fix: per-method `*Err error` fields injected
  per test.
- **G-batch1-2** — `mailjmap.fakeClient.Do` default returns empty
  `*jmap.Response{}`. Fix: default to a failing response unless
  `respond` is supplied, or make `respond` mandatory.
- **G-batch1-3** — `mail.MockBackend.Send`/`Append` return nil
  with no call recording. Fix: add `SendErr error` and
  `SendCalls []sendCall` fields; existing dev-tag tests update.
- **G-batch1-4** — `TestTokenReturnsCachedWhenFresh` never
  asserts `tok1 == tok2`. Fix: add the equality assertion.
- **G-batch1-5** — `TestIdlePollFallback` has no assertion at
  all. Fix: count STATUS poll invocations on the fake and
  assert > 0.
- **G-batch1-6** — `TestAppend_ImportsToFolder` asserts only
  `err == nil`. Fix: assert request shape (folder ID, blob ID,
  flag) via the fakeClient's request recorder.
- **G-batch2-1** — All four `contacts.Sync` path tests are
  `t.Skip` stubs. Fix: unskip and write real assertions against
  a `webdavtest` fake server (the harness exists at
  `internal/contacts/client_test.go:23`).
- **G-batch2-2** — `fakeContactsWriter.Multiget` returns
  `(nil, nil)`. Fix: per-method error injection.
- **G-batch2-3** — `PutAddressObject` has no `ErrAuth` test.
  Fix: add a 401-response table row mirroring
  `DeleteAddressObject`'s coverage.
- **G-batch3-1** — `account/cmds_test.go::blockingBackend`
  mutation methods all return nil. Fix: per-method `*Err error`
  fields.
- **G-batch3-2** — `compose/model_test.go::fakeCache.CreateDraft`
  /`UpdateDraft` silent-success. Fix: per-method error injection.
- **G-batch3-3** — `TestApp_BackendUpdateReArmspump` asserts only
  `cmd != nil`. Fix: invoke `cmd()` and type-switch on the
  returned msg to confirm it's `pumpUpdatesMsg`.
- **G-batch3-4** — `TestPopoverRendersAtRightShiftedColumn`
  derives `wantCol` from `m.popover.width()` (system under
  test). Fix: hard-code expected column for a fixed input width.
- **G-batch3-5** — `TestNewStyles` `Render("test") != ""`
  passes for `lipgloss.Style{}`. Fix: assert
  `style.GetForeground() == expected` per row.
- **G-batch3-6** — `TestNewListStyles` same pattern. Fix: same
  shape.
- **G-batch3-7** — `TestAttachPicker_OpenAction` asserts only
  `cmd() != nil`. Fix: type-switch on returned msg.
- **G-batch4-1** — `TestLoadFirstRunWritesTemplate` compares
  file content to `Template()` — the same call used to write.
  Fix: compare against the committed `template_golden.toml`
  fixture used by `TestTemplateMatchesGolden`.
- **G-batch4-2** — `cmd/poplar/reauth_test.go` two tests inline
  their own lookup loop instead of calling `runReauth`. Fix:
  call `runReauth` and assert returned error / exit code.
- **G-batch4-3** — `wizard/probe_test.go::TestProbeRoutesJMAP`
  SMTP fake silent-success. Fix: SMTP fake's default returns
  an error; JMAP routing test asserts SMTP fake never called.
- **G-batch4-4** — `cmd/poplar/cache_test.go::TestRunEvict_NoMatchingAccount`
  skips in dev env. Fix: construct an in-memory config with a
  valid account so the skip path doesn't fire.

(G-batch3-5 and G-batch3-6 are the same mechanical pattern but
counted separately because they live in distinct packages and
each needs a discrete fix.)

**P2 — BACKLOG / noted (17 items):**

- G-batch1-7 `TestMockBackendImplementsBackend` compile-time
  interface assertion only. Fold into G-batch1-3 fix.
- G-batch1-8 `TestBackend_DisconnectWithoutConnect` only nil-
  pointer panic can fail it. Leave; the property is real.
- G-batch1-9 `TestResolvedPassword_NonXOAUTH2_Caches` discards
  errors. One-line fix; fold into 40.1 if cheap.
- G-batch2-4 Attachment-progress tests discard `pct` return.
  Leave; `pct` is implementation-zero.
- G-batch2-5 `TestRenderBodyHeading` `!= "Title\n"` tautological
  escape hatch. Tighten to a positive ANSI-prefix assertion.
- G-batch2-6 `TestParseBlocksCorpus` skips on absent
  `e2e/testdata/*.html`. Either commit the corpus or delete
  the test.
- G-batch2-7 `TestCleanHTML` tracking-image case: add the
  negative `!Contains("track.example.com")` assertion.
- G-batch3-8 — 20 golden files in `ui/testdata/goldens/`. The
  golden-correctness problem is inherent to the snapshot
  pattern; mitigated by `UPDATE_GOLDENS=1` discipline and code
  review of the .txt diffs. Structural; no per-file remediation.
- G-batch3-9 `TestNewSpinner` `Contains("x")`. One-line fix.
- G-batch3-10 `TestSearchStyles` `GetForeground() == nil`. Fix
  alongside G-batch3-5.
- G-batch3-11 `TestNewCompiledThemeStyles` content-pass-through
  assertion. Fix alongside G-batch3-5.
- G-batch3-12 / -13 `wizard/model_test.go` pointer-identity +
  Width-storage assertions. Worth one revisit; not load-bearing.
- G-batch3-14 `TestModel_EditAsDraftWithoutDraftIsInert` early
  return on nil cmd. One-line fix.
- G-batch3-15 `TestThemeRegistryComplete` `Render("x") == "x"`
  trivial sub-check. Fold into G-batch3-5.
- G-batch4-5 `TestInstallLogger_*` mutate `slog.Default()`. Save
  + restore in the test, or use a `slog.New` local logger.
- G-batch4-6 `TestProbeRoutesJMAP` doesn't assert SMTP fake
  *not* called. Fold into G-batch4-3 fix.

## Consequences

- Pass 40.1 lands the 21 P1 items before Audit Final (Pass 41).
  The dominant fix pattern is uniform: extend each fake with
  per-method `*Err error` fields consulted on call, and update
  the call sites to set them in error-path table rows. Estimated
  shape: ~6 fakes get the error-injection treatment, ~6 style
  assertions get tightened to `GetForeground()`, ~4 cmd-shape
  assertions get type switches, ~3 test functions get rewritten
  to call the real entry point.
- The silent-success-fake cluster confirms that Audit F's
  finding pattern (errors dropped) and Audit G's (assertions
  that don't assert) share a root: the project's defensive
  posture under-weighted error-path observability. The fixes
  reinforce each other — F-batch2-2's drainer logging is
  meaningless without G-batch3-1's tests that exercise error
  branches.
- The lipgloss-style trivially-true assertion pattern is a
  bubbletea-specific tell worth adding to the
  `elm-conventions` skill's review checklist: never assert
  `Render("x") != ""` as a styling check; assert the style's
  resolved attributes (`GetForeground`, `GetBold`, etc.).
- The `contacts.Sync` four-test placeholder stub is the worst-
  case shape: green CI signal with zero coverage. Worth a
  hookify rule that flags `t.Skip` calls in committed test
  files — TBD in a future pass.
- No P0 findings → soak gate not invalidated. Audit Final
  (Phase Final) proceeds after 40.1.
- Audit-plan §"Phase G" walk strategy returned 38 findings
  across 22 packages — the largest single-phase yield so far,
  expected given the 33k-line test surface vs. Audit F's
  source-code lens. The per-batch cluster sizes (6/4/7/4 P1)
  are roughly uniform; no single batch dominated, which
  validates the four-cluster shape for this codebase.
- Phase Final trigger is "Phase G returns empty"; with 21 P1s
  queued, Phase Final gates on Pass 40.1 landing.
