---
title: Direct-on-libraries mail stack — supersede aerc fork
status: accepted
date: 2026-04-25
---

## Context

ADR-0002 chose to fork aerc's worker code (April 2026) on the
premise that the Go JMAP landscape was too thin to support a
library-based architecture. The Pass 2.9 research
(`docs/poplar/research/2026-04-25-mail-library-stack.md`) shows
the premise is wrong: `git.sr.ht/~rockorager/go-jmap` (already a
poplar dependency) covers JMAP Core, Mail, MDN, S/MIME, and
Push/EventSource — the full surface poplar needs. The same
research shows the fork's ~10 kLOC contributes mostly aerc's
async Action→Message worker idiom, which the synchronous
`mail.Backend` interface then has to bridge back via a pump
goroutine. Aerc's fork remains in the tree but has not been
touched since the day it landed; Pass 3 hasn't begun.

## Decision

Mail backends call upstream libraries directly under the
synchronous `mail.Backend` interface. No aerc fork.

- `internal/mailjmap/` is rewritten to call
  `git.sr.ht/~rockorager/go-jmap` directly (Fastmail JMAP).
- `internal/mailimap/` is a new package that calls
  `github.com/emersion/go-imap` v1 directly (Gmail IMAP).
- `internal/mailauth/` vendors `auth/xoauth2.go` (~80 LOC, fills
  emersion/go-sasl's XOAUTH2 gap) and `keepalive/` (~32 LOC).
  `internal/mailimap/xgmext/` vendors aerc's Gmail X-GM-EXT
  helpers (~300 LOC). All three are MIT-licensed snippets with
  preserved provenance comments.
- `internal/mailworker/` is deleted in Pass 3.
- The async→sync pump goroutine is deleted with the fork. Both
  backends are synchronous from the start.
- The library family for everything mail-adjacent is emersion
  (go-imap v1, go-message, go-smtp, go-sasl, go-webdav, go-vcard)
  plus rockorager/go-jmap. `emersion/go-smtp` lands in Pass 9
  for compose+send. `emersion/go-webdav` + `go-vcard` land
  post-1.0 for CardDAV contacts.

## Consequences

- Supersedes ADR-0002 (clean fork over direct import) — reversed.
- Supersedes ADR-0010 (synchronous adapter via async pump) — sync
  shape stays, the pump goroutine vanishes because there is no
  async to bridge.
- Supersedes ADR-0006 (fork namespace `internal/aercfork/` →
  `internal/mailworker/`) — no fork to namespace.
- Supersedes ADR-0008 (split aerc's `lib/` into focused packages)
  — those packages disappear with the fork.
- Supersedes ADR-0012 (inherited global `WorkerMessages` channel)
  — the channel disappears with the fork; multi-account in Pass
  11 gets per-account backends with no shared state.
- Pass 3 reshapes: was "wire prototype to live backend via
  existing JMAP adapter," becomes "implement direct-on-libraries
  JMAP backend; delete `internal/mailworker/`; wire live."
  Pass 8 (Gmail) gains the IMAP rewrite.
- Pass 9 (compose + send) inherits `emersion/go-smtp` as the
  obvious submission path with no additional research.
- Post-1.0 contacts inherit `emersion/go-webdav` + `go-vcard`
  from the same library family.
- Single-maintainer risk: emersion (one developer, decade-long
  track record across the full stack) and rockorager (one
  developer, used in production by aerc). Bounded by the
  library-shape mechanical nature of these protocols and our
  ability to fork either library if abandoned.
- BACKLOG #10 is closed by this ADR.
