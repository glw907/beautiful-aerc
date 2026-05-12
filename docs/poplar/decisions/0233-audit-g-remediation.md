---
title: Audit G remediation
status: accepted
date: 2026-05-15
---

## Context

Pass 40 (ADR-0230) ran Audit G against ~185 test files and queued
21 P1 items, all rooted in the same family: assertions that pass
without proving anything because their underlying fakes return
`nil` unconditionally, or because the expectation derives from
the system under test. Pre-beta endorses the fix; the dominant
shape is mechanical.

## Decision

Twenty-one fixes shipped in Pass 40.1, grouped by repair pattern:

**Per-method error injection on six fakes.** Each fake grew typed
`*Err error` fields consulted on call; existing happy-path
callers continue to leave them zero. New table rows exercise the
error path.

- `internal/mailimap/fake_test.go` — `copyErr`/`moveErr`/
  `expungeErr`/`appendErr` on `fakeClient`. Three new tests in
  `actions_test.go` (`TestMoveSurfacesUIDMoveError`,
  `TestMoveSurfacesCopyFallbackError`,
  `TestDestroySurfacesExpungeError`) assert that errors propagate
  through `Backend.Move`/`Destroy` and short-circuit the
  copy→store→expunge fallback when Copy fails.
- `internal/mailjmap/fake_test.go` — `fakeClient.Do` now returns a
  hard error when `respond` is nil. No existing call site relied
  on the empty-response default (all empty-`fakeClient{}` callers
  in the suite never hit `Do`).
- `internal/mail/mock.go` — `MockBackend` grew `SendCalls`,
  `AppendCalls`, `SendErr`, `AppendErr` plus the named
  `SendCall`/`AppendCall` record types. Two new tests verify
  recording and error injection.
- `internal/ui/account/cmds_test.go::blockingBackend` —
  `moveErr`/`destroyErr`/`flagErr`/`sendErr`/`appendErr` consumed
  by the corresponding pass-throughs.
- `internal/ui/compose/model_test.go::fakeCache` — `createErr`/
  `updateErr`/`loadErr`. Two new tests assert that draft-save
  errors land in `uicore.ErrorMsg{Op: "save draft"}` rather than
  being swallowed.
- `internal/cache/conflicts_test.go::fakeContactsWriter` —
  `multigetErr` + `multigetRet` plus a `multigets` counter, for
  symmetry; no current cache-side caller exercises the field
  beyond drainer ops, so the scaffolding waits for future tests
  rather than ship a vacuous assertion now.

**Assertion tightening on eleven sites.** Each replaces a
trivially-passing or self-derived check with a positive,
verifiable claim:

- `mailauth/oauth_test.go::TestTokenReturnsCachedWhenFresh`
  asserts `tok1 == tok2`.
- `mailimap/idle_test.go::TestIdlePollFallback` counts
  `UpdateFolderInfo` emissions; the production `pollFallback`
  constant became `defaultPollFallback` plus a per-Backend
  `pollInterval` field so the test can drop the cadence to 5 ms
  without a package-level mutable var.
- `mailjmap/jmap_test.go::TestAppend_ImportsToFolder` captures
  the request and asserts `Account`, `BlobID`, `MailboxIDs`,
  and `Keywords["$draft"]`.
- `internal/contacts/client_test.go` gained
  `TestPutAddressObject_Auth` for the 401→`ErrAuth` branch,
  mirroring the existing Delete-side coverage.
- `internal/ui/app_test.go::TestApp_BackendUpdateReArmspump`
  invokes the re-armed Cmd in a goroutine and asserts the
  returned `backendUpdateMsg` shape after pushing one update via
  the new `MockBackend.Emit`. The previous `cmd != nil` check
  only proved that *some* Cmd was returned.
- `internal/catkin/popover_test.go::TestPopoverRendersAtRightShiftedColumn`
  hard-codes the popover's outer width (24 cells) derived from the
  fixed footer line, removing the
  `wantCol := 80 - m.popover.width()` self-reference.
- `internal/ui/reader/attachpicker_test.go::TestAttachPicker_OpenAction`
  walks the returned `tea.BatchMsg` via the existing
  `collectMsgs` helper and asserts both `OpenAttachmentMsg` and
  `AttachPickerClosedMsg` carry the right UID/Att.
- `internal/config/loader_test.go::TestLoadFirstRunWritesTemplate`
  compares the written file to the `template.golden` fixture
  (the same artifact `TestTemplateMatchesGolden` uses) rather
  than to `Template()`'s own return value.
- `cmd/poplar/reauth_test.go` calls `runReauth` directly and
  asserts `errors.Is(err, errUnknownReauthAccount)` /
  `errReauthAccountNotOAuth`. `runReauth` no longer calls
  `os.Exit(78)`. It returns the sentinels and `main` maps them
  to exit codes.
- `internal/wizard/probe_test.go::TestProbeRoutesJMAP` injects an
  SMTP fake that returns an error and asserts the JMAP route
  never invokes it.
- `cmd/poplar/cache_test.go::TestRunEvict_NoMatchingAccount`
  writes a valid IMAP config via the shared
  `loadReauthTestConfig` helper, so the dev-env skip no longer
  fires and the test exercises `runEvict`'s real "account not
  found" branch.

**Lipgloss style attribute assertions** replace
`style.Render("x") != ""` checks:

- `internal/ui/styles_test.go::TestNewStyles` and
  `internal/ui/uicore/list_styles_test.go::TestNewListStyles`
  now assert `GetForeground()` / `GetBackground()` / `GetBold()`
  / `GetItalic()` per row. The pattern survives a passing test
  against a zero-value `lipgloss.Style{}` (which Renders to a
  non-empty string).

**Four real `contacts.Sync` tests** replaced the `t.Skip`
placeholders. The suite stands up an `httptest.NewServer` wrapping
`carddav.Handler` with a minimal `syncTestBackend` implementing
the third-party `carddav.Backend` interface; tests cover first-run
full pull, group-vCard skipping (`KIND:group` drops via
`parseObject`'s `p.Skip`), repeated full-pulls when the server
omits getctag, and the SupportsSync-stays-false case when the
server omits REPORT sync-collection. The `voice-check.sh` T14
rule grew a `scan_excl_tests` carve-out for the inevitable
`GetAddressBook` / `GetAddressObject` methods required by the
third-party interface.

## Consequences

- Audit G P1 queue closed; Audit Final (Pass 41) unblocked.
- Two production-touching changes ride along with the test
  remediation: `mailimap.Backend.pollInterval` (a test-overridable
  field replacing a package constant) and `reauth.go`'s sentinel-
  returning shape. Both are pre-beta-clean refactors; the
  finished tree reads as if poplar were always written this way.
- The `Render("x") != ""` styling-smoke anti-pattern is the
  bubbletea-specific tell ADR-0230 flagged as a new entry for the
  `elm-conventions` skill checklist. The skill update lands in
  this pass alongside the test changes so future passes catch the
  pattern at write time.
- The `contacts.Sync` test infrastructure (`syncTestBackend` +
  fixture) is a reusable seam for any future CardDAV-side
  coverage — multi-book sync, deletion handling, conflict
  recovery — without re-deriving the carddav.Backend bindings.
- One P1 item (G-batch2-2) shipped as scaffolding only:
  `fakeContactsWriter.Multiget` grew error-injection fields, but
  no cache-side caller exercises Multiget today, so a vacuous
  test would have re-introduced the audited smell. The field is
  ready for the first real consumer.
- `voice-check.sh` now distinguishes production code from test
  code for T14 (`Get*` getter prefix). The rule remains hard for
  production callers; tests are free to satisfy third-party
  interfaces that hard-code Get-prefix method names.
- No invariant changes. These are test-correctness fixes, not
  new binding facts. The `Backend.pollInterval` field is an
  implementation detail of `mailimap.idleLoop` and not part of
  the contract.
