---
title: XOAUTH2 access tokens via password-cmd
status: accepted
date: 2026-05-02
---

## Context

Gmail / Outlook XOAUTH2 access tokens expire (~1h on Gmail). The
IMAP backend caches `password-cmd` output on `b.password` so
reconnects within a session don't re-run the user's secret-
manager command. That cache breaks XOAUTH2: hour two of any
session sends a stale token.

`AccountConfig` previously carried `OAuthClientID`,
`OAuthClientSecret`, `OAuthRefreshToken`, decoded from TOML but
never read. The intent was an internal token-endpoint refresh —
never wired up because no UI captures the values.

## Decision

For `cfg.Auth == "xoauth2"`, `Backend.resolvedPassword()` skips
the cache: every dial re-runs `password-cmd`. Users wire any
refresher (`oauth2l`, `op`, a custom script) into `password-cmd`
and get a fresh token on every connection.

The unused `OAuth*` fields are deleted from `AccountConfig`,
the TOML decode struct, and the template. Pre-beta stance
(CLAUDE.md): strip dead fields; the consumer re-adds when ready.
The first-run wizard (Pass 9.6) reintroduces them — alongside
the HTTP exchange against Google's token endpoint — when there
is a UI to populate them.

## Consequences

- Reconnects on slow `password-cmd` block on the refresher each
  time. Acceptable pre-beta; documented in the template.
- The XOAUTH2 SASL mechanism (`internal/mailauth/xoauth2.go`) is
  unchanged.
- Internal refresh, when it lands, will live in `internal/mailauth/`
  as a `TokenSource` consumed by `dialCommand`/`dialIdle` —
  parallel to (not replacing) the `password-cmd` path.
