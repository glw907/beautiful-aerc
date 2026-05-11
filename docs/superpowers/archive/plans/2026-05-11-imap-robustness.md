# Pass 24 — IMAP robustness

Two soak-blocking IMAP-side outbound / connection bugs. No schema
changes; both are mail-infra-only edits with regression tests.

## Scope

- `internal/mail/backend.go` — new `ErrConnection` sentinel.
- `internal/mailimap/` — `classifyErr` connection routing; cmd
  redial seam; idle redial path; backend dial-fn test seam.
- `internal/cache/ops.go` — `nextOutboxRow` gates `KindAppend`
  behind a draft-linked `KindSend` sibling reaching `OpDone`.
- `internal/cache/ops.go` — `DiscardOp` on a draft-linked Send
  cascade-deletes sibling Appends so the gate never strands
  orphans.

Out of scope: JMAP (atomic via `#k1`), drainer matrix (transient
default catches `ErrConnection`), background OpDone cleanup.

## #53 — IMAP IDLE / cmd redial

### Detection

`mail.ErrConnection` is the new sentinel, parallel to `ErrAuth` /
`ErrNotFound`:

```go
// ErrConnection signals the IMAP TCP connection died mid-call.
// Backends drop the cached client and lazily redial on next use;
// the drainer treats it as transient.
var ErrConnection = errors.New("mail: connection lost")
```

`mailimap.classifyErr` gains a connection check before the
existing `imap.Error` switch. A helper `isConnectionDead(err)`
matches `io.EOF`, `io.ErrClosedPipe`, `net.ErrClosed`, any
`*net.OpError`, and `net.Error.Timeout()` — mirrors what
`imapclient.Client.Close` itself filters at `client.go:405`.

### Cmd redial seam

Replace `cmd := b.cmd` boilerplate at every call site
(actions/folders/messages/attachments/changes, plus smtp.go's
Append/PushDraft) with `cmd, err := b.cmdClient()`. New helpers:

```go
func (b *Backend) cmdClient() (imapClient, error)
func (b *Backend) dropCmd(c imapClient)
```

- `cmdClient` returns the cached `b.cmd` when set, otherwise
  dials a fresh command connection via `b.dialFn(ctx, "command")`,
  swaps it in under the mutex, and re-selects `b.current` on the
  new conn. Race-loser closes its duplicate (mirrors
  `smtpClientLocked`).
- `dropCmd` clears `b.cmd` iff it still points at `c`, calls
  `Logout` on the dead client, and logs a `Warn`.

Every action's error path adds a single `errors.Is(...,
ErrConnection)` branch that calls `dropCmd(cmd)` before returning
the wrapped error. The action itself does *not* retry — the
caller (cache drainer for writes, UI for reads) drives the next
attempt.

`b.connCtx` (and `b.connCancel`) get stored in `Connect`; the cmd
redial uses it as the parent ctx for OAuth refresh during dial.
`Disconnect` cancels it after the idle wind-down.

### Idle redial path

`idleLoop` becomes:

```go
attempts := 0
for {
    if ctx.Err() != nil { return }
    err := b.runIdleSession(ctx)
    if ctx.Err() != nil { return }
    if err == nil {
        attempts = 0
        continue
    }
    b.emit(Update{ConnState: ConnReconnecting})
    attempts++
    b.log.Warn("imap idle session ended, will redial",
        "attempt", attempts, "err", err)
    b.dropIdle()                                       // drop dead handle
    delay := backoff.Exponential(attempts, reconnectInitial, reconnectMax)
    select {
    case <-ctx.Done(): return
    case <-time.After(delay):
    }
    if err := b.dialIdle(ctx); err != nil {
        b.log.Warn("imap idle redial failed", "err", err)
        continue                                       // backoff grows
    }
}
```

`dropIdle()` clears `b.idle` under the mutex and Logout's the
dead handle. `dialIdle(ctx)` calls `b.dialFn(ctx, "idle")` and
swaps the result in. `runIdleSession` keeps reading `b.idle` at
the top of each iteration — once `dialIdle` succeeds, the next
session sees the fresh handle.

### Test seam

Add a per-backend dial fn so tests can override without touching
package-level state:

```go
type Backend struct {
    ...
    dialFn func(ctx context.Context, role string) (imapClient, error)
}
```

Default: `b.dialFn = func(ctx, role) { return dial(ctx, b, role) }`,
wired in `New` / `NewWithOAuth`.

### Tests

- `TestIdleLoop_RedialsOnConnectionError` — first `Idle` returns
  `io.EOF`; assert `b.idle` swapped to a fresh fake; second
  session blocks on stop; updates are
  `ConnConnected → ConnReconnecting → ConnConnected`.
- `TestCmdClient_RedialsAfterDrop` — pre-set `b.cmd` to a fake;
  call `dropCmd`; next `cmdClient()` invokes `dialFn` once and
  returns the fresh fake.
- `TestActions_DropCmdOnConnectionError` — `fakeClient.Store`
  returns `io.EOF`; `Flag` wraps with `ErrConnection`; `b.cmd`
  is now nil; next `Flag` redials.

## #52 — Outbox Append gate

### nextOutboxRow gate

Append rows linked to a draft become ineligible until a sibling
Send for the same `draft_id` reaches `OpDone`. The gate also
holds when the sibling row is absent (e.g. discarded), so the
Append never fires without a confirmed Send.

```sql
WHERE (o.status = ?
       OR (o.status = ? AND (o.next_eligible_at IS NULL OR o.next_eligible_at <= ?)))
  AND (o.scheduled_for IS NULL OR o.scheduled_for <= ?)
  AND NOT (
      o.kind = 'append'
      AND o.draft_id IS NOT NULL
      AND NOT EXISTS (
          SELECT 1 FROM outbox s
          WHERE s.draft_id = o.draft_id
            AND s.kind = 'send'
            AND s.status = ?
      )
  )
ORDER BY o.id LIMIT 1
```

Extra parameter: `OpDone`. The gate is a no-op for Move / Flag /
Destroy / PushDraft / Contact* / and for Append rows with
`draft_id IS NULL` (manual `Append` callers that pre-date the
outbound batch — `mailcompose.Editor` doesn't queue these today,
but the gate stays narrow).

### Discard cascade

`DiscardOp` on a draft-linked `KindSend` row also deletes any
sibling rows with the same `draft_id` and `kind = 'append'`
inside the same transaction. Keeps the outbox from accumulating
orphan Appends that can never fire (since the Send sibling is
gone and the gate requires `status = 'done'`). RetryOp doesn't
need a sibling sweep — the Send goes back to OpPending and the
Append's gate stays closed until success.

### Tests

- `TestNextOutboxRow_AppendGatedByPendingSend` — Send pending,
  Append pending: pickup yields Send.
- `TestNextOutboxRow_AppendGatedByFailedSend` — Send failed
  (eligibility expired), Append pending: pickup yields Send,
  never Append.
- `TestNextOutboxRow_AppendReleasedAfterSendDone` — Send done,
  Append pending: pickup yields Append.
- `TestNextOutboxRow_AppendWithoutDraftID_NotGated` — Append
  with NULL draft_id ignores the gate (back-compat).
- `TestDiscardOp_Send_CascadesToAppend` — queue
  Send+Append linked by draft_id; mark Send OpConflict;
  DiscardOp(Send) deletes both rows.

## ADRs

One ADR — **ADR-0208 — IMAP connection-dead recovery and outbox
Append gating** — covers both fixes since they share the soak
context. Sections: `mail.ErrConnection` sentinel; cmd/idle
drop-and-redial pattern; Append gate semantics; discard cascade.

## Invariants

`invariants.md` IMAP block: add `mail.ErrConnection` sentinel
line, mention drop-and-redial under `b.cmdClient()` / `dialIdle`,
re-select on cmd redial.

`.claude/rules/cache-invariants.md`: Append's draft-linked gate
in the drainer's pickup paragraph; discard cascade in the
RetryOp/DiscardOp paragraph.

## Pass-end ritual

Standard. Idiomatic-bubbletea check is N/A (no `internal/ui/`
changes).
