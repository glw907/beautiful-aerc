---
title: Cache outbox Send/Append dispatch
status: accepted
date: 2026-05-06
---

## Context

Pass 9f reshaped `mail.Backend.Send`/`Append` (ADR-0157) but the
cache outbox could not yet route to them: `KindSend`/`KindAppend`
existed as enum values from Cache III, the crash-recovery path
already routed both to `OpConflict crashed-mid-execute`, but
`SendArgs`/`AppendArgs` were placeholder empty structs and the
drainer's `dispatch` switch had no cases. Pass 9g wires the
dispatch end-to-end so Pass 9h's ComposeTab can hand assembled
MIME bytes to the cache and have them transmitted with no further
backend changes.

## Decision

Schema v6 adds `outbox.payload BLOB NULL` carrying the assembled
MIME bytes for `KindSend`/`KindAppend` (NULL for the existing
Move/Flag/Destroy kinds). Bytes are assembled once, at hit-Send
time, by `compose.AssembleMIME(draft, now)` and stored verbatim;
reassembly on dispatch is rejected because the assembler takes
`time.Now()` (Message-ID, Date, boundary) and attachments
referenced by filesystem path can move between enqueue and
dispatch. Locking the bytes in at queue time matches user intent.

`SendArgs{Envelope mail.Envelope}` and `AppendArgs{Flag mail.Flag}`
become envelope-only typed shapes. The MIME body lives in
`outbox.payload`; the destination folder lives in `outbox.folder`
(reusing the existing FK avoids a second source of truth). Send's
`outbox.folder` is the canonical Sent folder by convention so
`OutboxSummary` groups sensibly.

`(*Account).QueueSend(ctx, sentFolder, env, mime)` and
`(*Account).QueueAppend(ctx, folder, flag, mime)` are the new
payload-bearing entry points. Both share an internal
`insertFolderOp` helper that mirrors `QueueOp` minus the
optimistic-UI flip and the message-row resolution. `QueueOp`
keeps its existing signature for Move/Flag/Destroy. Inserts skip
optimistic UI because Send/Append have no message-row state to
mirror in 9g.

Drainer dispatch grows to take the full row so it can read the
folder name and payload: `dispatch(args, row)` switches on the
typed args and calls `Backend.Send(v.Envelope, row.Payload)` or
`Backend.Append(row.FolderName, row.Payload, v.Flag)`.
`decodeArgs` gains the matching JSON cases.

`revertOptimisticTx` no-ops on Send/Append (was an error). That
makes `DiscardOp` work on conflicted Send/Append rows: the
discard reads kind+args, the revert no-ops (no msgID, no UI
state), and the row deletes — payload deleted with it. The user
gets a real escape hatch on a conflicted Append-to-Sent.

For IMAP accounts, Pass 9h's ComposeTab will enqueue two ops in
order: `QueueSend` then `QueueAppend(folder=Sent, flag=Seen)`.
JMAP enqueues only `QueueSend` (server lands the Sent copy
atomically per ADR-0157). On partial failure (Send done, Append
fails) each op follows its own normal lifecycle; the
outbox-visibility surfaces (status-bar depth, conflicts list)
make the partial state visible. No special atomic-pair construct
— retry resets attempts, discard accepts the missing Sent copy.

## Consequences

ComposeTab (Pass 9h) dispatches real sends with no further cache
changes. Schema v6 is forward-only; downgrade story is "delete
the cache" (pre-beta posture endorses this). The drainer payload
read costs one extra column per row — trivial. The outbox is
unbounded by design: a per-account payload size cap is deferred
until real usage shows abuse, since user outbound intent
shouldn't be evicted.
