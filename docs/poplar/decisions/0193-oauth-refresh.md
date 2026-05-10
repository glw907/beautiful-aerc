---
title: OAuth refresh for Gmail/Outlook IMAP
status: accepted
date: 2026-05-10
---

## Context

Pass 14b shipped `StrategyOAuth` as a wizard option but wrote a
placeholder `password-cmd = "echo TODO-pass-14.1-oauth"` because
the real consent flow was out of scope. Gmail and Outlook cannot
authenticate IMAP/SMTP without a working XOAUTH2 token path; the
two presets sit unusable until that ships.

## Decision

- BYO OAuth client: user registers their own client in Google
  Cloud / Azure Portal once and pastes client_id + client_secret
  into the wizard. Maintainer-verified clients are out of scope
  (Google CASA audit, Microsoft publisher verification).
- Consent flow is loopback PKCE — universally supported by Google
  and Microsoft, no display-name verification step, no device-code
  fallback in v1.
- Refresh tokens persist via `zalando/go-keyring` with a per-
  account age-encrypted file at
  `$XDG_CONFIG_HOME/poplar/oauth/<slug>.age` as fallback. The
  wizard probes the keyring once and records the chosen backend
  in `[account] oauth-store = "keyring"|"age-file"`.
- Access tokens cache in-memory on the `*mailauth.Client`; refresh
  triggers when <5min remains or upstream returns invalid_grant.
- CLI surface: `--reauth=<name>` re-runs the OAuth sub-screen
  against a named account.

## Consequences

Unlocks Gmail + Outlook end-to-end through the wizard.
`password-cmd` stays the parallel auth path for self-hosted IMAP
and Fastmail JMAP — no cross-cutting refactor. Adds three deps:
`golang.org/x/oauth2`, `github.com/zalando/go-keyring`,
`filippo.io/age`. Forecloses device-code in v1; revisit only if
SSH-without-tunneling feedback identifies a real cohort. JMAP
OAuth deferred — Fastmail uses long-lived API tokens; no current
consumer wants it.
