# Cache 0 — Local Mail Cache Design

**Spec date:** 2026-05-02 (revised 2026-05-02)
**Pass:** 8.4 (design) → 8.4-review → 8.4-revise → 8.4a/b/c (implementation)
**Status:** reviewed (2026-05-02) — implementation-ready. See revision history below.

## Review notes

Pass 8.4-review (`docs/superpowers/reviews/2026-05-02-cache-0-review.md`)
flagged 12 must-fix and 10 should-fix findings. Pass 8.4-revise applied
all must-fixes and most should-fixes inline. Decisions that reversed
or substantively narrowed ADRs 0110 / 0111 / 0112 produced new ADRs:

- **ADR-0113** — drain-first sync ordering (RFC 4549 §6); syncer/
  drainer coordination invariant. Supersedes ADR-0112 sync-first.
- **ADR-0114** — UIDVALIDITY re-key contract; `anchor-lost` and
  `rekey-orphaned` outbox promotion paths. Narrows ADR-0110/0112.
- **ADR-0115** — JMAP one-row-per-Email-per-account model; junction
  table `message_mailboxes`. Narrows ADR-0110.
- **ADR-0116** — outbox terminal classification: `max-attempts = 10`
  default, auth-failure → conflict, crashed-mid-execute send →
  conflict. Supersedes ADR-0112 failure handling.
- **ADR-0117** — typed `Op` sum, name-based folder ops, drainer→UI
  `Events()` channel, undo via compensating `tea.Cmd` only.
  Supersedes parts of ADR-0111.

Should-fix findings deferred to backlog (none — all applied or
covered by the new ADRs).

## Purpose

Define the architecture, schema, and behavioral invariants of poplar's
local mail cache. Implementation happens in three subsequent passes:

- **Cache I (Pass 8.4a)** — schema + envelope/header cache + per-account
  SQLite + `ChangeTracker` interface and JMAP/IMAP implementations.
  Online-only behavior. Writes route through the cache.
- **Cache II (Pass 8.4b)** — body cache + eviction + `poplar cache` CLI
  subcommands.
- **Cache III (Pass 8.4c)** — outbox + offline detection + replay drain
  + `Q` Outbox overlay + `!` Conflicts overlay + status-bar offline
  badge.

The cache becomes a v1.0-frozen on-disk format per ADR-0105. Schema
choices made here become migration debt if wrong; the review/revise
passes that follow this design pass exist for that reason.

## Scope of this document

This spec defines:

1. **Storage architecture** — SQLite per account, schema, indexes.
2. **Cache shape** — `cache.Cache` as the UI-facing layer; `mail.Backend`
   and `mail.ChangeTracker` as protocol-level interfaces consumed by
   the cache.
3. **Unified write path** — every triage write flows through
   `cache.QueueOp`; one code path online and offline.
4. **Outbox model** — schema, status state machine, conflict policy,
   replay strategy.
5. **Eviction primitives** — body cache LRU + size + age.
6. **CLI surface** — `poplar cache` subcommands.
7. **Config surface** — top-level `[cache]` block.
8. **UI surfaces** — Outbox / Conflicts overlays, offline badge.
9. **Failure modes** — explicitly designed-against scenarios.

This spec does **not** define implementation order beyond the pass
breakdown above; that is the job of the per-pass plan docs (8.4a/b/c).

## A. Storage architecture

### A.1 Per-account SQLite databases

Each account gets its own SQLite database at:

```
$XDG_CACHE_HOME/poplar/<account-slug>/mail.db
```

Linux/macOS use `$XDG_CACHE_HOME` (or `~/.cache`); Windows uses
`%LOCALAPPDATA%\poplar`. macOS deliberately overrides the Application
Support default, consistent with the config-file precedent
(ADR-0102). The default is resolved via `os.UserCacheDir()`; the
`[cache] dir` config value is tilde-expanded before use.

`<account-slug>` is the account name from `config.toml` lower-cased,
with non-`[a-z0-9-]` characters replaced by `-`. Slug collisions
between accounts are detected at startup and surface a clear error
with both account names.

**Rationale.** Per-account isolation:

- Account add = create directory + open fresh DB (auto-migrates from
  schema 0).
- Account remove = `rm -rf` the directory.
- Corruption blast radius is one account.
- Schema migrations run per-account on connect, in parallel; one
  out-of-date account doesn't block startup of healthy accounts.
- Backup, inspection, and `grep` operate on a single account's DB.

The connection-bookkeeping cost (`map[accountName]*sql.DB`) is
trivial.

### A.2 Driver — pure-Go SQLite

`modernc.org/sqlite`. No cgo. Single dependency. Serial writer + many
readers via WAL mode is exactly the access pattern (one drain
goroutine writing, UI reads from the main loop's cmd closures).

Pragmas at open time (per connection):

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;
```

### A.3 Schema (full)

```sql
CREATE TABLE schema_version (
  version INTEGER NOT NULL
);

CREATE TABLE folders (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  name          TEXT    NOT NULL UNIQUE,        -- canonical (Inbox, Sent, ...)
  protocol_name TEXT    NOT NULL,               -- raw ("INBOX", "[Gmail]/Sent Mail", ...)
  role          TEXT,                           -- 'inbox' | 'sent' | 'trash' | 'drafts' | 'archive' | 'junk' | 'custom'
  uidvalidity   INTEGER,                        -- IMAP only; NULL for JMAP
  sync_token    BLOB,                           -- backend-opaque
  last_synced   INTEGER                         -- unix nanos
);

CREATE TABLE messages (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  protocol_id  TEXT    NOT NULL UNIQUE,         -- IMAP: UID stringified; JMAP: Email id
  thread_id    TEXT,
  in_reply_to  TEXT,
  subject      TEXT,
  from_addr    TEXT,
  to_addr      TEXT,
  cc_addr      TEXT,
  bcc_addr     TEXT,
  date_str     TEXT,                            -- legacy display fallback
  sent_at      INTEGER,                         -- unix nanos; authoritative for sort
  flags        INTEGER NOT NULL DEFAULT 0,      -- server-confirmed flags (mail.Flag bits)
  size         INTEGER,
  ui_flags     INTEGER NOT NULL DEFAULT 0,      -- optimistic UI-side flags
  ui_hide      INTEGER NOT NULL DEFAULT 0       -- 0/1; mid-move source hides from list
);

-- Folder membership. IMAP: one row per message (lives in exactly one
-- folder). JMAP: N rows per Email (mailboxIds is a set). Move = swap
-- the row(s); copy (IMAP) = additional row.
CREATE TABLE message_mailboxes (
  message INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  folder  INTEGER NOT NULL REFERENCES folders(id)  ON DELETE CASCADE,
  PRIMARY KEY (message, folder)
);
CREATE INDEX message_mailboxes_folder ON message_mailboxes(folder);

CREATE INDEX messages_sent   ON messages(sent_at DESC);
CREATE INDEX messages_thread ON messages(thread_id);

CREATE TABLE bodies (
  message       INTEGER PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
  bytes         BLOB    NOT NULL,
  fetched_at    INTEGER NOT NULL,
  last_accessed INTEGER NOT NULL
);
CREATE INDEX bodies_lru ON bodies(last_accessed);

CREATE TABLE outbox (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  folder       INTEGER NOT NULL REFERENCES folders(id)  ON DELETE CASCADE,
  message      INTEGER          REFERENCES messages(id) ON DELETE CASCADE,
  kind         TEXT    NOT NULL,                -- 'move' | 'flag' | 'destroy' | 'send' | 'append' | ...
  args         TEXT    NOT NULL,                -- JSON; type-specific (encoded from typed Op sum, see C.2)
  enqueued_at  INTEGER NOT NULL,
  status       TEXT    NOT NULL DEFAULT 'pending',
  attempts     INTEGER NOT NULL DEFAULT 0,
  last_attempt INTEGER,
  error        TEXT                             -- JSON: {kind, message, detail}
);
CREATE INDEX outbox_pending ON outbox(id) WHERE status IN ('pending', 'failed');
CREATE INDEX outbox_message ON outbox(message) WHERE status IN ('pending', 'executing');
```

Three non-obvious schema choices:

- **`messages` carries `ui_flags` and `ui_hide` separately from
  `flags`.** `flags` reflects what the server has confirmed;
  `ui_flags` reflects what the user has requested. The UI reads
  `ui_flags`; the backend confirmation path writes `flags` and may
  converge them. `ui_hide` lets a mid-move source message hide from
  the source folder before the server has confirmed the move. (Source:
  FairEmail's `EntityMessage` model.)
- **`messages.protocol_id` is `TEXT`, not `INTEGER`, and is unique
  per account (the DB is per-account, so this is account-scoped).**
  JMAP `Email` ids are strings; IMAP UIDs are stringified. The cache
  layer is protocol-agnostic.
- **Folder membership lives in `message_mailboxes`, not on
  `messages`.** This accommodates JMAP semantics natively (one
  `Email` may live in N mailboxes; `Email/set update mailboxIds` is
  a set operation, not a row move). For IMAP a message lives in
  exactly one folder, so the junction holds a single row per
  message; an IMAP `MOVE` rewrites the junction row in the same
  transaction as the protocol_id update. Replaces the rejected
  `messages.folder` + `UNIQUE (folder, protocol_id)` shape from the
  pre-revise spec, which couldn't represent JMAP cleanly (see
  ADR-0115).

**CONDSTORE assertion (IMAP).** The IMAP backend asserts CONDSTORE
support at `Connect` (alongside the existing UIDPLUS assertion). On
NOMODSEQ servers, `Connect` fails with a clear error directing the
user to a different account configuration. Same shape as UIDPLUS;
keeps `ChangeTracker` semantics uniform across servers and avoids a
NOMODSEQ-only fallback path that would be exercised rarely and
silently rot.

### A.4 Schema versioning and migrations

`schema_version` holds a single integer. Connect runs the upgrade
chain from `version` to the binary's current version, transactional
per step. Pre-beta passes can change the schema freely (ADR-0105);
beta-soak freezes it. Migrations from a frozen version are
write-once and lossless.

## B. Cache shape and interfaces

### B.1 Three layers

```
internal/ui/  ──reads/writes──▶  internal/cache/  ──reads/writes──▶  internal/mail{jmap,imap}/
                                       │
                                       └──holds──▶  *sql.DB (per account)
```

The UI talks to `cache.Cache`. The cache talks to backends. The cache
owns the SQLite handles. Backends know nothing about the cache.

### B.2 `mail.Backend` (revised)

`mail.Backend` shrinks to protocol I/O only. Method set:

```go
type Backend interface {
    AccountName() string
    AccountEmail() string
    Connect(ctx context.Context) error
    Disconnect() error

    ListFolders() ([]Folder, error)
    OpenFolder(name string) error
    QueryFolder(name string, offset, limit int) (uids []UID, total int, err error)
    FetchHeaders(uids []UID) ([]MessageInfo, error)
    FetchBody(uid UID) (io.Reader, error)
    Search(criteria SearchCriteria) ([]UID, error)

    Move(uids []UID, dest string) error
    Copy(uids []UID, dest string) error
    Flag(uids []UID, flag Flag, set bool) error
    Destroy(uids []UID) error

    Send(from string, rcpts []string, body io.Reader) error

    Updates() <-chan Update
}
```

(Note: `MarkRead` / `MarkUnread` / `MarkAnswered` / `Delete` from the
current interface fold into `Flag`. That collapse is a Cache I task,
not Cache 0.)

### B.3 `mail.ChangeTracker` (new sibling interface)

```go
type ChangeTracker interface {
    // Changes returns the complete set of message ids that were
    // added, modified, or removed in folder since the given token.
    //
    // Implementations MUST loop internally until the backend reports
    // no more pages (JMAP: hasMoreChanges = false; IMAP: VANISHED
    // and FETCH responses fully drained). Callers receive a
    // single complete delta or an error; partial deltas are an
    // implementation bug.
    //
    // Errors:
    //   - ErrCannotCalculateChanges (sentinel) — backend reports
    //     the cached SyncToken is too stale to compute a delta
    //     (JMAP cannotCalculateChanges, IMAP UIDVALIDITY change).
    //     The cache responds with the forced-refetch path defined
    //     in §D.4 (pre-wipe remap by protocol_id; unremapped
    //     pending outbox rows promoted to conflict with
    //     error.kind = 'anchor-lost').
    //   - context errors — propagated.
    //   - other errors — transient; the syncer applies backoff.
    //
    // SyncToken is opaque []byte; backends encode/decode their own
    // representation (JMAP: state string; IMAP: (uidvalidity, modseq, maxuid)).
    Changes(ctx context.Context, folder string, since SyncToken) (ChangeSet, SyncToken, error)
}

// ErrCannotCalculateChanges is returned by Changes when the backend
// cannot compute a delta from the supplied token. The cache responds
// by re-anchoring the folder (see §D.4).
var ErrCannotCalculateChanges = errors.New("mail: cannot calculate changes")

type SyncToken []byte

type ChangeSet struct {
    Added    []UID  // new since `since`
    Modified []UID  // flag/state changes since `since`
    Removed  []UID  // destroyed since `since`
}
```

Both v1 backends (`mailjmap.Backend`, `mailimap.Backend`) implement
both `Backend` and `ChangeTracker`. The cache holds both pointers per
account.

The header / metadata for `Added` and `Modified` UIDs comes from a
follow-up `Backend.FetchHeaders` call, decided by the cache (it knows
what it already has on disk). `Removed` UIDs flow through cache
delete + CASCADE.

**Why a sibling interface, not a fold-in?** `Backend` is "the protocol
speaks." `ChangeTracker` is "change detection." Different concerns;
testable independently. Reading the `Backend` interface tells you
nothing about sync state, which is correct.

### B.4 `cache.Cache` and `cache.Account`

```go
package cache

type Cache struct {
    accounts map[string]*Account
}

type Account struct {
    Backend       mail.Backend
    ChangeTracker mail.ChangeTracker
    db            *sql.DB
    drain         *drainer        // outbox replay goroutine
    sync          *syncer         // ChangeTracker poll loop
    online        atomic.Bool
}

// UI-facing reads (all served from SQLite):
func (a *Account) ListFolders() ([]Folder, error)
func (a *Account) QueryFolder(name string, offset, limit int) ([]MessageInfo, int, error)
func (a *Account) FetchHeaders(uids []UID) ([]MessageInfo, error)
func (a *Account) FetchBody(uid UID) (io.Reader, error)

// UI-facing writes (all enqueue into outbox).
// Folder is the canonical folder name (Inbox, Sent, …); the cache
// resolves to the row id internally inside the same transaction.
func (a *Account) QueueOp(ctx context.Context, folder string, msgID int64, args OpArgs) error

// Status + control:
func (a *Account) IsOnline() bool
func (a *Account) Outbox() ([]OutboxRow, error)
func (a *Account) Conflicts() ([]OutboxRow, error)
func (a *Account) ResolveConflict(opID int64, action ResolveAction) error

// Drainer→UI signal channel. App's pumpCacheCmd ranges this and
// re-emits CacheEvent values as tea.Msg into the program loop. The
// cache package never holds *tea.Program (preserves the layer
// boundary in §B.1).
func (a *Account) Events() <-chan CacheEvent
```

`App` constructs `*cache.Cache` and threads `*cache.Account` into
each `AccountTab`. `AccountTab` no longer holds `mail.Backend`
directly.

## C. Unified write path

### C.1 The single action API

Every triage write — online or offline — goes through:

```go
func (a *Account) QueueOp(ctx context.Context, op Op) error {
    return a.tx(ctx, func(tx *sql.Tx) error {
        // 1. INSERT into outbox with status='pending'.
        // 2. Apply optimistic state to messages (ui_flags, ui_hide).
        // 3. Commit.
        // 4. Signal drainer.
    })
}
```

There is no separate "online write" function. When online, the drain
goroutine picks up the row within milliseconds and runs it. When
offline, the row sits until reconnect. Same code, both paths.

**Rationale.** Mailspring's `TaskProcessor` model. The unified path:

- Eliminates duplicate code (one path tested = both paths tested).
- Makes optimistic UI a property of every action, not a bolt-on.
- Naturally handles the "I started online, went offline mid-action,
  came back online" case.
- Means online performance is measured against the same code that
  runs offline — no surprises.

### C.2 Op kinds

`OpArgs` is a sealed sum at the Go layer; the on-disk `args TEXT`
column carries its JSON encoding. The kind string in the `kind`
column is derived from the concrete `OpArgs` type at insert time.

```go
type OpArgs interface{ opArgs() }

type MoveArgs    struct { Dest string }            // dest folder name
type FlagArgs    struct { Flag mail.Flag; Set bool }
type DestroyArgs struct{}
type SendArgs    struct { /* Pass 9 */ }
type AppendArgs  struct { /* Pass 9 */ }

func (MoveArgs)    opArgs() {}
func (FlagArgs)    opArgs() {}
func (DestroyArgs) opArgs() {}
func (SendArgs)    opArgs() {}
func (AppendArgs)  opArgs() {}
```

Kind catalog (Cache 0 scope marked **bold**):

- **`move`** — `MoveArgs{Dest}`. Optimistic: set `ui_hide = 1` on
  source row.
- **`flag`** — `FlagArgs{Flag, Set}`. Optimistic: update `ui_flags`.
- **`destroy`** — `DestroyArgs{}`. Optimistic: `ui_hide = 1`; row
  deleted on success.
- `send` — Pass 9 (Compose). Schema accommodates; not implemented in
  Cache 0–III.
- `append` — Pass 9. APPEND a local message to a server folder
  (used by Send to copy outgoing into Sent).

Adding a kind is a new `OpArgs` impl + drainer dispatcher case + a
new kind string. No schema migration. The on-disk `args` JSON
schema is part of the v1.0-frozen format; new fields on existing
kinds must be additive (and unknown fields tolerated on read).

### C.3 Optimistic UI semantics

- `ui_flags` mirrors `flags` until a queued `flag` op modifies it.
  When the backend confirms, the drainer writes the new value to
  `flags` and (if no conflict) leaves `ui_flags` matching.
- `ui_hide = 1` removes a row from msglist queries (`WHERE
  ui_hide = 0`). On successful destroy, the row is deleted (CASCADE
  removes outbox rows). On successful move, the row is deleted from
  the source folder; the destination folder's normal sync cycle
  re-adds it (or the backend returns the new id, which the drainer
  uses to insert directly).
- The UI never observes intermediate state. Reads always go through
  `cache.Account` methods; those methods return only rows where
  `ui_hide = 0`.

**Syncer/drainer coordination invariant.** The syncer (`Changes()`
poll loop, also fed by JMAP push and IMAP IDLE) MUST NOT update
`ui_flags` for any message whose `messages.id` appears in an outbox
row with status `pending` or `executing`. The syncer updates only
`flags`. The drainer is solely responsible for converging
`ui_flags → flags` after backend confirmation. Without this
invariant, a server-side change pushed during an in-flight queued
flag op would revert the optimistic display before the drainer
runs. (The `outbox_message` partial index supports the EXISTS
check.)

## D. Outbox state machine and replay

### D.1 State machine

```
                             pending
                                │
                                │ drainer picks up
                                ▼
                            executing
                                │
                ┌───────────────┼────────────────┐
                │               │                │
              success       conflict          failure
                │               │                │
                ▼               ▼                ▼
              done          conflict          failed
                                                 │
                                                 │ network error;
                                                 │ backoff timer
                                                 ▼
                                              pending
```

States:

- **`pending`** — newly enqueued, awaiting drain. Drainer's primary
  query: `SELECT ... WHERE status IN ('pending', 'failed') AND
  (last_attempt IS NULL OR last_attempt < now - backoff(attempts))`.
- **`executing`** — drainer is currently running this op against the
  backend. Persists across crashes.
- **`done`** — successful. GC'd by startup sweep after
  `outbox-retention` (default 7d).
- **`conflict`** — backend reported the op cannot be reconciled.
  Stays until user acts via the `!` Conflicts overlay or
  `poplar cache resolve`.
- **`failed`** — transient failure (network, rate limit). Backoff
  timer starts: 1s, 2s, 4s, ... capped at `backoff-max` (default
  60s). Status flips to `pending` after backoff window elapses on
  next drainer tick.

### D.2 Crash recovery

On startup, ops in `executing` state are reclassified:

- **Idempotent kinds (`move`, `flag`, `destroy`)** — reset to
  `pending`. Safe to replay; the conflict matrix handles "already
  applied" as success.
- **Non-idempotent kinds (`send`)** — reset to `conflict` with
  `error.kind = 'crashed-mid-execute'`. User must resolve via the
  `!` Conflicts overlay. Resetting to `failed` would cycle
  `executing → failed → pending → executing → …` forever on a
  reproducible crash; promotion to `conflict` requires explicit user
  acknowledgement. (This matters for Pass 9; mentioned here so
  Cache 0's schema and the conflict-overlay UX accommodate it.)

### D.3 Replay strategy

**Drain order.** Per-account, single goroutine, strict enqueue order
(`ORDER BY id`). One op at a time. No drain-time coalescing. Across
accounts: parallel.

**Drainer query.** The drainer's pickup query is:

```sql
SELECT * FROM outbox
WHERE status IN ('pending', 'failed')
  AND (last_attempt IS NULL OR last_attempt < :now - backoff(attempts))
ORDER BY id
LIMIT 1
```

A `failed` op whose backoff window has not elapsed is **skipped**;
later `pending` ops on the same account proceed. This non-blocking
behavior is an explicit invariant — the drainer never head-of-line
blocks the whole queue on a single failed op (K-9's MessagingController
takes the opposite stance, which we deliberately reject; rationale in
review B3).

**Per-op execution.**

1. Mark `status = 'executing'`, `last_attempt = now`, `attempts++`.
2. Read joined `messages.protocol_id` for the op's `message` row
   (gets the current UID/JMAP id, which may differ from when
   enqueued).
3. Dispatch on the typed `OpArgs`; call the appropriate `Backend`
   method.
4. On success → `status = 'done'`, optionally apply confirmation
   side-effects (e.g., move op → update `message_mailboxes`).
5. On `notFound` from backend (message destroyed remotely) →
   `status = 'done'` (idempotent success).
6. On other backend conflict (folder gone, etc.) → `status =
   'conflict'`, populate `error`.
7. On 4xx authentication error → `status = 'conflict'`,
   `error.kind = 'auth-failure'`. Does not enter the backoff loop.
   User resolves by re-authenticating (Pass 9.6 token refresh; until
   then, restarting `password-cmd` and re-launching). This prevents
   the OAuth-token-expires-mid-drain infinite-retry trap (review D3).
8. On network error → `status = 'failed'`, populate `error`. Backoff
   timer governs retry.
9. When `attempts >= max-attempts > 0`, transition `failed →
   conflict` with `error.kind = 'max-attempts-exceeded'`. Default
   `max-attempts = 10`. Setting `max-attempts = 0` (unlimited)
   remains opt-in for users who want indefinite retry on intermittent
   networks (review B6 / D3).

**Drain-first ordering.** On reconnect, the per-account scheduler
drains the outbox **before** running `ChangeTracker.Changes()`. RFC
4549 §6 ("Processing Offline Queues") explicitly mandates queued
client actions before pulling server-side changes; the spec follows
the RFC. The FK CASCADE protection still works under drain-first
because `notFound` from the backend is already a `done` outcome in
the conflict matrix — a remote-removed message produces idempotent
success during drain, then the subsequent `Changes()` cycle reaps
the cache row.

(IMAP UIDVALIDITY signals received during drain fence both
connections until re-key completes; see §D.4. Replaces the
sync-first ordering rejected in ADR-0113.)

**Drainer→UI signals.** The drainer publishes `CacheEvent` values on
`(*Account).Events()` after each terminal transition (`done`,
`conflict`). `App` runs a `pumpCacheCmd` (mirroring `pumpUpdatesCmd`
for `mail.Update`) that ranges this channel and re-emits the values
as `tea.Msg` into the program loop. The cache package never holds a
`*tea.Program` reference. UI redraws on `CacheEvent` reception
(message list refresh, conflict-count badge, status bar).

**Why no coalescing.** I considered batching same-kind/same-args ops
into single backend calls (Thunderbird's strategy). Dropping it for
Cache 0 because:

- Mailspring runs in production without coalescing.
- JMAP `Email/set` is naturally batched at the protocol level — the
  JMAP backend implementation can coalesce when constructing the
  HTTP request, transparently to the queue.
- For IMAP, typical triage volumes (dozens of STORE/MOVE on a
  long-lived connection) don't have meaningful latency cost.
- Pre-beta YAGNI (ADR-0105) — add coalescing if profiling shows it
  matters. Coalescing belongs at the backend implementation, not the
  queue.

### D.4 Conflict matrix

| Kind                  | Replay-time check                                                                                          | Outcome                                                                                                   |
|-----------------------|------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------|
| Any                   | Joined `messages` row CASCADE-deleted (remote-removed via `Changes()`)                                     | Outbox row already gone — no work. (Also handles "remote destroyed" universally.)                         |
| `move(msg, dest)`     | Read current `messages.folder` for msg; current folder is already `dest`                                   | Skip (idempotent success). `status = 'done'`.                                                             |
| `move(msg, dest)`     | Current folder differs from enqueue-time folder (another client moved it)                                  | Apply our move from current location. Audit-trail entry. `status = 'done'`.                               |
| `move(msg, dest)`     | Backend returns "destination folder doesn't exist"                                                         | `status = 'conflict'`. User resolves.                                                                     |
| `move(msg, dest)`     | Backend returns `notFound` for protocol_id                                                                 | Idempotent success (message gone before we got there). `status = 'done'`.                                 |
| `flag(msg, F, set)`   | Backend already has flag in target state                                                                   | Skip (idempotent success). `status = 'done'`.                                                             |
| `flag(msg, F, set)`   | Backend has opposite state (another client toggled)                                                        | Apply our value (last-writer-wins). `status = 'done'`.                                                    |
| `flag(msg, F, set)`   | Backend `notFound`                                                                                         | Idempotent success.                                                                                       |
| `destroy(msg)`        | Backend already destroyed (or `notFound`)                                                                  | Idempotent success. (JMAP spec: `notFound` in `Email/set destroy` is success. IMAP UID EXPUNGE: no-op.)   |
| `send(msg)`           | Pass 9.                                                                                                     | Pass 9.                                                                                                    |
| Any                   | Backend returns 4xx authentication error                                                                   | `status = 'conflict'`, `error.kind = 'auth-failure'`. Bypasses backoff loop.                              |
| Any                   | `attempts >= max-attempts > 0`                                                                              | `failed → conflict`, `error.kind = 'max-attempts-exceeded'`.                                              |
| Any                   | `messages` row deleted during UIDVALIDITY re-key                                                           | `pending`/`executing` → `conflict`, `error.kind = 'rekey-orphaned'` (before CASCADE).                     |
| Any                   | `messages` row deleted during forced full-refetch (`cannotCalculateChanges` / re-key fallback)             | `pending`/`executing` → `conflict`, `error.kind = 'anchor-lost'` (before CASCADE).                        |
| `send(msg)`           | Crashed mid-execute on restart                                                                             | `executing → conflict`, `error.kind = 'crashed-mid-execute'` (Pass 9).                                    |

**UIDVALIDITY re-key contract (IMAP).** The pre-revise spec
delegated re-key behavior to "the IMAP folder sync code"; the
review (D-pattern-1) flagged that this is load-bearing and
under-specified. The Cache I implementation MUST honor this
contract:

1. **Connection fence.** A UIDVALIDITY change observed on either
   the command connection (after a `SELECT` reply) or the IDLE
   connection (mid-IDLE, on reconnect) fences both connections:
   the drainer is paused for the affected folder, in-flight ops
   are aborted (transitions back from `executing` to `pending`),
   IDLE is dropped, and re-key runs against a freshly-opened
   command connection.
2. **Atomic re-key.** Re-key runs in a single SQLite transaction.
   The implementation does `UID SEARCH ALL` (optionally narrowed
   by `Date.Sent`) plus `UID FETCH (UID FLAGS RFC822.HEADER)` to
   build an old→new `protocol_id` mapping (matched by Message-ID
   and date when possible). Inside the transaction:
   - Update `messages.protocol_id` for matched rows.
   - Delete `messages` rows that didn't match.
   - Promote any pending/executing outbox row whose `messages` row
     was deleted to `conflict` with
     `error.kind = 'rekey-orphaned'` *before* the implicit CASCADE
     would silently destroy it. (CASCADE still runs for any
     leftover, but the explicit promotion fires first and writes
     the user-visible conflict row.)
   - Update `folders.uidvalidity` and reset `folders.sync_token`.
3. **Resume.** On commit, the drainer is unpaused; the syncer's
   next tick fetches the new `Changes()` baseline.

If the implementation cannot do an authoritative remap (e.g.,
`UID SEARCH` itself fails), the fallback is the forced-refetch path
described next — it shares an implementation path.

**`cannotCalculateChanges` / forced full-refetch (JMAP & IMAP).**
JMAP servers return `cannotCalculateChanges` when the cached
`SyncToken` is too stale; the IMAP path falls back here when re-key
matching fails. Both must avoid silent data loss for pending outbox
rows:

1. Fetch the full current folder listing into a staging set.
2. For each existing `messages` row in the folder, attempt to remap
   by `protocol_id` against the staging set. Matched rows survive
   with their existing `messages.id`. Unmatched rows are queued for
   delete.
3. Before deleting, promote any pending/executing outbox row whose
   `messages` row would be deleted to `conflict` with
   `error.kind = 'anchor-lost'`.
4. Commit deletes + insert new staging rows in one transaction.

This shares an implementation path with the UIDVALIDITY re-key
above; both are special cases of "wipe-and-re-anchor with pending-
op promotion."

**Outbox-row references stay at `messages.id`.** Because outbox
rows reference `messages.id` (not UIDs/JMAP ids), they continue to
work across re-keys — at replay, the queue reads
`messages.protocol_id` fresh and gets the current id. The promotion
paths above are for the cases where the message row itself cannot
survive the re-anchor.

This is the load-bearing reason Cache 0 uses local-row references in
the queue, not UIDs. Thunderbird's offline-IMAP queue wipes on
UIDVALIDITY change (`DeleteAllOfflineOpsForCurrentDB()`) — silent
data loss; the cited K-9 path has the same shape. FairEmail's design
dissolves the problem by not putting UIDs in the queue. Adopting
FairEmail's approach plus the explicit promotion paths above.

## E. Eviction (body cache)

### E.1 What's evicted

Body cache only. Header cache is the source of truth for offline
msglist rendering and is small (~200B/row × 100k rows = ~20MB even
for large accounts). Headers are removed only when `Removed` UIDs
arrive from `Changes()`.

### E.2 Three orthogonal limits

All configured under `[cache]`:

1. **`max-size`** (default `2GB`) — total body cache disk size cap.
   When exceeded, evict by `last_accessed ASC` until under cap.
2. **`max-age`** (default `90d`) — bodies whose `last_accessed` is
   older than the cap are evicted regardless of size.
3. **`max-per-folder`** (default `0` = no cap) — caps body count per
   folder. Optional defense against an archive folder crowding out
   the inbox.

### E.3 When eviction runs

- At startup (after migration, before serving UI).
- On a hourly timer while running.
- **Before** any body insert that would push total over `max-size`,
  not after — `SQLITE_FULL` fires before a post-insert eviction
  could free space (review D6). The drainer pre-checks
  `(total + new_body_size) > max_size` and runs eviction inline if
  so.
- When free disk space on the cache filesystem drops below a low
  watermark (`free_disk < 256MB` or `free_disk < 5% of partition`,
  whichever is larger). Eviction may exceed the `max-size`
  reduction target to restore the watermark.
- On `EROFS` / `ENOSPC` (see §I) the cache pauses body inserts and
  surfaces a banner; header inserts and outbox writes continue
  best-effort against WAL until that too fills.

### E.4 Pinned bodies

Currently-displayed bodies (the message in the viewer) are pinned in
an in-memory set, not on disk. The eviction sweep skips pinned
rows. The pin is released when the viewer closes the message.

## F. CLI surface

`poplar cache <subcommand>`:

- `status` — per-account: `headers: N rows`, `bodies: M rows / X MB`,
  `outbox: P pending, C conflicts, F failed`, `last sync: 3m ago
  (offline since 02:14)`, `sync token: abc123…`.
- `size [--by folder | --by account]` — disk usage breakdown.
- `clear [--bodies | --headers | --outbox | --all] [--account NAME] [--confirm]`
  — destructive. Refuses without `--confirm` on tty.
- `evict` — manually run eviction sweep against current limits.
- `vacuum` — `VACUUM` SQLite to reclaim disk after large eviction.
- `outbox [--all] [--account NAME]` — list pending/executing/failed
  ops. `--all` includes `done` audit trail.
- `conflicts [--account NAME]` — list `conflict`-status ops.
- `resolve <op-id> {retry | discard}` — non-interactive equivalent of
  the `!` overlay actions.

## G. Config surface

New top-level `[cache]` block in `config.toml`:

```toml
[cache]
# Storage
dir              = "~/.cache/poplar"  # base; per-account subdir auto-created.
                                      # Tilde-expanded; default os.UserCacheDir().
max-size         = "2GB"
max-age          = "90d"
max-per-folder   = 0                  # 0 = no cap

# Sync
sync-interval    = "5m"               # full Changes() cadence between IDLE pushes
offline-grace    = "30s"              # consecutive failures before flipping to offline

# Outbox
outbox-retention = "7d"               # how long 'done' ops persist for audit
backoff-min      = "1s"
backoff-max      = "60s"
max-attempts     = 10                 # cap; 0 = retry network failures forever (opt-in).
                                      # On exceed, failed → conflict with
                                      # error.kind = 'max-attempts-exceeded'.
```

`config.LoadCache()` mirrors `config.LoadUI()` (ADR-0102). Defaults
applied at decode time. Validation errors carry context.

**Why top-level `[cache]`, not `[ui.cache]`.** The cache is a
subsystem on par with accounts and UI. Future surfaces (the
hypothetical `poplar export-mbox`, a notification daemon) are also
cache consumers and would not look under `[ui]` for cache config.

## H. UI surfaces (Cache III)

All three are deviations from the GUI mail-client field. Justified
as TUI-native ergonomics that GUI clients deprioritized but did not
argue against.

### H.1 Status-bar offline badge

Renders when *any* of:

- An account's drain goroutine is in offline mode.
- An account's outbox has rows in `failed` state.

Format (status bar): `OFFLINE · 3 pending` (offline mode) or
`3 pending` (online but ops stuck) or absent (steady state).

Bubbles analogue: existing status bar; new text segment.

### H.2 Outbox overlay (key `Q`)

Modal list of pending + executing + failed ops across all accounts.
Per-row actions:

- `c` — cancel (only valid for `pending` and `failed`; removes the
  outbox row and reverts the optimistic UI state).
- `r` — force retry (move `failed` → `pending` immediately,
  bypassing backoff).

Bubbles analogue: `bubbles/list` inside an overlay using the same
modal pattern as the help popover (ADR-0082) and link picker
(ADR-0087). Implementation cribs the overlay frame from those.

### H.3 Conflicts overlay (key `!`)

Modal list of `conflict`-status ops. Per-row actions:

- `r` — retry (move to `pending`).
- `d` — discard (move to `done` with an `error.discarded = true`
  marker).

Same `bubbles/list` overlay pattern as `Q`.

### H.4 Per-message indicator (optional, Cache III polish)

A subtle marker on msglist rows whose `messages.id` matches a
`pending` / `executing` / `failed` outbox row. Single-cell symbol or
color tint. Not load-bearing; can defer to Polish II if needed.

## I. Failure modes designed against

These are the scenarios the design must handle correctly. Each is
called out so the review pass can verify the design holds.

1. **UIDVALIDITY change between enqueue and replay.** Solved
   structurally — outbox rows reference `messages.id`; UIDs read
   fresh at replay time.
2. **Message destroyed remotely while op pending.** Solved by FK
   CASCADE; outbox row removed when message row removed.
3. **Crash mid-execute on idempotent op.** `executing` state
   persists; restart resets to `pending`; safe to replay.
4. **Crash mid-execute on non-idempotent op (`send`).** `executing`
   state persists; restart marks `failed` with
   `error.kind = 'crashed-mid-execute'`; user resolves manually.
   (Cache 0 schema only; behavior implemented in Pass 9.)
5. **Sync failure cascade hides offline state from user.** Mitigated
   by `offline-grace` window. Auth errors stay error-banner;
   network errors trip offline.
6. **Outbox grows unbounded if user permanently offline.** No hard
   cap. `poplar cache outbox` lets user inspect; `cache clear
   --outbox` purges. Warning log when outbox exceeds 1000 entries
   per account.
7. **Eviction races body fetch.** Pinned-bodies in-memory set
   prevents eviction of currently-displayed body.
8. **Backend returns success but op was actually a conflict** (e.g.
   move target was renamed). Detected by next `Changes()` cycle;
   `messages` row converges to current server state. Lost intent,
   but no data loss.
9. **Two queued ops on the same message arrive in the wrong order
   under concurrency.** Single drain goroutine per account
   guarantees enqueue-order replay. Cross-account ops are
   independent.
10. **SQLite WAL grows unbounded.** Periodic `wal_checkpoint(TRUNCATE)`
    on the drain goroutine's idle ticks.
11. **OAuth token expires mid-drain (review D3).** Auth errors map
    to `conflict` with `error.kind = 'auth-failure'`, not the
    backoff loop. User re-authenticates.
12. **Forced full-refetch destroys pending intent (review D4 /
    D-pattern-2).** `cannotCalculateChanges` and IMAP re-key
    fallback both run the pre-wipe remap path in §D.4. Unremapped
    pending rows → `conflict` with `error.kind = 'anchor-lost'`.
13. **UIDVALIDITY change mid-drain (review D9 / D-pattern-1).**
    Either-connection observation fences both connections;
    in-flight ops abort; re-key runs atomically. See §D.4.
14. **`ResolveConflict` races the drainer (review D5).**
    `ResolveConflict` issues `UPDATE ... WHERE id = ? AND status =
    'conflict'` and checks `RowsAffected`; `0` returns a
    user-visible "already changed" error that triggers an overlay
    refresh. SQLite WAL serializes the write; the drainer never
    promotes a `conflict` row, so the only race is user-vs-user
    via the same overlay (a `discard` on a row another action just
    `retry`'d) — handled by the same `RowsAffected` check.
15. **Account removed from config while DB has pending ops (review
    D10).** Startup scans `$XDG_CACHE_HOME/poplar/*/mail.db`. Any
    DB whose slug doesn't resolve to a configured account and has
    non-`done` outbox rows surfaces a startup warning naming the
    account and the pending count. The DB is not auto-deleted;
    user runs `poplar cache clear --account <slug> --confirm`.
16. **Disk full / read-only filesystem (review C7 / D6).** `EROFS`
    and `ENOSPC` from SQLite writes flip the cache to a
    degraded mode: body inserts paused, header inserts and outbox
    writes attempted best-effort, error banner shows "cache disk
    unavailable." UI reads continue from existing rows.

## J. Open questions — review outcomes

The pre-revise spec listed five open questions. Pass 8.4-review +
8.4-revise resolved each:

1. **Unified write path migration cost** — resolved. Pass 8.4a's
   plan (`docs/superpowers/plans/2026-05-02-cache-i-implementation.md`)
   prescribes a strangler-fig order: cache writes first (with
   `MessageList.Apply*` and old backend Cmds still alive), then
   cache-backed reads, then deletion of the legacy paths. The
   intermediate window is safe because the cache becomes the single
   source of truth at step 2 (review C1).
2. **`ui_flags` + `ui_hide` split** — confirmed. The split is
   load-bearing once the syncer/drainer coordination invariant
   (§C.3) is in place. A single-state design would force the syncer
   to either suppress server updates for in-flight ops (same
   complexity in a different place) or stomp the optimistic state
   (review B4).
3. **No drain-time coalescing** — confirmed. JMAP backend is free
   to coalesce at the `Email/set` boundary; IMAP per-op latency
   is acceptable on a long-lived connection. Revisit if profiling
   later shows otherwise.
4. **`max-attempts` default** — changed from `0` (forever) to `10`
   (cap). Permanent failures (auth, deleted folder, server quota)
   no longer loop silently. Users on intermittent links can still
   set `max-attempts = 0` (review B6 / D3, ADR-0116).
5. **`!` and `Q` overlay keys** — kept as-is (TUI-native invention,
   no conflict with vim conventions on the account view). Revisit
   if user feedback during pre-beta shows discovery friction;
   helped by help-popover entries (ADR-0072).

## Sources read for this design

- FairEmail `EntityOperation.java` — operation queue schema, FK
  CASCADE pattern, dedup-at-enqueue.
- FairEmail `ServiceSend.java` — send semantics in unified queue,
  `executing` state for crash recovery.
- Mailspring-Sync `Task.hpp` + `TaskProcessor.cpp` — unified write
  path, `performLocal/performRemote` two-phase, status state machine.
- Thunderbird desktop `nsIMsgOfflineImapOperation.idl` +
  `nsImapOfflineSync.cpp` — bitfield-per-message model (rejected),
  type-ordered replay (rejected), automatic coalescing (rejected),
  UIDVALIDITY wipe (rejected).
- meli `melib/src/imap/sync/mod.rs` — confirmed: read-side cache
  only, no offline-write queue. Confirms poplar would be the first
  TUI mail client with offline triage queueing.
- himalaya — confirmed: stateless CLI, no offline.
- offlineimap / mbsync — UIDVALIDITY hard-stop pattern (rejected).
- Outlook (CEM) — silent-Conflicts-folder pattern (rejected).
- K-9 Mail `MessagingControllerCommands.java`,
  `MessagingController.java`, `OutboxStateRepository.kt` — explicit
  `RETRIES_EXCEEDED` terminal state (adopted as
  `max-attempts-exceeded`); UID-in-blob queue (rejected, same
  failure mode as Thunderbird desktop); head-of-line-blocking
  drainer (rejected).
- Camel (Evolution) `camel-imapx-folder.c`,
  `camel-imapx-message-info.c` — server-flags-only model; surfaces
  the syncer-vs-drainer ordering risk that the §C.3 coordination
  invariant addresses.
- RFC 4549 §6 — drain-first ordering for offline queues
  (ADR-0113).
- RFC 7162 — CONDSTORE/QRESYNC; CONDSTORE asserted at IMAP
  Connect.
