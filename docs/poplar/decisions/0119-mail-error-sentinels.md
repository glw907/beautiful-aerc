---
title: Backend error sentinels — mail.ErrAuth, mail.ErrNotFound
status: accepted
date: 2026-05-02
---

## Context

Spec §D.4 (the conflict matrix) treats authentication failure and
"message gone before we got there" as distinct outcomes from
generic transient errors. The drainer needs to route on the
distinction: auth-failure → conflict (`auth-failure` kind, bypasses
backoff per ADR-0116); not-found → idempotent success
(`done`); transient → failed with backoff.

The first cut of the drainer (this pass) used substring matching
on the rendered error message — `strings.Contains("auth", "401",
"unauthorized", …)`. That's fragile (a server message containing
"unauthorized change" would match), and it pushes protocol-shape
knowledge into the cache package.

## Decision

Two typed sentinels live in `internal/mail`:

- `mail.ErrAuth` — wraps any backend authentication failure (HTTP
  401/403 from JMAP, IMAP `AUTHENTICATIONFAILED`/`AUTHORIZATIONFAILED`/
  `PRIVACYREQUIRED`, expired OAuth token).
- `mail.ErrNotFound` — wraps "the message you asked about is gone"
  (HTTP 404, IMAP `NONEXISTENT`).

Each backend ships a `classifyErr(err)` helper that uses
`errors.As` against the upstream library's typed error
(`*jmap.RequestError`, `*imap.Error`) and wraps with the sentinel
on a recognized shape. JMAP wires this through a `(*Backend).do`
shim that replaces every `b.client.Do(req)` call site. IMAP wraps
at every `fmt.Errorf("...: %w", err)` return in `actions.go`,
`messages.go`, and `folders.go`.

The cache drainer routes via `errors.Is(err, mail.ErrAuth)` and
`errors.Is(err, mail.ErrNotFound)`. The substring scanner is
deleted.

## Consequences

- Adding a backend (or upgrading a backend library that surfaces
  new error shapes) requires extending its `classifyErr` rather
  than touching the cache. Protocol knowledge stays at the
  protocol boundary.
- A backend that returns an unwrapped auth error degrades to
  "transient → failed → backoff loop" until `max-attempts`
  promotes it to conflict. Acceptable: the conflict still
  surfaces, just with worse latency.
- Future failure modes (rate-limit, quota, server-error) get the
  same pattern — add a sentinel, wrap at the boundary, route in
  the drainer.
