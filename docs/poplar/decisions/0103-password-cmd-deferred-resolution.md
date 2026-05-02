---
title: password-cmd resolved at first Connect, cached on Backend
status: accepted
date: 2026-05-02
---

## Context

Pre-pass, account credentials were either inline `password = "..."`
or routed through the legacy aerc-era `credential-cmd` that
injected a token into the JMAP source URL at config-load time.
Both patterns have problems: inline passwords sit in plaintext on
disk; `credential-cmd` runs synchronously at startup so secret-
manager prompts (1Password, pass) interrupt every launch even
when the user never connects to mail.

## Decision

- `[[account]] password-cmd = "<shell command>"` runs `/bin/sh -c
  <cmd>` and uses stdout (with trailing newline trimmed) as the
  password. Mutually exclusive with `password = "..."`; the parser
  rejects both being set with `account "p": both password and
  password-cmd set; use one`.
- Resolution is **deferred**: at config-load the field is decoded
  but not run. Each backend (`internal/mailimap`, `internal/mailjmap`)
  resolves at first `Connect` via a private `resolvePassword(cfg)`
  function and caches the cleartext result on a `password string`
  field of `Backend`, mu-guarded.
- The two backends each carry their own copy of the ~30-line
  resolver + cache. The duplication is intentional: extracting
  to `internal/mailauth/` (or a method on `*AccountConfig`) is
  cheap, but it would couple the mail-backend layer to a config-
  package method that today has no other consumer. Refactor when
  a third backend lands.
- Legacy `credential-cmd` and `outgoing-credential-cmd` fields
  removed (along with `runCredentialCmd` / `injectCredential`
  helpers and the `Outgoing` / `OutgoingCredCmd` fields on
  `AccountConfig`). `Outgoing` will be re-added in Pass 9 when
  compose lands.

## Consequences

- Password-manager prompts surface near the first `Connect`
  attempt, not at startup. `poplar config check` is the explicit
  way to test all credentials up front.
- Reconnects within a session reuse the cached password — no
  re-prompt per reconnect. Process restart re-runs the command.
- The `password-cmd` runs through `/bin/sh -c`, so users get full
  shell quoting. Windows lacks `/bin/sh`; on Windows the
  documented fallback is `password = "$VAR"` with the env var set
  by the user's secret-manager integration. A future
  `runtime.GOOS`-aware exec path can add `cmd.exe /C` support.
- `cfg.Password` (post env-var substitution) and the cached
  `Backend.password` coexist: inline always wins via
  `resolvePassword`'s first branch; the cache only ever holds a
  `password-cmd` result.
