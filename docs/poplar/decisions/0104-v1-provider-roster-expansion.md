---
title: v1 provider roster — outlook, mailbox-org, posteo, runbox, gmx, protonmail
status: accepted
date: 2026-05-02
---

## Context

Pass 8 (ADR-0098) introduced the provider-preset registry with
fastmail/yahoo/icloud/zoho. Pass 8.5 expands to the v1 roster.
Gmail (ADR-0101) lands in Pass 8.1 with X-GM-EXT-1 quirks.

## Decision

Six new presets in `internal/config/Providers`:

- `outlook` — `outlook.office365.com:993`, AuthHint `xoauth2`.
- `mailbox-org` — `imap.mailbox.org:993`, AuthHint `app-password`.
- `posteo` — `posteo.de:993`, AuthHint `app-password`.
- `runbox` — `mail.runbox.com:993`, AuthHint `app-password`.
- `gmx` — `imap.gmx.com:993`, AuthHint `app-password`.
- `protonmail` — `127.0.0.1:1143` via local Bridge,
  `StartTLS = true`, **`InsecureTLS = true`** (Bridge ships a
  self-signed cert on loopback), AuthHint `bridge-password`.

`Provider` gains an `InsecureTLS bool` field which decodes from
`insecure-tls` and flows through preset resolution to
`AccountConfig.InsecureTLS`. Self-hosted IMAP setups can opt in
via the same field at the account level.

When TLS handshake fails on a host that looks self-hosted (RFC
1918 IPv4, IPv6 ULA, `.local`, or 127.x) **and** `InsecureTLS` is
not already set, the error wrap appends "set insecure-tls = true
if self-signed". The hint is suppressed when `InsecureTLS` is
already on (the failure is something else).

## Consequences

- ProtonMail support is "via Bridge" only — there is no direct
  ProtonMail protocol. The template documents the Bridge install
  steps and the `bridge-password` workflow.
- The Outlook preset assumes XOAUTH2 — the actual OAuth flow
  arrives with the OAuth-helper work that Pass 8.1's Gmail
  preset will share.
- The TLS hint is heuristic. False positives (RFC 1918 host with
  a real cert that fails for an unrelated reason) are possible
  but cheap; the hint is advisory.
