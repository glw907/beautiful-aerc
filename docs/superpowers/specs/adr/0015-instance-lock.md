# ADR-0015: second instance refuses with an actionable message

Date 2026-07-27. Status: accepted (Phase 4). Resolves the SY-7
open pick.

## Context

SY-7 requires detecting a second poplar against the same store and
either attaching read-only with a banner or refusing with an
actionable message, and lets Phase 4 pick. The store has one
writer by construction (ADR-0003).

## Decision

gofrs/flock takes `LOCK_EX|LOCK_NB` on a lock file beside the
store at startup. A second instance refuses to start, printing
which process holds the lock (pid from the lock file's advisory
metadata) and what to do. The kernel releases flock on any
process death including SIGKILL, so no stale-lock heuristics
exist.

## Alternatives considered

- **Read-only attach with a banner**: every mutation path in the
  UI grows a read-only gate for a rare case (two deliberate
  launches); the honest cost is a whole degraded mode to design,
  test, and keep from rotting. Refusal costs the user one
  keypress in the first terminal.
- **PID files**: stale-file heuristics after kill -9; flock's
  kernel-release property deletes the whole problem class.
- **Socket claim**: portable-ish but heavier, and it conflates
  instance identity with IPC poplar does not have (C3: no
  daemon).

## Consequences

The SY-7 test launches a second instance against a live store and
asserts refusal plus outbox integrity. If a genuine read-only use
case emerges post-v1 (a pager over the store), it arrives as its
own decision, not as a degraded mode of the client.
