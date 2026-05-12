# Pass 35 — Native OAuth final wiring

**Date:** 2026-05-11
**Status:** design accepted
**Goal:** Finish wiring native OAuth (PKCE + refresh) for Gmail and
Outlook IMAP so the user can complete first-run setup through the
wizard without shelling out to `oauth2l` or pasting app-specific
passwords. The bulk of the infrastructure already exists from
earlier scaffolding; this pass closes the remaining gaps and tags
native OAuth as the canonical XOAUTH2 path.

## Context

Pre-Pass-35 inventory (already built):

- `internal/mailauth/`: `Client` with `Authorize` (PKCE +
  loopback consent server, kernel-assigned port), `Token` (cached
  access token, 5-minute refresh window), `ForceRefresh`;
  `TokenStore` with keyring + age-file fallbacks; XOAUTH2 SASL.
- `cmd/poplar/reauth.go`: `--reauth=<name>` re-runs consent.
- `cmd/poplar/backend.go`: `openBackend` constructs
  `*mailauth.Client` for IMAP accounts with `[account.oauth]`
  and calls `mailimap.NewWithOAuth`.
- `internal/config/providers.go`: presets `gmail` and `outlook`
  carry `OAuthDefaults` (AuthURL / TokenURL / Scopes) and
  `OAuthRequiresSecret: true`. `[account.oauth]` schema lives in
  ADR-0193.
- `internal/wizard/oauth_strategy.go`: `Authorizer` seam,
  `NewOAuthStrategy`, `Apply`.
- `internal/ui/wizard/section_oauth.go`: huh form for client
  ID/secret, spinner during consent, retry/cancel on failure,
  `oauthAdvanceMsg` to flow into the probe stage.
- `internal/mailimap/`: `auth.go` and `smtp.go` already route
  through `mailauth.Client.Token` when `b.oauth` is non-nil;
  one retry with `ForceRefresh` on auth failure.

The three open questions in the Pass 35 starter prompt resolved
inline from existing code:

1. **Loopback redirect port** — kernel-assigned (`{0,0}` →
   `127.0.0.1:0`). Both Gmail and Outlook accept
   `http://127.0.0.1:PORT/callback` for native PKCE clients. No
   config knob; YAGNI until a firewall report.
2. **Browser-open seam** — `mailauth.SetOpenBrowser` stays
   separate from the App-side `URLOpener`. `mailauth` runs
   outside the tea loop (CLI `--reauth` and a wizard Cmd);
   `URLOpener` is bubbletea-scoped. Different lifecycles.
3. **Wizard probe composition** — the OAuth section already
   advances to the probe stage via `oauthAdvanceMsg`. The gap
   is that `wizard.Probe` passes `nil` for the OAuth client, so
   the IMAP probe runs without an access token and fails for
   OAuth accounts. This pass fixes that.

## Decision

Three code slices plus live verification plus an ADR.

### 1. Thread `*mailauth.Client` through `wizard.Probe`

`internal/wizard/probe.go` currently has:

```go
imapProbeFn = func(ctx context.Context, cfg config.AccountConfig) mail.ProbeResult {
    return mailimap.Probe(ctx, cfg, nil)
}
smtpProbeFn = mailimap.ProbeSMTP
```

Change `imapProbeFn` and `smtpProbeFn` signatures to take a
`*mailauth.Client`. `wizard.Probe` becomes:

```go
func Probe(ctx context.Context, cfg config.AccountConfig, oauthCli *mailauth.Client) mail.ProbeResult {
    // ...
}
```

Non-OAuth callers pass `nil`. Two callers:

- `internal/ui/wizard/section_probe.go` — build the client from
  `m.State.OAuthCID/Secret/Store` + the preset's
  `OAuthDefaults`, using the same `mailauth.OpenStore` +
  `mailauth.NewClient` pattern as `section_oauth.go`. The
  refresh token is already in the store from the just-completed
  consent flow, so `Token(ctx)` returns immediately without
  another browser hop.
- `cmd/poplar/config_cmd.go` — `config check` already routes
  through `openBackend`, not `wizard.Probe`; no change.

`mailimap.ProbeSMTP` also takes a `*mailauth.Client` and uses it
when `cfg.SMTP.Auth == "xoauth2"`. Mirrors the IMAP probe pattern.

### 2. Replace the `oauth2l` block in the config template

`internal/config/template.go:87` currently shows
`password-cmd = "oauth2l fetch --type=oauth2 …"` as the canonical
Gmail/Outlook recipe. Replace with an `[account.oauth]` block
example showing `client-id`, `client-secret`, and `oauth-store`.
The `password-cmd` example survives for self-hosted IMAP and
mailbox-org accounts that still use app passwords.

### 3. ADR-0220 — native OAuth is the canonical XOAUTH2 path

Records:

- `[account.oauth]` is the supported credential mechanism for
  Gmail and Outlook IMAP/SMTP.
- Loopback redirect is ephemeral (`127.0.0.1:0`); no fixed port.
- Browser-open seam stays separate from `URLOpener` — `mailauth`
  is tea-loop-free.
- `oauth2l` shell-out is deprecated; the template no longer
  recommends it. (Users with existing `password-cmd = "oauth2l
  …"` configs continue to work — no schema break.)

Update `docs/poplar/invariants.md` to record:

- `mailauth.Client` is constructed in three places — `openBackend`,
  `runReauth`, and `section_oauth.go`/`section_probe.go` — and
  passed through both `mailimap.Probe` and
  `mailimap.ProbeSMTP` when present.
- Update `docs/poplar/decisions/INDEX.md` with the ADR-0220 row.

### 4. Live verification

End-to-end against a real Gmail account and a real Outlook
account. tmux captures at 80×24 and 120×40 covering:

- Wizard credentials form (preset, email, OAuth client ID/secret)
- Spinner during consent flow
- Probe-stage pass for both backends
- Account loads after wizard completion (inbox renders)
- `poplar --reauth=<name>` rerunning consent and updating the
  stored refresh token

Both keyring and age-file token stores exercised (keyring on the
primary account, age-file on a second test account or by
unsetting the keyring env var). Capture session log via
`POPLAR_LOG=debug` to verify `mailauth.Token` cache hits and
`oauth2.Token`'s `RefreshToken` rotation handling.

## Consequences

- The wizard's probe stage works for OAuth accounts. Today it
  fails for Gmail/Outlook because the probe runs with `nil`
  OAuth client → no access token → AUTHENTICATIONFAILED.
- `wizard.Probe` and `mailimap.ProbeSMTP` gain a parameter; one
  signature change ripples through `imapProbeFn` /`smtpProbeFn`
  indirections and the wizard `section_probe.go` caller.
- `oauth2l` is documented as deprecated. Existing user configs
  using `password-cmd = "oauth2l …"` continue to work (no schema
  change), but the template no longer steers new users toward
  the shell-out path.
- `[account.oauth]` becomes a first-class wizard output. Pre-1.0
  rules: no compat shim for users mid-migration from
  `password-cmd` — the migration is "re-run `poplar init
  --interactive`" or edit TOML by hand.

## Out of scope

- JMAP OAuth (Fastmail uses bearer tokens by design).
- OAuth device-code flow (for headless/SSH sessions).
- Additional OAuth providers beyond Gmail and Outlook.
- Refactoring `mailauth` — current API survives.
- Polishing the wizard OAuth section UX beyond what live
  verification surfaces.

## Tasks (target: ~6, well under 12-task ceiling)

1. Change `mailimap.ProbeSMTP` signature to take
   `*mailauth.Client`; route through it when
   `cfg.SMTP.Auth == "xoauth2"`. Update callers.
2. Change `wizard.Probe` signature + `imapProbeFn` / `smtpProbeFn`
   to thread `*mailauth.Client`. Update tests.
3. Build the client in `section_probe.go` from wizard state when
   the strategy is `StrategyOAuth`; thread into `wizard.Probe`.
4. Replace the `oauth2l` block in `internal/config/template.go`
   with an `[account.oauth]` example.
5. Live verification: Gmail account end-to-end through wizard;
   tmux captures at both widths; `--reauth` re-run; log capture.
6. Live verification: Outlook account end-to-end; same artifacts.
7. ADR-0220 + invariants update + INDEX.md row. (Folded with the
   pass-end consolidation ritual.)

## Verification

- `make check` green before commit.
- Live wizard runs land in the cache and render the inbox; the
  account is usable after wizard exits.
- `poplar --reauth=<name>` updates the refresh token and the
  next `mailimap.Connect` succeeds without re-running consent.
- Auth-error retry path exercised by deliberately revoking the
  refresh token mid-session — backend surfaces `mail.ErrAuth`,
  not a transport error.
