# Pass 23 — First-launch safety

Three first-launch / config hazards in scope (BACKLOG #49, #29, #51).
Pure implementation; no open questions. No UI surface changes.

## Scope

### A. `config.ResolvePreset` (#49)

`internal/config/accounts.go:364-394` currently inlines the
preset-merge into `toAccountConfig`. Extract it as:

```go
// ResolvePreset fills empty backend/transport fields on c from the
// provider preset table. Idempotent. Safe to call from the wizard
// before the TOML round-trip.
func ResolvePreset(c *AccountConfig)
```

Behaviour: when `c.Preset` (or, on the decoder path, the raw
provider key) names a `Providers[…]` entry, copy `Backend`,
`Host`, `Port`, `StartTLS`, `InsecureTLS`, `GmailQuirks`, `Source`
(from `preset.URL`), and SMTP host/port/StartTLS/InsecureTLS into
the matching empty slots. Non-empty slots win — user overrides
survive.

Decoder calls it after seeding the locals; the OAuth-defaults
block stays inline (it mutates the entry, not the cfg).
`wizard.Apply` calls it on the default branch after setting
`cfg.Preset`, so the returned cfg has `Host`/`Source`/SMTP ready
for `wizard.Probe`.

### B. Template-defaults guard (#29)

The decoder already defaults `Name` to `Email` when `Name == ""`
(landed in Pass 14a). Backlog entry is stale. Add a regression
test that round-trips `config.Template()` (with the placeholder
email replaced by a real-looking address) through
`ParseAccountsFromBytes` and asserts no error + `Name == Email`.
Close #29 against the existing implementation.

### C. `MockBackend` build-gate (#51)

Production binaries must refuse `provider = "mock"`. Tests still
want it.

- `cmd/poplar/backend.go` drops `case "mock", "":` from the
  production switch. Empty `Backend` (which can only reach this
  layer via a config that bypassed validation) errors with a
  named error.
- `cmd/poplar/backend_dev.go` (`//go:build dev`) and
  `cmd/poplar/backend_nodev.go` (`//go:build !dev`) provide
  `openMockBackend(acct) (mail.Backend, error)`. Production
  returns `errMockUnavailable`; dev returns
  `mail.NewMockBackend()`.
- Production switch calls `openMockBackend` on `case "mock":`,
  so dev tests still work; production binaries surface a clear
  error if a mock-provider config ever leaks through.
- Validator in `internal/config/accounts.go` stays permissive
  for `provider = "mock"` (tests use it). Empty provider is
  already rejected via the unknown-provider path; tighten the
  error message with an explicit early branch
  (`fmt.Errorf("provider is required")`).
- Makefile: `make test` and `make check` switch to
  `go test -tags dev ./...` so the dev-tagged backend is in scope
  during unit tests. `make build`/`make install` stay untagged.
- `make test-imap` keeps its existing `-tags=integration`; can add
  `dev` if needed (not needed — that target hits real IMAP).

## Out of scope

- IMAP IDLE redial (#53) — Pass 24.
- Outbox send/append gate (#52) — Pass 24.
- Any UI work.

## Tasks

1. Plan doc (this file).
2. `ResolvePreset` extraction, decoder refactor, `wizard.Apply`
   invocation, unit tests for both paths.
3. Template round-trip regression test (#29).
4. `cmd/poplar/backend.go` split; Makefile `-tags dev` update;
   verify `config_discover_folders_test` runs under the new
   tags.
5. Pass-end ritual: ADR, invariants/BACKLOG/STATUS edits,
   archive, make check, commit, push, install.
