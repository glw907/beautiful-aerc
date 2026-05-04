---
title: Attachment cache shape — separate v5 table, lazy bytes
status: accepted
date: 2026-05-04
---

## Context

Pass 8.6 adds attachment metadata + downloaded bytes to the
cache. Two storage shapes were considered:

1. Extend the existing `bodies` table into a generic `parts`
   table holding the body part(s) and every attachment.
2. A separate `attachments` table parallel to `bodies`,
   sharing the message FK.

Bodies and attachments have different size profiles (bodies
typically < 100 KB; attachments often MB-scale) and the user
controls them with different cache caps. Conflating them
would force every body lookup to disambiguate by part type
and would couple eviction policy across two very different
workloads.

## Decision

New v5 `attachments` table parallel to `bodies`:

```
CREATE TABLE attachments (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    message      INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    part_id      TEXT    NOT NULL,
    filename     TEXT    NOT NULL DEFAULT '',
    mime_type    TEXT    NOT NULL,
    size         INTEGER NOT NULL,
    content_id   TEXT    NOT NULL DEFAULT '',
    disposition  TEXT    NOT NULL,
    bytes        BLOB,
    fetched_at   INTEGER,
    UNIQUE (message, part_id)
)
```

Metadata + bytes share the row. `bytes` is nullable: the row
exists from the first `Attachments(uid)` call (metadata-only);
`bytes` populates lazily on first `FetchAttachment(uid, partID)`
under a separate size backstop (`max-attachment-size`,
default 2 GB, plumbed through `cache.Config.MaxAttachmentSize`).

Eviction is by oldest `messages.sent_at` and clears
`bytes`/`fetched_at` rather than deleting the row — metadata
must survive so the viewer (Pass 8.7) can still list parts
without a backend roundtrip after eviction.

A zero-length `Attachments(uid)` result is not cached:
distinguishing "empty" from "not yet populated" would require
a marker column, and the cost of an occasional re-fetch on
truly attachment-free messages is lower than the schema
weight.

## Consequences

- Bodies and attachments evict independently.
- Metadata is permanent until the message row is deleted;
  bytes are best-effort.
- Schema bumped to v5 with a single forward migration.
