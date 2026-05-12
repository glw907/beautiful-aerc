---
title: Outbox per-row size cap
status: accepted
date: 2026-05-12
---

## Context

Audit E F4 (ADR-0225): `outbox.payload` carries the full assembled
MIME bytes for every Send/Append/PushDraft row, and the drainer
holds rows indefinitely on transient failure. A stalled multi-MB
message against a disconnected backend bloats the per-account
SQLite file with no upper bound, and the cache has no signal to
refuse a payload at queue time. ADR-0122's body-cache backstop
covers `bodies.bytes` but not `outbox.payload`.

## Decision

Add `[cache] max-outbox-bytes` (TOML; default 0 = unlimited;
mirrors `max-size`'s "0 disables" semantics from ADR-0122).

- `config.CacheConfig.MaxOutboxBytes int64` threads into
  `cache.Config.MaxOutboxBytes` and onto `*Account.maxOutboxBytes`.
- `cache.insertFolderOp` rejects with the new sentinel
  `cache.ErrOutboxRowTooLarge` when `maxOutboxBytes > 0 &&
  len(payload) > maxOutboxBytes`. The check fires before the
  INSERT transaction opens, so no optimistic UI flip needs
  reversal.
- `QueueOutbound` propagates the error verbatim. The JMAP
  one-op shape returns `(nil, err)`; the IMAP two-op shape never
  reaches the `Append` row because the `Send` row failed first.
- Writer round-trip emits `max-outbox-bytes = "<size>"` only when
  the value differs from the default, matching the existing
  `max-size`/`max-attachment-size` writer pattern.

Inline fix: `cmd/poplar/root.go` was threading only `MaxSize` into
`cache.Config`, dropping the user's `max-attachment-size` and the
new `max-outbox-bytes`. Fixed by passing all three fields.

## Consequences

Catches the stuck-large-payload scenario at the API boundary
instead of letting the drainer churn on it. Users get an inline
banner via the compose surface (the existing `c.err` path) when
their outbound message exceeds the cap, with the byte count and
the configured limit in the error message.

Default 0 = unlimited preserves prior behavior for everyone who
doesn't set the field. Operators with constrained disk budgets
opt in. No migration; the column already exists.
