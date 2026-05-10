---
title: Probe transcripts, typed config errors, canonical config writer
status: accepted
date: 2026-05-10
---

## Context

The first-run wizard (Pass 14b) needs three substrate pieces from
the existing config and mail-backend packages:

1. A way to render a connect-test as a step-by-step transcript so
   the wizard's probe screen can show progress live and surface
   server-side failure detail at the right step. Both `mailimap`
   and `mailjmap` already have monolithic `Connect` paths; neither
   exposes per-step granularity.
2. Typed validation errors carrying enough context to drive the
   wizard's `--repair` flow and a friendly malformed-config error
   in `runRoot`.
3. A canonical TOML writer the wizard can call to assemble the
   first-run `config.toml`.

Pass 14a lands these as standalone substrate so 14b can build on
them without churn. None of the three has UI dependencies; all
three are testable in isolation.

A fourth concern surfaced inline: the `name = ` field on
`[[account]]` was required, which made even minimal configs awkward.
BACKLOG #29 tracked this; the fix lands here because it touches
`accountEntry.toAccountConfig` alongside the new `ConfigError`
plumbing.

## Decision

### `mail.ProbeResult` shared types

`internal/mail/probe.go` exports `ProbeStatus` (`ProbeOK`,
`ProbeFail`), `ProbeStep{Label, Status, Detail}`, and
`ProbeResult{Steps, Err}`. `ProbeResult.OK()` reports whether the
probe completed without a failed step. The types live in
`internal/mail` so both `mailimap` and `mailjmap` import them
without cycle. `ProbePending`/`ProbeSkip` are reserved — when a
future probe (SMTP, OAuth) needs intermediate states, add them
back at that pass.

### `mailimap.Probe` (5 steps)

`mailimap.Probe(ctx, cfg) mail.ProbeResult` records: Connecting,
TLS handshake, AUTHENTICATE, CAPABILITY (UIDPLUS), STATUS INBOX.
Each is one ProbeStep; the first failure stops the transcript.
The dial+TLS+auth phase routes through a `probeDial` package
function variable so tests substitute a fake. The TLS-layering
helper extracted from `dial()` is now `layerTLS(raw, cfg, tlsCfg,
opts)` — both the live dial path and the probe call it.

STATUS INBOX uses read-only `SELECT INBOX` rather than `STATUS
INBOX (MESSAGES)`. Both return the same EXISTS count; SELECT is
already on the `imapClient` interface and the wizard probe is
short-lived so there's no cost from briefly selecting INBOX.

### `mailjmap.Probe` (3 steps)

`mailjmap.Probe(ctx, cfg) mail.ProbeResult` records: Resolving
session URL, Authenticate, mailbox/get. The spec wireframe
proposed five steps (URL, TLS, bearer, Session/get, Account/get),
but `~rockorager/go-jmap`'s `Client.Authenticate()` bundles TLS,
bearer, and Session/get into one call with no per-phase hook. The
honest 3-step layout reflects the library boundary; carving the
single library call into synthetic sub-steps would mis-attribute
errors. Authenticate is the test seam (`probeAuth` package
variable); `probeMailboxGet` runs the `Mailbox/get` RPC.

### `config.ConfigError`

`internal/config/errors.go` defines
`ConfigError{Path, Line, Account, Field, Message, Suggest}` and
the sentinel `ErrConfigInvalid`. `(*ConfigError).Is(target) bool`
reports whether target is `ErrConfigInvalid`, so callers can
branch with `errors.Is`. Pass 14a migrates the four most user-
facing validators in `accountEntry.toAccountConfig` (unknown
provider, missing host, missing source, missing smtp.host) plus
the new "name and email both blank" check. Identity/signature
validators stay on bare `fmt.Errorf` until a consumer reads them
through Field/Suggest.

### `config.Render`

`config.Render(accts []AccountConfig, ui UIConfig, cache
CacheConfig) []byte` emits canonical TOML. Round-trips through
`Load*` are semantic, not byte-for-byte; comments are not
preserved and default-valued fields are elided. The
`[account.smtp]` block precedes `[[account.identity]]` blocks in
the output — a TOML quirk: a bare `[section]` header after array-
of-tables entries would otherwise be parsed as a sub-table of
the last array element.

For 14a the renderer covers AccountConfig (provider/backend,
host/port, source, identities, signatures, smtp), `[ui]`, and
`[cache]`. `[ui] theme` is not rendered because `UIConfig` does
not yet model theme; 14b adds that field alongside the wizard's
theme section.

### Account `name` defaults to email (#29)

`accountEntry.toAccountConfig`: when `name` is empty, default to
`email`. Fail with a `ConfigError{Field: "name"}` only when both
are blank. The template documents the defaulting; the validator
no longer requires `name`.

## Consequences

- Pass 14b can build the wizard on a clean, pre-existing test
  surface: probe transcripts already work standalone, `Render`
  has a round-trip test, `ConfigError` already drives the friendly
  error formatter that 14c will wire into `runRoot`.
- The 3-step JMAP transcript looks shorter than the spec
  wireframe. The wizard's probe screen reads the same generic
  `ProbeStep` shape regardless of length; the wireframe gets
  refreshed when 14b lands the probe screen.
- `ProbePending`/`ProbeSkip` removed from the enum; future probes
  that need them re-add at their own pass without preserving dead
  consts here (pre-beta posture).
- `bringupTLS` private helper in probe.go was extracted into the
  shared `layerTLS` in auth.go; the live dial path now uses the
  same code.
- `config.Render` does not yet emit `[ui] theme` because the
  loader does not yet read it. 14b extends both ends.
- BACKLOG #29's name-required validator is closed by the default-
  to-email behavior. Existing configs with `name` set keep
  working unchanged; new configs can omit it.
