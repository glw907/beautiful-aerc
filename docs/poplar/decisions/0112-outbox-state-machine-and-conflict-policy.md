---
title: Cache 0 — outbox queue, replay state machine, and conflict policy
status: accepted (sync ordering superseded by 0113; failure handling superseded by 0116; UIDVALIDITY contract narrowed by 0114)
date: 2026-05-02
---

## Context

The outbox is the action queue produced by `cache.QueueOp` and drained
by a per-account goroutine. Its schema, replay strategy, and conflict
policy define the v1.0-frozen contract for offline triage and become
the foundation for Pass 9's send semantics.

Source-level reads of FairEmail (`EntityOperation.java`,
`ServiceSend.java`), Mailspring-Sync (`TaskProcessor.cpp`), and
Thunderbird desktop (`nsIMsgOfflineImapOperation.idl` +
`nsImapOfflineSync.cpp`) informed this decision. The major
architectural fork was queue-row references: by message-row id (FK
CASCADE) vs. by IMAP UID + UIDVALIDITY pair. Thunderbird and K-9 use
the latter; FairEmail uses the former and dissolves the UIDVALIDITY
problem entirely.

A second fork was replay strategy: enqueue-order replay (FairEmail,
Mailspring) vs. type-ordered with automatic coalescing (Thunderbird).
Coalescing buys IMAP/JMAP round-trip efficiency at the cost of losing
sequential intent and complicating error attribution.

A third fork was post-replay retention: delete-on-success (Thunderbird,
Mailspring) vs. retain-as-audit (none surveyed).

## Decision

### Schema

Outbox rows reference `messages.id` (the cache row id), not the
protocol UID/JMAP id. Foreign keys CASCADE on delete. Status is one
of `pending | executing | done | conflict | failed`. Args are a
type-specific JSON blob.

Schema is defined in full in
`docs/superpowers/specs/2026-05-02-cache-0-design.md` section A.3.

### Replay state machine

```
pending ─▶ executing ─▶ done | conflict | failed ─▶ pending (after backoff)
```

- `pending` — newly enqueued; awaiting drainer.
- `executing` — drainer running this op against the backend.
- `done` — successful. GC'd after `outbox-retention` (default 7d).
- `conflict` — backend reported irreconcilable state (e.g., dest
  folder gone). Stays until user resolves via `!` overlay or
  `poplar cache resolve`.
- `failed` — transient failure. Backoff 1s, 2s, 4s, …, capped at
  `backoff-max` (default 60s). Returns to `pending` on next drainer
  tick after backoff window elapses. `max-attempts = 0` (default)
  retries forever; user can cap.

### Crash recovery

On startup, `executing` rows are reclassified by op-kind
idempotency:

- **Idempotent** (`move`, `flag`, `destroy`) → reset to `pending`.
- **Non-idempotent** (`send`) → reset to `failed` with
  `error.kind = 'crashed-mid-execute'`; user resolves manually.

### Replay strategy

Per-account, single goroutine, strict enqueue order (`ORDER BY id`).
Cross-account: parallel.

`ChangeTracker.Changes()` runs **before** draining the queue on
reconnect. Remote-removed UIDs CASCADE-delete dependent outbox rows
before we attempt to replay against them.

**No drain-time coalescing.** Mailspring runs in production without
it. JMAP `Email/set` is naturally batched at the protocol level — the
JMAP backend implementation can coalesce when constructing the
request. IMAP triage volumes don't exhibit meaningful per-op
latency on a long-lived connection. If profiling later proves
otherwise, coalescing belongs at the backend, not the queue.

### Conflict policy

| Replay-time observation | Outcome |
|---|---|
| Joined `messages` row already deleted (CASCADE from remote-removed) | Outbox row already gone; no work. |
| `move(msg, dest)` and current folder == dest | `done` (idempotent). |
| `move(msg, dest)` and current folder != enqueue folder | Apply our move from current location (apply-ours, last-writer-wins). |
| `move(msg, dest)` and dest folder gone | `conflict`. |
| `flag(msg, F, set)` and server already in target state | `done` (idempotent). |
| `flag(msg, F, set)` and server in opposite state | Apply ours (last-writer-wins). |
| `destroy(msg)` (any remote state) | `done` (JMAP `notFound` = success per spec; IMAP UID EXPUNGE naturally a no-op). |
| Backend `notFound` for any kind | `done` (idempotent). |
| Backend network error | `failed` + backoff. |

UIDVALIDITY change handling: none in the queue. The folder-sync code
re-keys `messages.protocol_id` on UIDVALIDITY change. Outbox rows
reference `messages.id`, read `protocol_id` fresh at replay, and pick
up the new UID transparently. If a message row is deleted because the
folder was reindexed and the protocol couldn't remap, CASCADE removes
the outbox row.

## Consequences

- The outbox schema and state machine are part of the v1.0-frozen
  data format. Fields can be added in pre-beta (ADR-0105); shape is
  fixed at beta-soak.
- Send (Pass 9) fits this model: kind `send` with `executing` state
  for crash detection, no need for separate outbox-for-mail
  scaffolding.
- UIDVALIDITY changes — the source-vetted failure mode that wipes
  Thunderbird's offline queue and hard-stops offlineimap/mbsync
  folders — are structurally absent from this design.
- The `!` Conflicts overlay and `Q` Outbox overlay (Cache III) are
  TUI-native UX inventions; no surveyed client ships per-op cancel
  or per-conflict resolve UIs. Defensible as a TUI affordance;
  flagged for review-pass scrutiny.
- Adding a new op kind is `INSERT` + drainer dispatcher case, no
  schema migration.
- "Apply ours" on flag conflicts is universal among OSS clients
  (FairEmail, K-9, Thunderbird, Mailspring); only Outlook is
  server-wins. We follow the OSS majority; documented for future
  review if user complaints emerge.
- **Superseded by ADR-0113** — sync ordering is drain-first, not
  sync-first (RFC 4549 §6). Plus a syncer/drainer coordination
  invariant: the syncer MUST NOT update `ui_flags` for messages
  with a `pending`/`executing` outbox row.
- **Superseded by ADR-0116** — failure classification:
  `max-attempts = 10` default with `failed → conflict
  (max-attempts-exceeded)` on cap; auth errors → `conflict
  (auth-failure)`, bypassing backoff; crashed-mid-execute send →
  `conflict`, not `failed`.
- **Narrowed by ADR-0114** — the bare "no UIDVALIDITY-specific
  code in the queue" framing is insufficient; the IMAP folder
  sync code now has a binding contract (atomic re-key, connection
  fence, explicit promotion of orphaned rows to `conflict`). The
  queue still references `messages.id` and remains
  UIDVALIDITY-agnostic; the contract just defines what happens to
  rows whose `messages` row can't survive re-anchor.
