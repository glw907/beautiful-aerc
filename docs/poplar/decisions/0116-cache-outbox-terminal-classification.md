---
title: Cache 0 — outbox terminal classification (auth, max-attempts, crashed-mid-execute)
status: accepted
date: 2026-05-02
---

## Context

ADR-0112 specified `failed → pending` after backoff with
`max-attempts = 0` (retry forever) as the default, and
`crashed-mid-execute` for non-idempotent ops resetting to `failed`
on restart. Pass 8.4-review found three cases where this loops
silently:

- **D3** — OAuth token expires mid-drain; the drainer sees 401,
  marks `failed`, backs off, retries with the same expired token,
  loops forever. ADR-0108 defers token refresh to Pass 9.6, so
  there's no in-band fix; the drainer must classify the failure
  out of the retry loop.
- **B6** — Permanent server-side failures (deleted destination
  folder returning 5xx instead of 404, revoked credential, server
  quota) loop forever under `max-attempts = 0`. K-9 ships a
  distinct `RETRIES_EXCEEDED` terminal state for exactly this.
- **B5** — A `send` op that keeps crashing mid-execute under the
  pre-revise spec cycles `executing → failed
  (crashed-mid-execute) → pending → executing → ...` forever on
  reproducible crashes. Crashed-mid-send must require explicit
  user action.

## Decision

Three terminal-classification rules, codified in spec §D.3 / §D.4:

1. **Auth errors → conflict.** 4xx authentication errors map to
   `status = 'conflict'`, `error.kind = 'auth-failure'`. Bypasses
   the backoff loop. User resolves via the `!` overlay (or by
   restarting `password-cmd` and retrying); Pass 9.6's token-
   refresh story will handle this in-band but does not change the
   terminal classification.

2. **Max-attempts cap.** Default `max-attempts = 10`. When
   `attempts >= max-attempts > 0`, the drainer transitions
   `failed → conflict` with
   `error.kind = 'max-attempts-exceeded'`. `max-attempts = 0`
   (unlimited) remains opt-in for users on intermittent links who
   want indefinite retry.

3. **Crashed-mid-execute send → conflict.** Non-idempotent ops
   (`send`) reset from `executing` to `conflict` (not `failed`)
   on startup, with `error.kind = 'crashed-mid-execute'`. Pass 9
   schema and Conflicts-overlay UX accommodate this.

All three cases land in the same `!` Conflicts overlay with
`retry` / `discard` actions.

## Consequences

- Supersedes ADR-0112's failure-handling section. The state
  machine itself (`pending → executing → done | conflict |
  failed → pending`) is unchanged; the classification rules above
  govern which terminal each failure lands in.
- The `error.kind` vocabulary now includes: `auth-failure`,
  `max-attempts-exceeded`, `crashed-mid-execute`,
  `rekey-orphaned` (ADR-0114), `anchor-lost` (ADR-0114), plus
  backend-supplied conflict messages. Part of the v1.0-frozen
  outbox-error vocabulary.
- Default config changes: `max-attempts = 10` instead of `0`. The
  pre-revise default would have shipped a silent infinite-retry
  trap to users; this default surfaces permanent failures in 10
  attempts (≈ a few minutes under default backoff) instead of
  never.
- Existing users on intermittent connections who want the
  pre-revise behavior set `max-attempts = 0` explicitly.
- The classification is implementation-driven (the drainer maps
  backend errors to `error.kind`). Backend impls must surface
  enough error detail (HTTP status code for JMAP; tagged response
  + auth-context for IMAP) for the drainer to classify reliably.
