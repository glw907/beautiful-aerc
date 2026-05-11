---
title: First-launch safety — ResolvePreset and dev-gated MockBackend
status: accepted
date: 2026-05-11
---

## Context

Pass 22's live-verify of the signature wizard surfaced three
first-launch hazards that converge on `internal/config/accounts.go`
and the wizard probe path:

- The wizard's pre-save probe (`probe_screen.go` calls
  `wizdomain.Apply`) handed JMAP/IMAP probes a config with no
  session URL or host. The preset-merge block lived only in
  the TOML decoder (`internal/config/accounts.go:364-394`), so
  every hosted-preset first-run died with "session URL is empty"
  or "host empty" before the file was even written. (#49)
- `internal/mail/mock.go` (280 lines of demo data) compiled into
  release binaries: `cmd/poplar/backend.go` accepted
  `case "mock", "":` and `MockBackend.Send` returned `nil`
  unconditionally. A user who hand-edited `config.toml` and
  dropped the provider line silently booted a fake mailbox; the
  first reply to a real-looking demo message would vanish with
  no SMTP traffic. (#51)
- BACKLOG #29 (template defaults `name` to email) was
  effectively resolved by Pass 14a but never closed. The current
  decoder already defaults `Name` to `Email` when blank; a
  regression test pins that against future drift.

## Decision

1. `config.ResolvePreset(*AccountConfig)` is the single
   preset-merge function. It fills empty backend/transport
   fields (Backend, Host, Port, StartTLS, InsecureTLS,
   GmailQuirks, Source, SMTP host/port/StartTLS/InsecureTLS)
   from `Providers[c.Preset]`. Non-empty slots win. Idempotent.
   The TOML decoder calls it after assembling the partial
   `AccountConfig`; `wizard.Apply` calls it on the hosted-preset
   branch before returning. The wizard probe now sees the same
   resolved config the runtime would after a TOML round-trip.

2. `MockBackend` is gated behind the `dev` build tag at the
   `cmd/poplar` layer. `openBackend`'s `case "mock":` delegates
   to `openMockBackend(acct)`, which is supplied by
   `cmd/poplar/backend_dev.go` (`//go:build dev`, returns
   `mail.NewMockBackend()`) or
   `cmd/poplar/backend_nodev.go` (`//go:build !dev`, returns a
   clear "not available in release builds" error). The config
   validator stays permissive for `provider = "mock"` so tests
   (in particular `cmd/poplar/config_discover_folders_test.go`)
   keep working. `make test` and `make check` now pass
   `-tags=dev` so the dev backend is in scope for unit tests;
   `make build` / `make install` stay untagged.

3. The decoder rejects empty `provider` with an explicit
   "provider is required" error before any other validation
   runs (previously empty provider fell through to the
   "unknown provider" path with a less useful message).

## Consequences

- The wizard is usable end-to-end for every hosted preset on
  first run. The repair path benefits too: `FromAccount` →
  `Apply` now produces a probe-ready config without round-
  tripping through disk.
- A misconfigured or hand-edited `config.toml` with
  `provider = "mock"` can no longer ship mail to /dev/null
  from a release binary. The error names the account and
  points at the `-tags dev` rebuild.
- Tests that drive `cmd/poplar` flows through `provider =
  "mock"` continue to work under `make test`; standalone
  `go test ./...` (without `-tags dev`) compiles but a few
  cmd/poplar tests will fail. Standardize on `make test`.
- BACKLOG #29, #49, #51 close.
