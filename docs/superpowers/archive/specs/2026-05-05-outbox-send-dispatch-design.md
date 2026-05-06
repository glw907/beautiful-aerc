# Pass 9g — Cache Outbox Send/Append Dispatch (Design)

Status: accepted
Date: 2026-05-05

## Context

Pass 9f landed `Backend.Send(env, mime)` and `Backend.Append(folder,
mime, flags)` on `mail.Backend` (ADR-0157). The cache outbox already
carries `KindSend` / `KindAppend` enum values (Cache III) and the
crash-recovery path routes both to `OpConflict crashed-mid-execute`,
but the typed args (`SendArgs`, `AppendArgs`) are placeholder empty
structs and the drainer's `dispatch` switch has no cases for them.
Pass 9g wires the dispatch end-to-end so that, once Pass 9h adds
ComposeTab, hitting send queues real ops that actually transmit.

## Decision

### Args shapes

```go
type SendArgs struct {
    Envelope mail.Envelope // From + Rcpts (RFC 5321)
}

type AppendArgs struct {
    Flag mail.Flag // OR'd flag bits to set on APPEND
}
```

Both remain JSON-encoded into `outbox.args` like every other
`OpArgs`. The MIME payload itself does *not* live in `args`. The
destination folder for Append is `outbox.folder` (already an FK
into `folders`); reusing it avoids a second source of truth. The
SendArgs row's `outbox.folder` is the Sent folder by convention
so `OutboxSummary` groups sensibly.

### MIME payload — schema v6

A new column `outbox.payload BLOB` carries the assembled MIME bytes.
NULL for `KindMove`/`KindFlag`/`KindDestroy`; required for
`KindSend`/`KindAppend`. Schema version bumps to 6.

Bytes are assembled once, at hit-Send time, by
`compose.AssembleMIME(draft, now)` and stored verbatim. The drainer
reads them on dispatch. Reassembly on dispatch is rejected: the
assembler takes `time.Now()` (Message-ID, Date, boundary tokens) and
re-running on a different clock produces a different message;
attachments referenced by filesystem path may be deleted, moved, or
modified between enqueue and dispatch. Locking in the bytes at
queue time matches user intent — "the message I just sent is the
one that arrives."

### Queue path

A new payload-bearing entry point:

```go
func (a *Account) QueueSend(ctx context.Context, sentFolder string, env mail.Envelope, mime []byte) (int64, error)
func (a *Account) QueueAppend(ctx context.Context, folder string, flag mail.Flag, mime []byte) (int64, error)
```

Both folder-scoped (no msgUID), both insert with `payload = ?`. They
share an internal helper that mirrors today's `QueueOp` minus the
optimistic-UI flip and the message-row resolution. `QueueOp` keeps
its existing signature for Move/Flag/Destroy callers. `sentFolder`
on `QueueSend` is the canonical Sent folder name; on JMAP it is
informational (server lands the Sent copy itself), on IMAP it
matches the folder ComposeTab targets with the follow-up Append.

### Drainer dispatch

`outboxRow` gains `Payload []byte` (read alongside the existing
columns). `dispatch` grows to take the row so it can read the
folder name and payload:

```go
func (a *Account) dispatch(args OpArgs, row *outboxRow) error {
    switch v := args.(type) {
    // ...existing cases use row.ProtocolID for uids...
    case SendArgs:
        return a.Backend.Send(v.Envelope, row.Payload)
    case AppendArgs:
        return a.Backend.Append(row.FolderName, row.Payload, v.Flag)
    }
}
```

`decodeArgs` gains the matching JSON-decoding cases.

`finalizeSuccess` already handles `!row.MessageID.Valid` (folder-
scoped ops) by writing only the outbox terminal state — no message-
table mutation. Send and Append fall through that branch unchanged.

### Optimistic UI / Discard

Send and Append have no optimistic message-row state in 9g (no
ComposeTab yet, no draft persistence). `applyOptimisticTx` already
no-ops on these args via its default case, so QueueSend/QueueAppend
need nothing extra.

`revertOptimisticTx` currently *errors* on SendArgs/AppendArgs
("not supported"). That changes to a no-op return, which makes
`DiscardOp` work correctly for conflicted Send/Append rows — the
discard path will:
1. Read kind+args+payload, decode args.
2. `revertOptimisticTx` no-ops (no msgID, no UI state).
3. Delete the outbox row (payload deleted with it).

That gives the user a real escape hatch on a conflicted Append-to-
Sent: discard accepts the missing Sent copy.

### IMAP two-step semantics

ComposeTab in Pass 9h will enqueue, for IMAP accounts, two ops in
order: `QueueSend` then `QueueAppend(folder=Sent, flag=Seen)`. JMAP
enqueues only `QueueSend` (server-side `Email/import` +
`EmailSubmission/set` lands the Sent copy atomically per ADR-0157).

If Send succeeds and the subsequent Append fails, each op follows
its own normal lifecycle: Send → `OpDone`, Append → `OpFailed`
(retry with backoff) → `OpConflict max-attempts-exceeded`. The
outbox-visibility surface (status-bar depth, conflicts list) makes
the partial state visible. Retry resets attempts; Discard accepts
the missing Sent copy. No special atomic-pair construct.

### Crash recovery

Already correct: `recoverExecuting` routes any `executing` row of
kind Send or Append to `OpConflict crashed-mid-execute`. No change.

### Backoff / max-attempts

Same defaults as today (1s→60s exponential, 10 attempts). Send and
Append don't get bespoke caps in 9g — if a recipient's MX is down
for an hour, the op cycles through retries and lands in conflict;
the user retries from the outbox surface. Tuning is post-9h work
informed by real usage.

### Tests

- `ops_test.go` (or new `send_test.go`): QueueSend/QueueAppend
  round-trip — args decode, payload bytes preserved bit-exact,
  outbox row shape correct.
- `drainer_test.go`: success cases for Send and Append against a
  fake backend; transient-error retry path; auth-failure → conflict;
  partial-failure scenario (Send done, Append fails → conflict).
- `conflicts_test.go`: DiscardOp succeeds on conflicted SendArgs /
  AppendArgs rows (regression test for the revert no-op change).
- Schema migration test: v5 DB upgrades to v6 with the new column.

## Out of scope

- ComposeTab UI (Pass 9h).
- Draft persistence across restart (Pass 9h.5).
- Per-account outbox payload size cap. Today's body cache has
  `MaxSize`; the outbox is unbounded by design (user's outbound
  intent shouldn't be evicted). If real usage shows abuse the cap
  comes later.
- Outbox surfacing (status-bar segment, conflicts list) already
  built in Cache III; no UI changes here.

## Consequences

- Schema v6 migration is forward-only; downgrade story is "delete
  the cache." Pre-beta posture endorses this.
- `cache.Account` API gains two methods, keeping `QueueOp` clean of
  the payload concern.
- The drainer payload read costs one extra column per row — trivial.
- Once 9g lands, Pass 9h's ComposeTab can dispatch real sends with
  no further cache changes.

## ADR

ADR-0158 records the payload column, the envelope-only args, the
two-step IMAP enqueue contract, and the standard-conflict semantics
for partial failure. Written at pass end.
