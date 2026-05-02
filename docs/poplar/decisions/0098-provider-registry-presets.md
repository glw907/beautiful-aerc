---
title: Provider registry presets for IMAP/JMAP accounts
status: accepted
date: 2026-05-02
---

## Context

Pass 8 adds a generic IMAP backend that needs per-provider quirks
(host, port, IMAP vs JMAP, TLS shape, auth hint). Asking users to
look up `imap.mail.yahoo.com:993` or the Fastmail JMAP session URL
every time they configure an account is friction we can eliminate.

## Decision

`internal/config/providers.go` holds a `Providers` map keyed by
short name (`fastmail`, `yahoo`, `icloud`, `zoho`). Each entry is
a `Provider` struct carrying `Backend` (`"imap"` or `"jmap"`),
`Host`/`Port`/`StartTLS` (IMAP) or `URL` (JMAP), `AuthHint`, and
`HelpURL`. Setting `backend = "yahoo"` in `accounts.toml` resolves
through `LookupProvider` during `toAccountConfig` — the canonical
`Backend` becomes `"imap"`, host/port/source fill in. Direct
`backend = "imap"` with explicit `host` / `port` still works for
self-hosted servers.

Adding a new well-known provider is a single struct literal in the
map. No code changes elsewhere.

The provider name `fastmail` resolves to the JMAP backend; IMAP
against Fastmail uses `backend = "imap"` with explicit host. Gmail
gets a distinct `gmail` preset in Pass 8.1 that carries the
`GmailQuirks` flag for X-GM-EXT and Trash-precondition handling.

## Consequences

- New providers are one struct literal away.
- `Provider` struct is the schema; future fields (e.g.,
  `OAuthEndpoint`) extend it without touching consumers.
- Eager "source is required" validation moved from
  `toAccountConfig` to per-backend constructors (each protocol
  package validates its own inputs at `Connect`).
- `accounts.toml` documents become significantly shorter for
  hosted-provider users.
