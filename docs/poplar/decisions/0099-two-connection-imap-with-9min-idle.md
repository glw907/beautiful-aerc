---
title: Two-connection IMAP backend with 9-minute IDLE refresh
status: accepted
date: 2026-05-02
---

## Context

`mail.Backend` is synchronous (ADR-0075). IMAP IDLE blocks until
the server tears down the connection or the client sends DONE,
which conflicts with synchronous command dispatch on the same
socket. RFC 2177 also caps IDLE at 29 minutes; many servers (Gmail
in particular) drop idle connections sooner.

## Decision

Each `mailimap.Backend` owns **two physical IMAP connections**:

- **Command connection** (`b.cmd`) — used by every blocking
  `mail.Backend` method. Strictly synchronous; the package mutex
  serializes access.
- **Idle connection** (`b.idle`) — owned by `idleLoop` (a single
  goroutine started in `finishConnect`). Blocks in `Idle(...)`
  emitting `mail.Update` values; refreshes every **9 minutes**
  (well below RFC 2177's 29-minute cap and Gmail's lower cap) by
  sending DONE, re-`Select`ing the current folder, and re-issuing
  IDLE. Folder switches arrive via `b.switchCh` (buffered 1).
  Reconnect uses exponential backoff (1s → 60s, mirroring
  `mailjmap.pushLoop`). Servers without IDLE fall back to a 30s
  poll loop driven by `STATUS UIDNEXT` deltas.

Both connections share the `dial(cfg, role)` path in `auth.go`,
which applies kernel keepalive (TCP_KEEPIDLE / KEEPINTVL /
KEEPCNT) so dead-router scenarios surface in seconds rather than
hours.

## Consequences

- Backend struct gains `cmd`, `idle`, `idleCancel`, `idleDone`,
  `switchCh`, and `updates` channel fields.
- `Disconnect` must signal `idleCancel`, wait on `idleDone`, then
  Logout both connections.
- Folder selection on `cmd` does not implicitly affect `idle` — it
  must be propagated through `switchCh`.
- Tests use the `imapClient` interface to swap a fake into both
  slots; the real adapter (`realClient`) wraps `imapclient.Client`
  v2.
- The idle goroutine never touches `b.mu` from inside an IDLE
  callback — unilateral data is dispatched onto `b.updates` and
  consumers handle reconciliation.
