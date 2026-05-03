# Plan — Pass 8.4a — Cache I implementation

**Pass:** 8.4a
**Type:** implementation pass — first of three (Cache I → II → III)
**Inputs:**
- `docs/superpowers/specs/2026-05-02-cache-0-design.md` (revised, status: reviewed 2026-05-02)
- ADR-0110 (narrowed by 0114, 0115), ADR-0111 (parts superseded by 0117),
  ADR-0112 (superseded in part by 0113, 0116; narrowed by 0114),
  ADR-0113, ADR-0114, ADR-0115, ADR-0116, ADR-0117
- Pass 8.4-review findings (`docs/superpowers/reviews/2026-05-02-cache-0-review.md`)

## Goal

Land the on-disk cache foundation: schema, per-account SQLite handles,
the `mail.ChangeTracker` interface and both backend implementations,
the unified write path through `cache.QueueOp`, and the strangler-fig
migration from direct-`mail.Backend` triage Cmds to cache-backed reads.

Cache I scope is **online behavior only**. The outbox table exists
and is written to (writes flow through `QueueOp`), but the offline-
detection state machine, `Q`/`!` overlays, and offline status badge
land in Cache III (Pass 8.4c). Body cache + eviction + `poplar cache`
CLI land in Cache II (Pass 8.4b).

## Critical context

- Pre-beta; refactor freedom (ADR-0105). The on-disk schema is
  v1.0-frozen — get it right.
- Strangler-fig migration is mandatory (review C1). Don't try to
  switch every call site at once. The order below is binding.
- The cache package never imports `bubbletea`. UI signaling is via
  `(*cache.Account).Events() <-chan CacheEvent` + `pumpCacheCmd`
  (review C3, ADR-0117).
- `App.pendingAction.onUndo` synchronous-mutation half is removed in
  step 12 below, not earlier — undo continues to work via
  `MessageList.Apply*` until cache-backed reads land.

## Approach — ordered task list

### Phase 1: Cache scaffolding (no UI changes yet)

1. **`internal/cache/` package skeleton.**
   - `cache.Cache` (account map), `cache.Account` (per-account
     handle holding `mail.Backend`, `mail.ChangeTracker`,
     `*sql.DB`, drainer + syncer goroutine handles, `Events()`
     channel).
   - `modernc.org/sqlite` driver wired via `database/sql`. Pragmas
     applied per connection at open time (WAL, foreign_keys,
     synchronous=NORMAL, busy_timeout=5000).
   - `cache.Open(accountName string, backend mail.Backend, ct
     mail.ChangeTracker, dir string) (*Account, error)`.
   - Tilde-expand `dir`; default via `os.UserCacheDir()`.

2. **Schema migration framework.**
   - `schema_version` table; ordered `migration` funcs registered
     in an init() or a slice in `internal/cache/schema.go`.
   - On `Open`, walk versions and apply each in a single
     transaction. Fresh DB starts at version 0 and runs all
     migrations to current.

3. **Schema version 1 = full schema from spec §A.3.**
   - Tables: `schema_version`, `folders`, `messages`,
     `message_mailboxes`, `bodies`, `outbox`.
   - Indexes: `message_mailboxes_folder`, `messages_sent`,
     `messages_thread`, `bodies_lru`, `outbox_pending`,
     `outbox_message`.
   - All FKs `ON DELETE CASCADE`. `messages.protocol_id UNIQUE`.

4. **`mail.ChangeTracker` interface in `internal/mail/`.**
   - Interface + `SyncToken []byte` + `ChangeSet{Added, Modified,
     Removed []UID}` + `ErrCannotCalculateChanges` sentinel.
   - Godoc spells the `hasMoreChanges` loop contract (Implementations
     MUST loop internally until the backend reports no more pages).

### Phase 2: ChangeTracker implementations

5. **JMAP `ChangeTracker` in `internal/mailjmap/`.**
   - `(*Backend).Changes(ctx, folder, since)` issues `Email/changes`
     in a loop until `hasMoreChanges = false`.
   - Maps `cannotCalculateChanges` → `mail.ErrCannotCalculateChanges`.
   - Encodes/decodes `SyncToken` as the JMAP state string (UTF-8
     bytes).

6. **IMAP `ChangeTracker` in `internal/mailimap/`.**
   - Asserts CONDSTORE at `Connect` (alongside existing UIDPLUS
     assertion). NOMODSEQ servers fail Connect with a clear error.
   - `Changes()` uses `SELECT ... (CONDSTORE)` plus
     `UID FETCH 1:* CHANGEDSINCE <modseq> (UID FLAGS)` and
     `UID SEARCH UID > <maxuid>` to compute deltas.
   - On UIDVALIDITY change: returns `mail.ErrCannotCalculateChanges`
     after issuing the connection-fence signal (drainer-pause hook
     wired in step 8).
   - SyncToken = `(uidvalidity, modseq, maxuid)` packed binary.

### Phase 3: Backend interface shrink

7. **`mail.Backend` collapse: `MarkRead`/`MarkUnread`/`MarkAnswered`/
   `Delete` → `Flag(uids, flag, set)`.**
   - Update both backend impls.
   - Delete the four old methods; rewrite call sites in
     `internal/ui/triage*.go` to call `Flag` (still going through
     `mail.Backend` directly at this point — strangler-fig step 1).

### Phase 4: Cache reads (still no UI cutover)

8. **Cache reads.**
   - `(*Account).ListFolders()` reads `folders` joined with
     classification (cache-side classifier wraps `mail.Classify`).
   - `(*Account).QueryFolder(name, offset, limit)` joins
     `messages` ⨝ `message_mailboxes` filtered by folder, ordered
     `sent_at DESC`, gates on `ui_hide = 0`. Paginated.
   - `(*Account).FetchHeaders(uids)` reads from cache; on miss, the
     syncer's next tick populates from the backend.
   - `(*Account).FetchBody(uid)` — Cache II scope; stub for now
     (always backend-direct until Pass 8.4b lands the bodies path).

9. **Syncer goroutine.**
   - Per-folder. On open: full `Changes()` from stored
     `folders.sync_token`; baseline if NULL.
   - Listens on JMAP push / IMAP IDLE for nudges; drives
     `Changes()` on each tick, with the `sync-interval` cadence as
     the floor.
   - Writes to `messages` / `message_mailboxes` / `folders.sync_token`
     in transactions.
   - **Coordination invariant** (ADR-0113): EXISTS check against
     `outbox_message` partial index before writing `ui_flags`. If
     a `pending`/`executing` row exists, write only `flags`.
   - On `ErrCannotCalculateChanges` or UIDVALIDITY change: runs the
     re-anchor path from spec §D.4 (atomic re-key + outbox
     promotion to `rekey-orphaned` / `anchor-lost`).

### Phase 5: Cache writes + outbox drainer

10. **`(*Account).QueueOp(ctx, folderName, msgID, args OpArgs)`.**
    - Sealed sum: `MoveArgs`, `FlagArgs`, `DestroyArgs` (Cache 0
      scope) plus reserved `SendArgs`, `AppendArgs`.
    - Inside transaction: resolve folder name → row id; INSERT
      outbox row with `status='pending'`; apply optimistic
      `ui_flags`/`ui_hide` to the message row; commit; signal
      drainer.

11. **Outbox drainer goroutine.**
    - Per-account, single goroutine. Pickup query from spec §D.3.
    - State-machine transitions per §D.3 / §D.4 (success → done;
      `notFound` → done; backend conflict → conflict; 4xx auth →
      conflict (auth-failure); network → failed + backoff;
      `attempts >= max-attempts > 0` → conflict
      (max-attempts-exceeded); crashed-mid-execute send → conflict
      on restart).
    - **Drain-first ordering** (ADR-0113): scheduler drains queue
      to completion before the syncer's next `Changes()` tick on
      reconnect.
    - Publishes `CacheEvent` on `Events()` after each terminal
      transition.

12. **`pumpCacheCmd` in `internal/ui/cmds.go`.**
    - Mirrors `pumpUpdatesCmd`. Ranges
      `(*cache.Account).Events()`; re-emits values as `tea.Msg`.

### Phase 6: UI cutover (strangler-fig steps b → c)

13. **Step b — switch reads to cache-backed.**
    - `App.NewApp` constructs `*cache.Cache` and threads
      `*cache.Account` into each `AccountTab`.
    - `AccountTab.SetMessages` / `AppendMessages` source from
      `cache.Account.QueryFolder` instead of holding a backend
      `mail.Backend` pointer for reads.
    - `MessageList.Apply*` and the old App-layer optimistic state
      stay alive at this step. Reads now reflect cache state; the
      Apply* mutations and the cache write path *both* update the
      same row's `ui_flags`/`ui_hide`. This window is safe because
      both paths converge on the same SQL state.

14. **Step c — switch triage writes to `QueueOp`.**
    - `dispatchTriage`, `dispatchMoveFromPicker`, retention sweep,
      `emptyFolderCmd` all call `cache.QueueOp` instead of the
      backend method.
    - Keep the toast/undo timer in `App.pendingAction`; replace
      `onUndo func()` callsites — undo now fires only the saved
      `inverse tea.Cmd` (a compensating `QueueOp`).

15. **Step d — delete legacy paths.**
    - Remove `MessageList.Apply{Delete,Insert,Flag,Seen}` and any
      callers; reads come from cache.
    - Remove `triageStartedMsg.onUndo` field.
    - Remove the App-layer optimistic-state plumbing that
      ADR-0089 introduced (the queue + cache replace it).

### Phase 7: Tests

16. **Unit tests** per package. `internal/cache/` table-driven
    tests for: schema migration up from version 0; QueueOp
    transactional atomicity; drainer state-machine transitions
    (use a fake `mail.Backend` and `mail.ChangeTracker`); re-anchor
    path with simulated UIDVALIDITY change and `anchor-lost`
    promotion; coordination invariant (syncer must not stomp
    `ui_flags` on rows with pending outbox).

17. **Integration test.** Cache + fake backend + fake change
    tracker driving a full triage cycle (queue → drain → success →
    Events emitted). Crash-recovery test: kill the drainer
    mid-execute, re-open the DB, assert idempotent ops reset to
    `pending` and `send`-class ops reset to `conflict`.

## Outputs

- New `internal/cache/` package (schema, account, drainer, syncer,
  events, op-sum types).
- `mail.ChangeTracker` interface + both backend impls.
- `mail.Backend` shrunk to `Flag` (four old methods removed).
- UI rewired to read/write through `cache.Account`.
- Old App-layer optimistic-state plumbing deleted.
- Comprehensive tests; `make check` green.
- ADR(s) for any binding fact that emerges during implementation
  not already covered by ADR-0110–0117.
- Invariants.md gets a new prose section on the cache (now that
  binding facts are realized in code, not just spec).

## Hand-off

Cache II (Pass 8.4b) takes over: body cache + LRU/age/per-folder
eviction + `poplar cache` CLI subcommands.

## Standard pass-end ritual

`/simplify`, idiomatic-bubbletea check (UI changes occurred — cutover
in steps 13–15), new ADRs as needed, invariants.md update (add cache
binding facts), STATUS.md update, archive this plan, `make check`,
commit + push, `make install`, tmux verification of triage smoke test
at 80×24 and 120×40 (ensure Apply* removal hasn't broken visual
behavior).
