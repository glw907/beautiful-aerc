# Pass 8.5 B — Overengineering Audit: `internal/config/`

**Date:** 2026-05-03
**Lens:** Config decoder; provider registry; first-run flow.

## Findings

- internal/config/account.go:22-25 — 3 — Fields `Headers []string`, `HeadersExclude []string`, `CheckMail time.Duration` declared in `AccountConfig` but never written by `toAccountConfig` and never read by any consumer outside the config package.
  Action: delete
  Rationale: `accountEntry` has no corresponding TOML fields, so they are always zero after parsing; no backend or UI code reads them.

- internal/config/account.go:29 — 3 — Field `Aliases []*mail.Address` declared in `AccountConfig` but never written by `toAccountConfig` and never read anywhere.
  Action: delete
  Rationale: Same — no TOML field, no consumer; pure dead scaffolding.

- internal/config/accounts.go:41-47 — 2 — `ParseAccounts` is a one-call-site path-handling wrapper over `ParseAccountsFromBytes`; only callers are tests in same package.
  Action: delete
  Rationale: Test call sites can be replaced with `os.ReadFile(path)` + `ParseAccountsFromBytes(data)` inline.

- internal/config/providers.go:9 — 3 — `Provider.Name string` field is set in every struct literal (duplicating the map key) but never read.
  Action: delete
  Rationale: Map key is already the canonical name; field is structurally redundant.

- internal/config/providers.go:17-18 — 3 — `Provider.AuthHint string` and `Provider.HelpURL string` fields have no production-code consumers; only reads are in `providers_test.go`.
  Action: delete
  Rationale: No CLI/backend/UI surface reads these at runtime; documentation belongs in comments or the template, not as live fields.

- internal/config/loader.go:14-19 — 4 — `type Source int` and its constants `SourceFlag`/`SourceEnv`/`SourceDefault` are exported but every production caller of `Resolve` discards the second return with `_`; only used internally in `Load`.
  Action: refactor
  Rationale: Collapse to an unexported bool `flagExplicit` inside `Load`; drop the exported type, constants, and the second return of `Resolve`.
