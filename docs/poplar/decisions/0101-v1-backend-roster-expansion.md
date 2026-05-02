---
title: V1 backend roster — generic IMAP plus presets
status: accepted
date: 2026-05-02
---

## Context

ADR-0075 set the v1 backend roster as "Fastmail JMAP + Gmail IMAP."
That captured intent but understated the IMAP side: once an IMAP
backend exists at all, it works against any IMAP server, not only
Gmail. The provider registry (ADR-0098) makes well-known servers
one-line account configs.

## Decision

V1 supports two protocol backends, exposed through the provider
registry plus direct configuration:

- **JMAP** — `backend = "jmap"` or `backend = "fastmail"`. Native
  protocol; only Fastmail in v1.
- **IMAP** — `backend = "imap"` (explicit host/port) or one of the
  presets `yahoo`, `icloud`, `zoho` that resolve to IMAP. Self-
  hosted servers use direct `imap` with the optional `insecure-tls`
  flag for self-signed certs (ADR-0102 if/when codified). Gmail
  joins the preset list in Pass 8.1 with `GmailQuirks` enabling
  X-GM-EXT-1 handling and the Trash-precondition-before-EXPUNGE
  pattern.

Maildir, mbox, and notmuch remain out of scope — those are aerc's
strength, not poplar's.

## Consequences

- Supersedes the "Fastmail JMAP + Gmail IMAP only" line in
  invariants.md; the new line names the protocol set, not the
  provider set.
- `mail.Backend` interface is unchanged.
- `cmd/poplar/backend.go` dispatch is `imap` → `mailimap.New`,
  `jmap` → `mailjmap.New`. Preset names canonicalize to one of
  these during config decode (ADR-0098).
- Future providers slot into `internal/config/providers.go`
  without touching the dispatch.
