# Cache II — body cache + manual eviction + `poplar cache` CLI

**Pass:** 8.4b
**Date:** 2026-05-03
**Status:** draft

## Goal

Land the body-cache layer on top of the Cache I foundation: make
`(*cache.Account).FetchBody` a write-through cache against the
`bodies` table, ship a single size-cap backstop, and expose a
`poplar cache` CLI for inspection and manual eviction.

## Non-goals

- Automatic age-based eviction (no background sweeper, no LRU
  touch-on-read).
- Body prefetch on `Changes()` arrival.
- Body compression.
- The full §F CLI surface from the Cache 0 spec
  (`clear`/`size`/`outbox`/`conflicts`/`resolve`) — those land
  with Cache III alongside outbox/offline.
- Per-folder caps. Archival opt-out flags.
- Low-disk watermark eviction. `EROFS`/`ENOSPC` banner. (Cache III.)

## Prior-art framing

Six of eight major clients (Apple Mail, Fastmail desktop, mutt,
neomutt, Thunderbird, Evolution) ship **no automatic body
eviction policy**. The two that do — Outlook and Geary — implement
sent-date *fetch windows*, not LRU eviction on access time. K-9
caps message *count* per folder.

Fastmail's new desktop app (Oct 2025) ships the closest analogue
to what poplar does naturally: **lazy population**. Bodies enter
the cache only when the user opens a message; old untouched mail
is never cached. Cache II adopts that idiom plus a single size
backstop for the runaway-growth case.

The Cache 0 spec's LRU-on-`last_accessed` model (ADR-0110/0111/
0112) is more aggressive than any major client. This pass
narrows it.

## Policy

**Lazy population.** `FetchBody` is the only path bodies enter
the cache. Cache miss → backend fetch → store. Cache hit → return
stored bytes, no `last_accessed` UPDATE.

**No automatic eviction.** No startup sweep. No hourly timer. No
low-disk watermark. The cache grows with use and is bounded only
by the size backstop and manual prune.

**Single backstop:** `[cache] max-size` (default `2GB`). Enforced
inline in the store path: if `current_total + len(new_body) >
max_size`, evict by `messages.sent_at ASC` until under cap, then
insert. Set to `0` to disable the cap.

**Manual prune:** `poplar cache evict --older-than DUR` filters
by `bodies.fetched_at` (age in cache, not sent date). Matches the
user mental model: "drop bodies I haven't refreshed in a year."

## Schema (v3 → v4)

Drop `last_accessed` + `bodies_lru`:

```sql
-- migration v4
DROP INDEX bodies_lru;
ALTER TABLE bodies DROP COLUMN last_accessed;
```

Final v4 shape:

```sql
CREATE TABLE bodies (
    message    INTEGER PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    bytes      BLOB    NOT NULL,
    fetched_at INTEGER NOT NULL  -- unix nanos
);
```

No new index. Size-backstop eviction joins through
`messages_sent` (existing index on `messages.sent_at`); age-prune
queries `fetched_at` directly with a row scan — acceptable
because the table is small (one row per opened message).

Bodies stored as raw RFC822 bytes drained from
`mail.Backend.FetchBody`. No compression in 8.4b.

`SQLite DROP COLUMN` is supported since 3.35 (March 2021);
`modernc.org/sqlite` is well past that floor.

## API changes

### `mail.Backend.FetchBody`

Signature changes from `(io.Reader, error)` to `([]byte, error)`.
The cache needs the size before insert and exposes hits without
re-reading from disk on every call. Both backends
(`mailjmap`, `mailimap`) and the viewer's `bodyFetchCmd` are
updated to the new shape.

Pre-beta posture: change in place, no compat shim.

### `(*cache.Account).FetchBody`

Becomes write-through:

```go
func (a *Account) FetchBody(uid mail.UID) ([]byte, error) {
    if b, ok := a.lookupBody(uid); ok {
        return b, nil
    }
    body, err := a.Backend.FetchBody(uid)
    if err != nil {
        return nil, err
    }
    a.storeBody(uid, body)  // best-effort; logs on failure
    return body, nil
}
```

`lookupBody`: one-statement read,
`SELECT bytes FROM bodies WHERE message = (SELECT id FROM messages WHERE uid = ?)`.

`storeBody`: opens a transaction, computes
`SELECT SUM(length(bytes)) FROM bodies`, runs size-backstop
eviction if needed, then `INSERT OR REPLACE` the body row.
Failure logs but does not propagate — the body is still returned
to the caller.

### Concurrency

Per-account sqlite pool serializes via WAL + busy_timeout. A
concurrent `FetchBody` for the same UID either hits the
freshly-stored row or races into a parallel backend fetch
(rare, idempotent — `INSERT OR REPLACE` handles the duplicate).
No extra locking.

## Eviction queries

```sql
-- size backstop: oldest sent first, until under cap
DELETE FROM bodies WHERE message IN (
    SELECT b.message FROM bodies b
    JOIN messages m ON m.id = b.message
    ORDER BY m.sent_at ASC
    LIMIT ?
);

-- manual --older-than: by fetched_at
DELETE FROM bodies WHERE fetched_at < ?;
```

Size eviction loops in batches of (say) 32 rows, recomputing
`SUM(length(bytes))` each pass, until under cap. The batch
bounds the worst-case transaction size; the loop bounds the
worst-case adversarial input.

## Config surface

New top-level `[cache]` block in `config.toml`:

```toml
[cache]
# Body cache size cap. Evicts oldest-by-sent-date until under cap
# when a new body would push total over. Set to 0 to disable.
max-size = "2GB"
```

Decoded by `internal/config` alongside `[ui]`. Threaded into
`cache.Open` via a `cache.Config` struct
(`{ MaxSize int64 }`).

`parseSize` accepts `KB`/`MB`/`GB`/`TB` suffixes (1024-based).
Default `2 << 30` if unset. Validation: non-negative, "did you
mean `2GB`?" hint on garbage strings, matching the existing
provider-suggestion pattern.

`config.Template()` first-run TOML grows a commented-out
`[cache]` section.

Out of scope for 8.4b: `cache.dir` override (Cache I uses
`os.UserCacheDir()`, deferred until needed),
`[cache.sync.*]` (Cache III).

## CLI surface

New file `cmd/poplar/cache.go`. Cobra parent
`poplar cache` with three subcommands:

### `poplar cache stats`

Per-account, one row in a tab-aligned table:

```
ACCOUNT     HEADERS    BODIES          OUTBOX           DB SIZE
fastmail    1,247      342 / 18.4 MB   3 pending        42.1 MB
work        8,912      1,103 / 612 MB  0                638.4 MB
```

- `HEADERS` = `COUNT(*) FROM messages`
- `BODIES` = `COUNT(*)` + `SUM(length(bytes))` from `bodies`
- `OUTBOX` = aggregate of pending/executing/failed/conflict
  (uses the existing outbox shape from Cache I)
- `DB SIZE` = `pragma page_count * page_size`

Sizes humanized via a small `humanizeBytes` helper.

### `poplar cache evict --older-than DUR [--account NAME]`

`DUR` parsed by `time.ParseDuration` plus `d`/`w` suffix
extension. Default scope: all accounts.
`--account NAME` scopes to one.

Output (per account):
`evicted N bodies (X MB freed) from <account>`

No `--confirm` — eviction is reversible (re-fetched on next
view). Empty result is a valid outcome:
`evicted 0 bodies from <account>`.

### `poplar cache vacuum [--account NAME]`

Runs `VACUUM` per account. Output:
`vacuumed <account>: N MB → M MB`.

`VACUUM` requires no other readers; the implementation drains
the pool (or runs against a fresh single connection that
bypasses the pool) before issuing the statement. Decision:
plan doc picks whichever is cleaner; spec only specifies the
visible behavior.

### Out of scope for 8.4b

Push to Cache III (Pass 8.4c) per the starter prompt:

- `cache size [--by folder|account]`
- `cache clear [--bodies|--headers|--outbox|--all]`
- `cache outbox`
- `cache conflicts`
- `cache resolve <op-id>`

## File layout

```
internal/cache/
  bodies.go         NEW — lookupBody, storeBody, evictBySize, evictByAge
  schema.go         EDIT — v3→v4 migration
  reads.go          EDIT — FetchBody write-through
  account.go        EDIT — Config struct, threaded through Open
  bodies_test.go    NEW — table-driven: hit, miss, eviction, age prune

internal/mail/
  backend.go        EDIT — FetchBody signature → ([]byte, error)

internal/mailjmap/backend.go    EDIT
internal/mailimap/backend.go    EDIT

internal/config/
  cache.go          NEW (or fold into ui.go) — [cache] block + parseSize
  template.go       EDIT — commented [cache] in first-run TOML

internal/ui/cmds.go             EDIT — loadBodyCmd drops io.ReadAll
internal/ui/cmds_test.go        EDIT — blockingBackend FetchBody returns []byte
internal/ui/account_tab_test.go EDIT — pagingFakeBackend FetchBody returns []byte
internal/cache/cache_test.go    EDIT — fakeBackend FetchBody returns []byte

cmd/poplar/
  cache.go          NEW — cobra parent + stats/evict/vacuum
  cache_test.go     NEW — flag parsing, output format
  root.go           EDIT — register cache subcommand
```

## Test plan

**`internal/cache/bodies_test.go`:**

- Cache hit returns stored bytes; backend not called.
- Cache miss calls backend exactly once; second call hits cache.
- Size backstop: insert N bodies summing past `max_size`,
  verify oldest-by-`sent_at` evicted first, total under cap.
- Age prune: insert bodies with varying `fetched_at`, run
  `evictByAge(cutoff)`, verify only older rows removed.
- Concurrent fetch of same UID does not double-store
  (`INSERT OR REPLACE` semantics).
- `MaxSize == 0` disables the cap (no eviction even with
  large bodies).

**Schema migration:** open a v3 db with populated
`last_accessed`, run migration, verify column dropped, index
gone, body bytes intact.

**CLI tests** (`cmd/poplar/cache_test.go`):

- `stats` formatting with fixture account states.
- `evict --older-than` flag parsing including `7d`, `1w`,
  `48h` cases.
- `--account NAME` scoping (unknown name → error).
- `vacuum` end-to-end on a temp db.

**Existing cache tests:** all must continue green after the
`FetchBody` signature change.

## ADRs to write at pass end

- **ADR-0122** — Cache II policy: lazy population, no automatic
  eviction, single size backstop. Cite Apple Mail / Fastmail
  desktop / mutt prior art. Narrows ADR-0110/0111/0112's LRU
  portions.
- **ADR-0123** — Schema v4: drop `last_accessed` + `bodies_lru`.
  Ties to ADR-0122.
- **ADR-0124** — `mail.Backend.FetchBody` returns `[]byte`.
  Cite the cache-write-path requirement.

## Risks / open questions

- **`VACUUM` + connection pool.** `VACUUM` cannot run with
  active readers. Implementation must drain the pool or use a
  bypass connection. Plan doc decides; not a design risk, just
  an implementation detail.
- **Body size for very large messages.** A 50MB message body
  goes into RAM via `io.ReadAll` regardless. Acceptable for
  v1; truly large bodies (mailing list digests, attachment-
  laden) are a Cache III streaming concern.
- **Migration data loss risk.** `DROP COLUMN` on a table with
  existing rows is supported; no body bytes touched. The
  migration test guards this.
