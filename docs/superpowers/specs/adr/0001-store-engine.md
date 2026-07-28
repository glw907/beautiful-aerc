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
envelope; its relief valves (statement caching, list-path snippet
column) are pre-planned. FTS5 is derived state and rebuildable, so
index corruption is a repair path, never data loss.
