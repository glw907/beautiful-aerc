---
title: Cache 0 — local mail cache storage architecture
status: accepted (narrowed by 0114, 0115)
date: 2026-05-02
---

## Context

Poplar needs a local cache for offline reads, fast msglist rendering,
and the foundation for offline triage (Cache III). The cache becomes
a v1.0-frozen on-disk format per ADR-0105, so storage choices made
now become migration debt later.

Three storage candidates were considered: pure-Go SQLite
(`modernc.org/sqlite`), BoltDB (`go.etcd.io/bbolt`), and a
filesystem-of-files layout. SQLite wins on indexed queries (the
msglist's "newest N in folder X by SentAt" pattern), eviction
queries (`SUM(body_bytes)` and `ORDER BY last_accessed`), and
cross-table transactional updates (queue + optimistic UI in one
commit).

Storage layout was decided between one shared DB (with `account_id`
columns) and per-account DBs.

## Decision

**Driver:** `modernc.org/sqlite` (pure Go, no cgo). WAL mode,
foreign-key enforcement on, NORMAL synchronous, 5s busy timeout.

**Layout:** one SQLite database per account at
`$XDG_CACHE_HOME/poplar/<account-slug>/mail.db` (Linux/macOS;
`%LOCALAPPDATA%\poplar` on Windows). `<account-slug>` is the account
name lower-cased with non-`[a-z0-9-]` replaced by `-`. Slug
collisions detected at startup.

Account add = create directory + open fresh DB (auto-migrates from
schema 0). Account remove = `rm -rf` the account directory.
Per-account schema migrations run in parallel on connect.

Schema (full SQL in `docs/superpowers/specs/2026-05-02-cache-0-design.md`
section A.3) covers: `schema_version`, `folders`, `messages`,
`bodies`, `outbox`. All FKs use `ON DELETE CASCADE`. `messages` carries
both server-confirmed `flags` and optimistic `ui_flags` /
`ui_hide` columns (FairEmail pattern). `messages.protocol_id` is
TEXT (JMAP id strings or stringified IMAP UIDs).

A new `mail.ChangeTracker` interface sits beside `mail.Backend`. Both
v1 backends implement both. The cache holds both pointers per
account and uses `ChangeTracker.Changes(ctx, folder, since)
(ChangeSet, SyncToken, error)` for delta sync.

## Consequences

- New `internal/cache/` package owns the SQLite handles. New
  `internal/cache/Account` type holds `Backend`, `ChangeTracker`,
  `*sql.DB`.
- `mail.Backend` shrinks; `MarkRead`/`MarkUnread`/`MarkAnswered`/
  `Delete` collapse into `Flag` (Cache I task).
- Per-account isolation: corruption blast radius is one account;
  one stuck migration doesn't block startup of healthy accounts.
- Backup/grep/inspect stories work on a single-account file.
- Adding an account is purely additive — no shared-DB migration.
- `modernc.org/sqlite` adds ~5MB to the binary; one-time cost,
  doesn't compound.
- Implementation work split across Pass 8.4a (schema + headers +
  ChangeTracker), 8.4b (bodies + eviction + CLI), 8.4c (outbox +
  offline + overlays). Each pass has its own plan; review pass
  (8.4-review) gates implementation.
- **Narrowed by ADR-0115** — `messages` schema uses a junction
  table `message_mailboxes` rather than a `folder` column +
  `UNIQUE (folder, protocol_id)`, to fit JMAP's
  one-Email-N-mailboxes shape.
- **Narrowed by ADR-0114** — UIDVALIDITY re-key contract is
  spelled out (atomic transaction, connection fence, explicit
  promotion of orphaned outbox rows to `conflict`); the bare
  "FK CASCADE handles it" framing in this ADR is insufficient.
