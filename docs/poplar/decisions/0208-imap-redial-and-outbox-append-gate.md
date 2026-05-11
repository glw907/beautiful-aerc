---
title: IMAP connection-dead recovery and outbox Append gating
status: accepted
date: 2026-05-11
---

## Context

Two soak-blocking IMAP-side bugs surfaced in the 2026-05-11 audit:

**#53.** `mailimap.idleLoop` retried `runIdleSession` against the
same `*imapclient.Client` pointer that was set once in `Connect`.
When the TCP connection actually died (laptop sleep/wake, NAT
reap, transient partition past the 9-min IDLE refresh), the loop
hammered a dead handle forever, posting `ConnReconnecting` with
no recovery. The cmd connection had the symmetric defect: every
action (`Move`/`Flag`/`Destroy`/`Copy`/`Store`/`Expunge`/
`Append`) called `b.cmd.X(...)` against a pointer that was only
ever assigned in `Connect`.

**#52.** IMAP `QueueOutbound` enqueues `[send, append]` as two
independent outbox rows. The drainer picks `ORDER BY o.id LIMIT 1`
with no row-to-row dependency, so if `Backend.Send` failed and
transitioned to `OpFailed` with a future `next_eligible_at`, the
sibling Append (still `OpPending`) would dispatch and put a
never-sent message in the Sent folder. JMAP is unaffected — its
`Email/import` + `EmailSubmission/set` batch atomically via the
`#k1` creation reference.

## Decision

### Connection-dead detection and redial (#53)

A new `mail.ErrConnection` sentinel sits beside `ErrAuth` and
`ErrNotFound`. `mailimap.classifyErr` routes `io.EOF`,
`io.ErrClosedPipe`, `io.ErrUnexpectedEOF`, `net.ErrClosed`,
`*net.OpError`, and `net.Error.Timeout()` to it via the shared
`mail.WrapSentinel` helper (the previous `joined` types in
`mailimap` and `mailjmap` collapsed into one canonical wrapper).

The IMAP backend gains a per-instance dial seam (`b.dialFn`,
defaulting to the package-level `dial`) so tests can swap in
fakes without touching package globals. `Connect` stores a
`connCtx` (cancelled in `Disconnect`) used as the parent context
for redials.

`cmdClient()` is the new accessor every action uses: returns the
cached `b.cmd` when set, otherwise dials a fresh command
connection, swaps it in under the mutex (race-loser closes its
duplicate, mirroring `smtpClientLocked`), and re-selects
`b.current` on the new conn. `dropCmd(c)` clears `b.cmd` iff it
still points at `c` and logs the dead handle out. Every action's
error path runs `maybeDropOnConn(cmd, wrapped)` so a connection-
classified error drops the cache, and the next call dials fresh.

`idleLoop` checks for a live `b.idle` at the top of each
iteration (dialing via `dialIdle(ctx)` if absent), runs the
session, and on session error drops the dead handle, sleeps the
exponential backoff, and loops. Clean refresh returns reuse the
existing handle.

The cache drainer needs no new branch — `ErrConnection` falls
through to the transient default, exactly the desired behavior.

### Append gate (#52)

`nextOutboxRow` gains a `NOT EXISTS` subquery: a draft-linked
`KindAppend` row is ineligible until a sibling `KindSend` row
with the same `draft_id` reaches `OpDone`. The gate also closes
when the sibling row is absent — a stranded Append never fires
on its own. Append rows with `draft_id IS NULL` (rare, not
emitted by `QueueOutbound`) ignore the gate.

`DiscardOp` on a draft-linked `KindSend` cascade-deletes any
sibling `KindAppend` rows for the same draft inside the same
transaction. Without the cascade, discarding a conflicted Send
would strand the Append (gate requires `status = 'done'`, never
reached).

No schema change. The Send `OpDone` row persists (there is no
background cleanup), so the gate has a stable observable.

## Consequences

- IMAP recovers automatically from any TCP-level connection loss
  on both the idle and command paths. The "Reconnecting…"
  banner becomes a transient state rather than a stuck one.
- The drainer can never put an un-sent message in Sent.
- `mail.ErrConnection` is available to JMAP if push-loop dead-
  connection detection ever wants to use it; today the JMAP HTTP
  transport's keep-alive errors are handled by the drainer's
  transient retry path and do not need explicit classification.
- The `joined`/`errJoin`/`wrapSentinel` duplication in
  `mailimap` and `mailjmap` collapses into `mail.WrapSentinel`.
