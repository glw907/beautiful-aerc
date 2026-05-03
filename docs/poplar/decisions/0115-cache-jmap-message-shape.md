---
title: Cache 0 — JMAP one-row-per-Email message shape via junction table
status: accepted
date: 2026-05-02
---

## Context

ADR-0110 specified `messages` with a `folder` column and
`UNIQUE (folder, protocol_id)`. Pass 8.4-review (subagent A,
finding A4) found this doesn't fit JMAP semantics: one Email has
one id and an `mailboxIds: Id[Boolean]` set; `Email/set update` on
`mailboxIds` is a "move" without producing a new id. Multi-mailbox
Emails would either need duplicate rows (violating the natural
JMAP key) or a per-row "primary mailbox" concept that doesn't
exist in the protocol.

Two shapes were considered: (a) one `messages` row per Email per
account with a primary `folder` column plus a side junction table
for multi-mailbox cases, or (b) drop `messages.folder` entirely
and put all mailbox membership in a junction table.

(b) is uniformly cleaner: it works identically for IMAP
(membership is a single row in the junction) and JMAP (membership
is N rows). Folder queries become a single join either way. IMAP
`MOVE` becomes "rewrite the junction row plus update
`protocol_id`" in the same transaction. JMAP `Email/set
mailboxIds` becomes "replace the junction set."

## Decision

`messages` carries `protocol_id TEXT NOT NULL UNIQUE` (per-account
DB makes uniqueness account-scoped) and no `folder` column. A new
`message_mailboxes(message, folder)` junction table holds folder
membership with `PRIMARY KEY (message, folder)` and CASCADE on
both FKs. An index on `message_mailboxes(folder)` supports the
folder query path.

Folder queries:

```sql
SELECT m.* FROM messages m
JOIN message_mailboxes mm ON mm.message = m.id
WHERE mm.folder = ? AND m.ui_hide = 0
ORDER BY m.sent_at DESC
LIMIT ?, ?;
```

IMAP `MOVE` (drainer): in one transaction, update
`messages.protocol_id` to the new UID and replace the junction row
to point at the new folder. JMAP `Email/set mailboxIds`: replace
the junction set to match the new `mailboxIds` map.

Outbox `move` ops carry `MoveArgs{Dest}` and the source folder via
the existing `outbox.folder` column. The drainer maps to
`Backend.Move([uid], dest)` (IMAP) or to `Email/set
mailboxIds: { [src]: null, [dest]: true }` (JMAP).

## Consequences

- Narrows ADR-0110 (storage architecture). The junction-table
  shape is the binding statement; ADR-0110's other decisions
  (per-account DB, modernc.org/sqlite, WAL pragmas, FK CASCADE)
  stand.
- The schema accommodates JMAP Email-in-N-mailboxes without
  duplicating headers. Eviction, body cache, and outbox queries
  all key off `messages.id` and don't change.
- IMAP behavior is unchanged at the user-visible layer — exactly
  one junction row per message in an IMAP cache. The extra join
  is cheap with the `message_mailboxes_folder` index.
- The pre-revise `UNIQUE (folder, protocol_id)` constraint is
  removed; `messages.protocol_id UNIQUE` replaces it.
- No `messages.folder` column means thread queries and msglist
  ordering go through the junction. `messages_thread` is now
  unscoped by folder; that's correct under the junction model
  (a thread can span folders for JMAP).
- v1.0-frozen schema includes the junction table from day one.
  Adding it post-beta would be a destructive migration.
