---
description: Cache-layer binding facts for poplar's per-account SQLite store
paths:
  - "internal/cache/**/*.go"
  - "cmd/poplar/cache*.go"
  - "docs/superpowers/plans/**/*.md"
  - "docs/superpowers/specs/**/*.md"
---

# Poplar Cache Invariants

Binding facts for `internal/cache/`. Loaded when editing cache
sources, the `poplar cache` CLI, or planning passes that touch
the cache. The decision index in `docs/poplar/invariants.md` maps
each fact back to its ADR(s).

## Cache

- `internal/cache` is the on-disk store. One `*cache.Account` per
  email account; one SQLite database per account at
  `$XDG_CACHE_HOME/poplar/<slug>/mail.db` (slug = lowercased
  account name with non-`[a-z0-9-]` runs replaced by `-`).
  `modernc.org/sqlite` driver, WAL mode, foreign_keys ON,
  synchronous=NORMAL, busy_timeout=5000. Pool capped to
  4 open / 2 idle.
- Schema is versioned in `schema_version`; migrations run
  transactionally on `Open`. v1 installs the full Cache I shape
  (folders / messages / message_mailboxes / bodies / outbox per
  spec §A.3). v2 adds `outbox.next_eligible_at` so the drainer's
  pickup query filters the failed-row backoff window in SQL. v3
  adds `folders.exists_total`/`unseen_total` for unread badges on
  unopened folders. v4 drops `bodies.last_accessed` and the
  `bodies_lru` index (LRU eviction replaced by size backstop). v5
  adds the `attachments` table (metadata + lazy bytes; columns
  `id`, `message`, `part_id`, `filename`, `mime_type`, `size`,
  `content_id`, `disposition`, `bytes`, `fetched_at`; UNIQUE
  `(message, part_id)`; index on `message`).
- `mail.ChangeTracker` is the protocol-level change-detection
  sibling of `mail.Backend`; both v1 backends implement it. On a
  nil SyncToken both run an initial baseline pull. JMAP pages
  `Email/query` filtered by inMailbox (page size 500), piggybacks
  a sentinel-id `Email/get` per page to capture Email-type state
  in the same roundtrip, and returns all current Email IDs as
  Added; non-nil tokens route through account-scoped
  `Email/changes` per RFC 8621 §4.3. IMAP scan-and-diff
  (`UID SEARCH ALL` vs prior maxuid in SyncToken) handles nil
  tokens implicitly; CONDSTORE-aware incremental deferred.
  `Backend.FetchHeaders` chunks ids at 500 per `Email/get` to
  stay under `maxObjectsInGet`. UIDVALIDITY change →
  `mail.ErrCannotCalculateChanges` → cache re-anchor.
- `(*Account).FetchBody(ctx, uid)` is write-through with lazy
  population: cache miss → backend fetch → store; cache hit →
  return stored bytes. `storeBody` runs a size backstop: when an
  insert would push total stored size over `cache.Config.MaxSize`
  (default 2 GB from `[cache] max-size` in `config.toml`), it
  evicts the oldest messages by `messages.sent_at` inline.
  `Backend.FetchBody` returns `([]byte, error)` — no `io.Reader`.
- `(*Account).Attachments(ctx, uid)` and `FetchAttachment(ctx, uid,
  partID)` mirror the body pattern. Metadata populates lazily on
  first `Attachments` call (zero-length results are not cached);
  bytes populate lazily on first `FetchAttachment`. Bytes eviction
  runs a separate size backstop against
  `cache.Config.MaxAttachmentSize` (default 2 GB from `[cache]
  max-attachment-size` in `config.toml`), oldest by
  `messages.sent_at`, clearing `bytes`/`fetched_at` while keeping
  the metadata row. Bodies and attachments evict independently.
- `cache.Slugify`, `cache.DBPath`, `cache.OpenDB` are exported
  helpers used by `cmd/poplar/cache.go`; all path/DSN logic is
  canonical here.
- `(*Account).QueueOp(ctx, folder, msgUID, args)` is the single
  forward write entry. `OpArgs` is a sealed sum (`MoveArgs`,
  `FlagArgs`, `DestroyArgs`; reserved `SendArgs`, `AppendArgs`).
  Inside one transaction: resolve folder → row id, insert
  outbox row with `status='pending'` and `next_eligible_at=NULL`,
  apply optimistic `ui_flags`/`ui_hide` to the message row,
  commit, signal drainer. After the drainer marks a row
  `conflict`, `(*Account).RetryOp(ctx, opID)` and
  `(*Account).DiscardOp(ctx, opID)` are the user-initiated
  resolution primitives. Retry resets `attempts = 0` and signals
  the drainer. Discard reverts the optimistic flip via
  `revertOptimisticTx` (mirror of `applyOptimisticTx`) and
  deletes the outbox row in one transaction. Both reject
  non-conflict rows with `ErrNotConflict`.
- Cache exposes three read queries for the outbox-visibility
  surfaces: `OutboxSummary` (grouped by kind/folder/status, where
  folder is the destination for Move ops via `json_extract` and
  empty for other kinds), `OutboxConflicts` (decoded error JSON,
  ordered by enqueued_at ASC), `OutboxDepth` (counts per status).
- Outbox status is the typed `cache.OpStatus` enum
  (`OpPending`/`OpExecuting`/`OpDone`/`OpFailed`/`OpConflict`)
  stored as the underlying string. Op kind is `cache.OpKind`
  (`KindMove`/`KindFlag`/`KindDestroy`/`KindSend`/`KindAppend`).
  `CacheEvent` carries both as typed values.
- Drainer is per-account, single goroutine. Wakes on `drainSignal`
  (every QueueOp) or a 5-second idle ticker. Conflict matrix:
  success → `OpDone`; `ErrAuth` → `OpConflict auth-failure`;
  `ErrNotFound` → idempotent `OpDone`; transient → `OpFailed`
  with exponential `next_eligible_at`; `attempts >= max` →
  `OpConflict max-attempts-exceeded`. Crash recovery resets
  `OpExecuting`: idempotent kinds → `OpPending`;
  `send`/`append` → `OpConflict crashed-mid-execute`.
- Syncer/drainer coordination invariant (ADR-0113): the syncer's
  upsert path uses an `EXISTS (… outbox_message …)` guard so an
  in-flight pending/executing op's `ui_flags` is never reverted
  by a concurrent server-side change.
- `(*Account).Events()` is the drainer→UI signal channel
  (buffered 32). Drops on backpressure are counted in
  `(*Account).DroppedEvents()` so the UI can detect staleness
  and reconcile via a full cache re-read.
- `internal/backoff.Exponential(attempts, initial, max)` is the
  shared exponential-backoff helper used by the cache drainer,
  the JMAP push loop, and the IMAP idle reconnect loop. Returns
  `initial` on attempt ≤ 1; doubles each subsequent attempt;
  caps at `max`.
