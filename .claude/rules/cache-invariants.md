---
description: Cache-layer binding facts for poplar's per-account SQLite store
paths:
  - "internal/cache/**/*.go"
  - "cmd/poplar/cache*.go"
---

# Poplar Cache Invariants

Binding facts for `internal/cache/`. Loaded when editing cache
sources or the `poplar cache` CLI. The decision index in
`docs/poplar/invariants.md` maps each fact back to its ADR(s).

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
  `(message, part_id)`; index on `message`). v6 adds
  `outbox.payload BLOB NULL` carrying the assembled MIME bytes
  for `KindSend`/`KindAppend` (NULL for Move/Flag/Destroy). v7
  adds the `drafts` table (`draft_id` PK, `server_uid` nullable
  pointer at the server-side image, `server_folder`, `payload`
  BLOB holding the gob-encoded `compose.Draft`, `dirty`,
  `created_at`, `updated_at`, `last_pushed_at`) plus a partial
  index `drafts_by_server_uid` on non-null `server_uid`. v8 adds
  the contacts cache: `addressbooks` (href PK + display_name +
  sync_token + ctag + supports_sync + last_synced_at), `contacts`
  (uid PK, vCard blob + projection columns, FK addressbook_href
  ON DELETE CASCADE), `contact_emails` and `contact_phones`
  (FK contact_uid ON DELETE CASCADE, label, pref order, NOCASE
  index on email address), and `message_recipients` (`message_uid
  INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE`
  since v13; column name is historical, value is `messages.id`),
  role IN ('from','to','cc'), address, name, sent_at; PK
  (message_uid, role, address); NOCASE+sent_at-DESC index for the
  ranking query). Migration backfills `message_recipients` from
  existing `messages` rows in the same transaction. v9 drops
  `NOT NULL` from `outbox.folder` so contact ops (folderless) sit
  alongside mail ops in the same outbox; the drainer's pickup
  query `LEFT JOIN`s `folders` rather than inner-joins. v10 adds
  `outbox.scheduled_for` (unix nanos, user-intent dispatch hold)
  and `outbox.draft_id TEXT REFERENCES drafts(draft_id) ON DELETE
  SET NULL`. The drainer pickup gate becomes `now >=
  COALESCE(scheduled_for, 0) AND now >= COALESCE(next_eligible_at,
  0)`; `outbox_pickup` is rebuilt with `scheduled_for` as the
  leading column. Send/Append's OpDone transition runs in one tx
  with `DELETE FROM drafts WHERE draft_id = ?` when draft_id is
  set. v11 adds `messages_fts` (FTS5; subject/from/to/cc/body)
  and backfills both header columns and body text (via
  `content.ExtractPlainText` over cached `bodies.bytes`) in one
  tx. v12 drops `messages.date_str`. v13 rebuilds
  `message_recipients` with FK + CASCADE to `messages(id)` (the
  copy step's JOIN drops pre-existing orphans) and scrubs orphan
  `messages_fts` rows. Current schema version: 13. ADR-0224.
- The contacts ingest path lives in `internal/contacts/` (Client
  + Sync + Store seam); `cache.Account` implements `contacts.Store`
  (`Books`/`UpsertBook`/`ApplyChangeset`) and adds
  `SuggestAddresses(ctx, prefix)` (recency-decayed score over
  `message_recipients` joined to the carded pool, LIMIT 7),
  `LookupContact(ctx, address) (Contact, uid, ok)` (uid-keyed
  child rows; UID returned so callers can thread it onto
  `OpenFormMsg`), `LoadStoredVCard(ctx, uid) StoredVCard` for the
  patch path, and `DefaultBookHref(ctx)` for the single-book v1
  save destination. The CardDAV writer is `contacts.Writer`
  (`PutAddressObject`, `DeleteAddressObject`, `Multiget`); the
  App constructs `*contacts.Client` once and stores it on
  `Account.ContactsWriter`, shared with the existing
  `SyncContacts` path. Per-message `writeRecipientsTx` runs
  inside the upsert transaction in `reads.go` so
  `message_recipients` stays current.
- Contact write-back rides the existing outbox: `KindContactPut`
  + `ContactPutArgs{BookHref, Href, IfMatch}` carries the assembled
  vCard bytes in `outbox.payload`; `KindContactDelete` +
  `ContactDeleteArgs{Href, IfMatch}` carries no payload.
  `(*Account).QueueContactPut(ctx, uid, c, args, vcardBytes)`
  upserts the local row + emails (re-derived PREF) and inserts the
  outbox row in one tx; `QueueContactDelete(ctx, uid)` reads
  href/etag from the existing row, deletes (FK cascades
  `contact_emails`/`contact_phones`), inserts the outbox row.
  Drainer dispatch routes via `Account.ContactsWriter`; sentinel
  matrix uses `errors.Is(contacts.ErrAuth | ErrNotFound |
  ErrPreconditionFailed)`. `ErrPreconditionFailed` (412 = stale
  ETag) maps to `OpConflict precondition-failed`. `ErrNotFound`
  on Delete is idempotent success. `revertOptimisticTx` no-ops on
  both kinds (refetch is the conceptual revert; `DiscardOp` works
  on conflicted rows). `recoverExecuting` resets contact ops to
  `OpPending` (idempotent under If-Match). vCard edits are
  lossless: `contacts.PatchVCard(stored, c, now)` mutates only
  the keys poplar models (FN/N/ORG/TITLE/NOTE/EMAIL/TEL/REV/KIND/UID);
  every other field round-trips verbatim. `contacts.BuildVCard(c,
  uid, now)` covers the new-contact case. Index 0 of EMAIL/TEL
  gets `PREF=1`; retained rows keep their existing TYPE param;
  added rows get poplar's canonical labels.
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
  (default 0 from `[cache] max-size` in `config.toml`; 0 disables
  the cap), it evicts the oldest messages by `messages.sent_at`
  inline. `Backend.FetchBody` returns `([]byte, error)` — no
  `io.Reader`. ADRs 0122 (partially superseded by 0187), 0187.
- `internal/cache/backfill.go` is the per-account body-cache
  filler. One `Backfiller` per `*cache.Account`, started by `Open`
  and stopped by canceling the context threaded into `Run`. Work
  queue is implicit: `SELECT m.protocol_id FROM messages m LEFT
  JOIN bodies b ON b.message = m.id WHERE b.bytes IS NULL ORDER
  BY m.sent_at DESC LIMIT 1`. Each tick (500ms) checks gates,
  drains a batch up to 2 MB, sleeps. Gates: `connOnline` (paused
  on `Reconnecting` and `Offline`); `idle` (5s threshold on
  `lastActivity` driven by `tea.KeyMsg`); `atCap` (90% of
  `acct.maxSize`, skipped when `maxSize <= 0`). Server back-
  pressure (IMAP `[THROTTLED]`, JMAP rate-limit, HTTP 429)
  detected by substring on `err.Error()`; routes through
  local `expBackoff(throttleAttempts, 1s, 60s)`,
  mirroring the outbox drainer's curve. `(*Account).
  BackfillProgress()` returns `(done, total)` for the status-bar
  segment. `(*Account).NotifyActivity()` /
  `NotifyConnState(online bool)` are App→Backfiller signal
  shims. ADR-0187.
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
  forward write entry for Move/Flag/Destroy. `OpArgs` is a sealed
  sum (`MoveArgs`, `FlagArgs`, `DestroyArgs`,
  `SendArgs{Envelope}`, `AppendArgs{Flag}`).
  `(*Account).QueueSend(ctx, sentFolder, env, mime, scheduledFor,
  draftID)` and `(*Account).QueueAppend(ctx, folder, flag, mime,
  scheduledFor, draftID)` are the payload-bearing entry points for
  outbound mail; both insert a folder-scoped row with the assembled
  MIME bytes in `outbox.payload`, optional `scheduled_for` /
  `draft_id`, and skip optimistic UI (no message-row state to
  mirror). `(*Account).QueueOutbound(...)` returns op IDs in
  dispatch order: `[send]` on JMAP, `[send, append]` on IMAP. On
  IMAP, the drainer's pickup query (`nextOutboxRow`) holds a
  draft-linked `KindAppend` row ineligible until its sibling
  `KindSend` (same `draft_id`) reaches `OpDone`; the gate also
  closes when the sibling is absent so a stranded Append never
  fires (ADR-0208). Inside one transaction: resolve folder → row
  id, insert outbox row with `status='pending'` and
  `next_eligible_at=NULL`, apply optimistic `ui_flags`/`ui_hide`
  to the message row (Move/Flag/Destroy only), commit, signal
  drainer. `insertFolderOp` rejects with `cache.ErrOutboxRowTooLarge`
  before opening the tx when `Config.MaxOutboxBytes > 0` and the
  payload exceeds it (default 0 / unlimited, mirrors `MaxSize`;
  TOML `[cache] max-outbox-bytes`). ADR-0226.
- `(*Account).CancelOps(ctx, opIDs)` deletes named outbox rows
  iff every one is `OpPending`; atomic across the slice. Returns
  `ErrNotPending` if any row has advanced. Used by the App's `u`
  undo-send binding inside the `[ui] undo-send-window` (default
  10s, range `[0, 5m]`; zero disables) and by the Outbox view's
  `c`. Linked drafts rows are not touched on cancel — caller
  relies on the in-memory Draft for compose-restore.
- `(*Account).RescheduleOp(opID, newScheduledFor)` updates
  `scheduled_for` iff `OpPending` and `scheduled_for > now`, else
  `ErrNotPending`. `(*Account).OutboxScheduled()` returns
  `[]OutboxRow` (pending or failed) joined left to `folders` and
  `drafts` via `draft_id`, ordered `scheduled_for ASC, id ASC`
  with NULL last; Subject derived from the first 4 KB of payload
  via `net/textproto`. The drainer's `dispatch(args, row)` routes
  Send/Append to `Backend.Send`/`Append` via `row.FolderName` /
  `row.Payload`; `revertOptimisticTx` no-ops on both kinds.
- After the drainer marks a row `conflict`,
  `(*Account).RetryOp(ctx, opID)` and
  `(*Account).DiscardOp(ctx, opID)` are the user-initiated
  resolution primitives. Retry resets `attempts = 0` and signals
  the drainer. Discard reverts the optimistic flip via
  `revertOptimisticTx` (mirror of `applyOptimisticTx`; no-op for
  Send/Append) and deletes the outbox row in one transaction.
  Discarding a draft-linked `KindSend` also cascades the delete
  to any sibling `KindAppend` rows for the same `draft_id` (the
  gate above would otherwise strand them; ADR-0208). Both reject
  non-conflict rows with `ErrNotConflict`.
- Cache exposes three read queries for the outbox-visibility
  surfaces: `OutboxSummary` (grouped by kind/folder/status, where
  folder is the destination for Move ops via `json_extract` and
  empty for other kinds), `OutboxConflicts` (decoded error JSON,
  ordered by enqueued_at ASC), `OutboxDepth` (counts per status).
- Outbox status is the typed `cache.OpStatus` enum
  (`OpPending`/`OpExecuting`/`OpDone`/`OpFailed`/`OpConflict`)
  stored as the underlying string. Op kind is `cache.OpKind`
  (`KindMove`/`KindFlag`/`KindDestroy`/`KindSend`/`KindAppend`/
  `KindPushDraft`/`KindContactPut`/`KindContactDelete`).
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
- `expBackoff(attempts, initial, max)` is a small per-package
  helper duplicated in `internal/cache/`, `internal/mailjmap/`,
  and `internal/mailimap/`. Returns `initial` on attempt ≤ 1;
  doubles each subsequent attempt; caps at `max`. ADR-0209.
