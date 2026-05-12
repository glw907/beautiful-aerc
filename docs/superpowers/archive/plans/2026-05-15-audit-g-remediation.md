# Pass 40.1 — Audit G remediation

Land the 21 P1 items from ADR-0230. Mechanical remediation.

Dominant fix shapes (per ADR-0230 §Decision):
- Per-method `*Err error` injection on six fakes.
- Tighten lipgloss style smoke tests to `GetForeground()`-style
  attribute checks instead of `Render("x") != ""`.
- Type-switch on returned `tea.Msg` instead of `cmd != nil`.
- Unskip the four `contacts.Sync` tests against `webdavtest`.

## Batches

### A — Silent-success fakes (6 fakes)

1. **G-batch1-1** `mailimap/backend_test.go` (and siblings):
   `fakeClient.Copy`/`Move`/`UIDExpunge` grow `*Err error` fields
   consulted on entry; existing callers default to nil.
2. **G-batch1-2** `mailjmap/backend_test.go::fakeClient.Do`: when
   `respond` is nil and no method matched, return a failing
   `*jmap.Response`. Existing callers that relied on the empty
   default must set `respond`.
3. **G-batch1-3** `internal/mail/mock.go`: add
   `SendErr`/`AppendErr error` and `SendCalls []SendCall`,
   `AppendCalls []AppendCall` to `MockBackend`. Update dev-tag
   tests.
4. **G-batch3-1** `ui/account/cmds_test.go::blockingBackend`:
   per-method `*Err error` fields for the mutation methods.
5. **G-batch3-2** `ui/compose/model_test.go::fakeCache`: per-method
   error injection on `CreateDraft`/`UpdateDraft`.
6. **G-batch2-2** `internal/contacts/*` test fake
   `fakeContactsWriter.Multiget`: per-method error injection.

### B — Assertion tightening (11 items)

- **G-batch1-4** `mailauth/cache_test.go::TestTokenReturnsCachedWhenFresh`:
  add `tok1 == tok2` (pointer or AccessToken equality).
- **G-batch1-5** `mailimap/idle_test.go::TestIdlePollFallback`:
  count STATUS poll invocations; assert > 0.
- **G-batch1-6** `mailjmap/backend_test.go::TestAppend_ImportsToFolder`:
  assert recorded request shape (folder ID, blob ID, flag).
- **G-batch2-3** `internal/contacts/client_test.go`: add 401 row
  for `PutAddressObject` mirroring `DeleteAddressObject`.
- **G-batch3-3** `internal/ui/app_test.go::TestApp_BackendUpdateReArmspump`:
  invoke `cmd()`, type-switch on `pumpUpdatesMsg`.
- **G-batch3-4** `helppopover/*` popover-column test: hard-code
  expected column for fixed width.
- **G-batch3-7** `attachpicker/*` open-action test: invoke `cmd()`
  and type-switch.
- **G-batch4-1** `internal/config/loader_test.go::TestLoadFirstRunWritesTemplate`:
  compare against `template_golden.toml`.
- **G-batch4-2** `cmd/poplar/reauth_test.go`: call `runReauth`
  directly; assert returned error/exit code.
- **G-batch4-3** `internal/ui/wizard/probe_test.go::TestProbeRoutesJMAP`:
  SMTP fake's default returns an error; assert SMTP fake never
  called for JMAP routing.
- **G-batch4-4** `cmd/poplar/cache_test.go::TestRunEvict_NoMatchingAccount`:
  construct in-memory config with valid account so skip doesn't
  fire.

### C — Lipgloss style assertions (2 items)

- **G-batch3-5** `internal/ui/styles_test.go::TestNewStyles`:
  assert `GetForeground()` per row (and other resolved attrs
  where meaningful).
- **G-batch3-6** sibling `TestNewListStyles`: same shape.

### D — Contacts Sync (1 multi-test item)

- **G-batch2-1** `internal/contacts/sync_test.go`: unskip the four
  stubs; assert against a `webdavtest` fake server (harness at
  `internal/contacts/client_test.go`).

## Pass-end

- `/simplify`; voice + modern-go checks via `make check`.
- ADR-0233 (remediation; supersedes nothing).
- Update `elm-conventions` skill: forbid `Render("x") != ""`
  styling smoke tests; require attribute assertions.
- BACKLOG entry for `t.Skip`-in-committed-test hookify rule.
- No invariant updates expected — bug-fixes / test surface, not
  binding-fact changes.
- Archive plan; `make check`; commit; push; install.
