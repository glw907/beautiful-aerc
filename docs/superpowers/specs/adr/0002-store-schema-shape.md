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

## Revision 2 (2026-07-27, post-review)

- **One database file for all accounts** (`store.db`), not one per
  account: `account_id` is the isolation mechanism, the single
  writer and lock stay true at N accounts, and cross-account
  views stay reachable. Revision 1 stated both designs at once.
- **The index set is a schema artifact**: the covering list index
  `message_mailbox(mailbox_id, received_at DESC, message_id)`,
  the thread index `message(account_id, thread_key,
  received_at)`, the partial unread index, `outbox(state,
  next_attempt_at)`, and the occurrence indexes. Every hot query
  carries an `EXPLAIN QUERY PLAN` golden. Revision 1 named no
  indexes; the review showed the list query was unservable.
- **Reserved LATER scalars exist from migration one**
  (`message.hidden_until`, `thread.muted`, `mailbox.visible`,
  `message.origin`): a JSON reservation cannot back a hot-path
  predicate under this ADR's own rule, and `origin` is what keeps
  ST-4 import open across resync.
- **`search_text` is denormalized onto `message`** and
  `message_fts` has a single content source; the two-table
  external-content shape revision 1 described is not
  constructible. Resync reconciles by `server_id` and never
  re-mints internal keys. The account-column test is stated as:
  every table not reachable by FK from a scoped parent carries
  `account_id` (covers `sent_history`).
