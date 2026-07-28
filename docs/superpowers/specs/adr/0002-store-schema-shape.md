# ADR-0002: fat-table hybrid schema with poplar-minted keys

Date 2026-07-27. Status: accepted (Phase 4).

## Context

The store backs mail, calendar, contacts, outbox, and sync state
(SY-1), must stay migration-light across a fast-evolving model,
must be account-keyed from v1 (C4), and must keep the LATER items
(labels, snooze, saved searches, contact writes) open at zero cost.

## Decision

Every entity table pairs a `data` JSON column (the full serialized
model) with scalar columns for exactly the fields queries sort,
filter, or join on. Poplar mints an internal 64-bit primary key on
every row; server identifiers are indexed, replaceable side
columns. Every account-scoped table carries `account_id`, asserted
by a schema test. Message-to-mailbox membership is a join table
(multi-membership stays open for labels); flags are a bitfield
plus JSON overflow; `hiddenUntil` is reserved in the model for
snooze.

## Alternatives considered

- **Fully normalized schema**: every model change becomes a
  migration; the legacy client's churn showed how often the model
  moves. Mailspring ships the hybrid at scale and documents the
  trade (slower full-table scans, which poplar's indexed columns
  avoid on every hot path).
- **Server ids as primary keys**: Thunderbird Panorama's inherited
  per-folder key model is the documented cautionary tale; resync
  and backend migration both want server ids to be replaceable.
- **Separate cache database beside a canonical store** (aerc's
  LevelDB-plus-blob-files): two stores to keep crash-consistent;
  the QA-6 invariants get strictly harder for no capability.

## Consequences

New model fields are JSON-only changes. Queries touch indexed
columns; the JSON is hydration payload. The schema review checks
that no query predicate reaches into JSON on a hot path. Deleting
and re-syncing reproduces equivalent state over the SY-1 field set
because server-derived rows are disposable by construction.
