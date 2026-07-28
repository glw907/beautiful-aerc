# ADR-0001: modernc.org/sqlite is the store engine

Date 2026-07-27. Status: accepted (Phase 4). Survey:
`docs/poplar/research/2026-07-27-phase4-library-survey.md`.

## Context

C1 requires a local store behind every interactive read; C3 requires
`CGO_ENABLED=0`; SR-1 requires a full-text index transactionally
consistent with message mutations; QA-6 requires surviving SIGKILL
at arbitrary points; QA-2/3 set latency gates at a 100k-message
envelope.

## Decision

The store is SQLite through modernc.org/sqlite (pure-Go transpile,
FTS5/WAL/JSON1 compiled in), one database file per account, WAL
mode, with a single write connection owned by one writer goroutine
and a separate read pool. Full-text search is FTS5 with
external-content tables. Migrations are a hand-rolled
`schema_version` runner over `go:embed` SQL files.

## Alternatives considered

- **ncruces/go-sqlite3** (wasm): faster in some published read
  benchmarks, but its FTS5 module is a separate pre-1.0 package
  with near-zero adoption, the runtime just migrated
  wazero→wasm2go, and an open WAL corruption bug (Windows) is
  disqualifying evidence against the QA-6 bar today. Re-evaluate
  at a future major.
- **mattn/go-sqlite3**: cgo; C3 excludes it outright.
- **KV store (bbolt/pebble) + bleve**: hand-builds the
  transactional store-index consistency FTS5 provides, doubles the
  stores to keep crash-consistent, and only pays off at 10-100x
  the envelope.
- **Migration frameworks (goose, golang-migrate)**: fine tools
  aimed at server fleets; the embedded runner is under a hundred
  lines and keeps a dependency out of the trust base.

## Consequences

The concurrency discipline (ADR-0003) is load-bearing: modernc
connections are full-mutex, so reads must never queue behind the
writer. The measurement spike validates the discipline at the
envelope; its relief valves (statement caching, and the
denormalized `search_text` column keeping hot reads off the body
table) are pre-planned. FTS5 is derived state and rebuildable, so
index corruption is a repair path, never data loss.

## Revision 2 (2026-07-27, build boundary)

Two clauses of the Decision above are superseded. Read them
through this block; the Decision text is left intact as the
record of what was decided when.

**One store file holds every account.** The Decision says "one
database file per account". Technical design section 3 settled it
the other way, and ADR-0002's revision 2 agrees, so the Decision
has been wrong since the design landed. One database is what
makes a cross-account view a query rather than a merge, keeps the
FTS5 index single and transactionally consistent with the
mutations it indexes, and holds ADR-0003's discipline to one
writer rather than one per account. Multi-account is the named
first post-v1 priority, so the schema carries `account_id` from
the first migration even though v1 ships one account.

The cost is that a corrupt file takes every account with it.
SY-8's failure tests cover that path, and FTS5 stays derived
state that rebuilds without data loss.

**FTS5 uses a single content source, not external-content
tables.** The Decision's "external-content tables" is retracted by
ADR-0002 revision 2, which found the two-table external-content
shape not constructible as specified. `message_fts` indexes
`message(subject, search_text)` from one source, maintained in the
same transaction as the message write.
