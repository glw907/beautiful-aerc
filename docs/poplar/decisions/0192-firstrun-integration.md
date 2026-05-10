---
title: First-run wizard integration in runRoot
status: accepted
date: 2026-05-10
---

## Context

Pass 14a landed the typed `config.ConfigError` and probe substrate.
Pass 14b landed the wizard domain and bubbletea + huh surface plus a
`poplar config init --interactive` subcommand. Pass 14c integrates
those into the default `poplar` invocation: missing config should
auto-launch the wizard, a malformed config should surface a friendly
typed error with a recovery hint, and existing automation that
expected the exit-78 first-run path needs an opt-out.

## Decision

`runRoot` resolves config in three branches:

1. **Auto-launch on `ErrFirstRun`.** The freshly-written template is
   removed and `uiwiz.NewModel(t)` runs in `tea.NewProgram`. On a
   clean wizard exit the config is reloaded and the app proceeds.
   `--no-wizard` and `POPLAR_NO_WIZARD=1` route back to the prior
   exit-78 behavior for cron, CI, and automation.
2. **Friendly format on `ConfigError`.** `errors.As` extracts the
   typed error; `runRoot` prints `poplar: <error>` followed by the
   `Run \`poplar --repair=<acct>\`` hint when `Account != ""` and
   exits 78. `ErrOldAccountsToml` keeps its prior exit-78 path.
3. **`--repair=<name>`** loads existing accounts (tolerating
   `ConfigError`), seeds the wizard via `Model.WithRepair(name,
   cfg)` which restricts to the account section and reverse-applies
   the known-good fields, then splices `RepairResult` back into the
   accounts list and atomically rewrites `config.toml` via
   `config.Render`. The wizard's confirm step short-circuits its
   single-account `writeConfig` when `Repair` is set so the caller
   owns the multi-account file rewrite.

`internal/wizard/FromAccount(cfg)` is the reverse of `Apply`: it
seeds preset, label, identity name, and (for self-hosted) host /
port / session URL. Credentials are not reversed — repair always
re-collects them.

## Consequences

- Existing automation breaks on first-run unless it sets
  `POPLAR_NO_WIZARD=1` or passes `--no-wizard`. Documented in
  invariants and the help text.
- `--repair` is single-account-aware: it preserves other
  `[[account]]` blocks in the file via `config.Render` round-trip
  but cannot edit them simultaneously. Multi-account batch repair
  is post-1.0.
- The wizard's `RepairResult` field is the caller's accept channel.
  Future passes that add wizard sections beyond `account` need to
  decide whether they participate in repair (e.g., changing
  contacts URL via `--repair=foo --section=contacts`); the section
  registry already supports this.
