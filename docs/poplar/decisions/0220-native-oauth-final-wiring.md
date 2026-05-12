---
title: Native OAuth is the canonical XOAUTH2 path
status: accepted
date: 2026-05-11
---

## Context

Pass 35 closed the last gap in the native-OAuth subsystem that
ADR-0193 introduced. The probe stage of the setup wizard (and
`poplar config check`) ignored OAuth state and fell through to
`cfg.ResolvePassword()` for the IMAP `AUTHENTICATE` and SMTP auth
steps — so an OAuth-only account would fail the pre-save
reachability test even when consent had succeeded and a refresh
token was in the store. The config template still documented the
`oauth2l` shell-out as the recommended XOAUTH2 recipe even though
poplar has handled PKCE consent natively since ADR-0193.

## Decision

`mailimap.Probe(ctx, cfg, *mailauth.Client)` and
`mailimap.ProbeSMTP(cfg, *mailauth.Client)` accept the OAuth
client. When non-nil, Probe records an `oauth-token` step,
calls `Token(ctx)`, and threads the access token into the dial
seam; ProbeSMTP routes through `NewWithOAuth` so `smtpAuth`'s
xoauth2 branch resolves through `mailauth`. `wizard.Probe`
forwards the same client into both. The UI probe screen builds
the client from the just-collected wizard state via
`internal/ui/wizard.buildOAuthClient(state)`, which is also the
sole construction site used by `oauthSection.runAuthorize`; the
parallel `cmd/poplar.buildOAuthClient(acct)` covers the
account-config path (`config check`, `--reauth`, `openBackend`).

The config template no longer mentions `oauth2l`; the `# OAuth
providers` block documents the native flow, the `[account.oauth]`
shape, the keyring/age-file token store, and `--reauth`.

## Consequences

- A user can run `poplar` against a Gmail or Outlook account with
  nothing but a registered desktop OAuth client. The wizard's
  probe stage now succeeds end-to-end; live verification (Pass
  35.1) confirms the wire-level steps against real Google and
  Microsoft endpoints.
- `oauth2l` is unsupported in fresh configs. Existing user
  configs that shell out via `password-cmd` continue to work —
  the password-resolution path is unchanged.
- `mailauth.Client` is now constructed in four sites:
  `cmd/poplar.openBackend`, `cmd/poplar.runReauth`,
  `oauthSection.runAuthorize`, and `probeScreen.oauthClient`. The
  two wizard sites share `internal/ui/wizard.buildOAuthClient`.
- Refresh-token rotation surfaces are unchanged from ADR-0193:
  `mailauth.Client.Token` writes a rotated refresh token back to
  the store when the token endpoint returns one.
