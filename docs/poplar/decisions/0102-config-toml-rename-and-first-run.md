---
title: config.toml rename, provider key, first-run template, friendly errors
status: accepted
date: 2026-05-02
---

## Context

Pass 8.5 polished the v1 configuration story. The pre-pass file was
`accounts.toml` with a `backend` key naming the resolved backend
("imap"/"jmap") — confusing because `provider` presets (yahoo,
icloud, …) decode to the same key. There was no first-run path, no
template, and validation errors lacked account context. Path lookup
was ad-hoc in `cmd/poplar/backend.go` and duplicated by
`config_init.go`.

## Decision

- The runtime config file is `config.toml` (was `accounts.toml`).
  Default location is `~/.config/poplar/config.toml` on Linux and
  macOS (poplar deliberately uses XDG on macOS, matching pass /
  nvim / tmux / git). Windows uses `%APPDATA%\poplar\config.toml`.
- The TOML key for the provider/backend selector is `provider`
  (was `backend`). The `accountEntry.Provider` field decodes it;
  `AccountConfig.Backend` continues to hold the resolved canonical
  backend string after preset lookup.
- Path resolution lives in `internal/config/loader.go` as
  `Resolve(flagPath string) (string, Source, error)`. Precedence:
  `--config` flag, then `$POPLAR_CONFIG`, then OS default.
- `config.Load(flagPath)` returns `([]AccountConfig, path, error)`.
  When the default path is missing it writes `Template()` (the
  self-documenting template) and returns `ErrFirstRun`; the root
  command prints a hint and exits 78 (EX_CONFIG). When a legacy
  `accounts.toml` is found at the same dir, it returns
  `ErrOldAccountsToml` and exits 78 with a rename hint. With an
  explicit `--config` path, missing files error without writing.
- Validation errors carry account name and provider context:
  `account "p" (provider = "imap"): host is required for imap
  accounts`. Unknown providers surface a Levenshtein-based "did
  you mean" suggestion when within edit distance 2.
- New cobra subcommands: `poplar config init` (write template;
  refuses to overwrite without `--force`), `poplar config check`
  (validate + connect-test each account), `poplar config path`
  (print resolved path). The previous `poplar config init`
  (folder discovery) was renamed to `poplar config discover-folders`.
- The self-documenting template lives in `internal/config/template.go`
  as a const string and is checked against `template.golden` so any
  drift surfaces in code review.

## Consequences

- Pre-1.0 churn: users must rename `accounts.toml` → `config.toml`
  and `backend = ...` → `provider = ...`. The rename hint runs at
  startup; no compat shim.
- The `Source` enum is exported. Today only `Load` uses it, but it
  also describes the resolution channel for future diagnostics.
- The template-write-on-first-run also fires when `$POPLAR_CONFIG`
  is set (SourceEnv) — the user explicitly chose where the file
  lives and is OK with poplar materializing it. Explicit `--config`
  flag (SourceFlag) errors instead.
- `poplar config check` is sequential per-account today; parallel
  would speed multi-account checks but complicates output ordering.
  Defer until users report it.
