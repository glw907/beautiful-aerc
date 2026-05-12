---
title: UPSERTs that read the rowid downstream use RETURNING id
status: accepted
date: 2026-05-14
---

## Context

Pass 40 (Audit G) surfaced a runtime FK error at startup:

```
query folder: link message StpgQ3PoV__Z ↔ folder:
constraint failed: FOREIGN KEY constraint failed (787)
```

`upsertMessages` in `internal/cache/reads.go` did
`INSERT INTO messages … ON CONFLICT(protocol_id) DO UPDATE …`
then read `res.LastInsertId()` and used the result as the foreign
key in a subsequent `INSERT INTO message_mailboxes (message,
folder) VALUES (?, ?)`. A fallback `if id == 0 { SELECT id … }`
guarded the fresh-connection case but missed the stale-non-zero
case.

SQLite's `sqlite3_last_insert_rowid()` is connection-scoped and
updates only on a real INSERT; on the UPDATE branch of an UPSERT
it returns whatever the most recent INSERT on the connection
yielded — possibly from an earlier transaction, possibly a row
that has since been deleted. The Go driver
(`modernc.org/sqlite`) exposes this faithfully. When the cached
value pointed at a now-deleted message id, the FK link in
`message_mailboxes` blew up; when it pointed at a still-living
but wrong message id, the link silently associated the wrong
message with the folder (the worse failure mode — green test
suite, corrupt junction table).

## Decision

For any UPSERT whose rowid is read downstream, use the
`RETURNING id` clause and `tx.QueryRow(...).Scan(&id)` instead
of `res.Exec(...).LastInsertId()`. `RETURNING` is SQLite 3.35+
and works for both the INSERT and UPDATE branches of an
`ON CONFLICT DO UPDATE`. `LastInsertId()` remains correct on
plain INSERTs (the `outbox` ops paths in `ops.go`), so it is
not blanket-banned — only on UPSERTs that read the rowid.

The fix applies to `upsertMessages`. Sibling UPSERTs in
`internal/cache/` either don't read the rowid (`addressbooks`,
`contacts`, `bodies`, `folders` in syncer) or use the conflict
column itself as the downstream key (`drafts.draft_id`,
`attachments` keyed by `(message, part_id)` is read via the
`UNIQUE` lookup). No other call site needs the change.

## Consequences

- The FK error at startup is gone; sync re-runs of existing
  messages no longer link phantom rows in `message_mailboxes`.
- The `if id == 0 { fallback SELECT }` workaround disappears
  with the rewrite — `RETURNING id` always returns the affected
  row, so the dead branch is removed rather than preserved as a
  scar.
- Pre-existing junction-table corruption (if any) from past runs
  is not retroactively cleaned. The `message_mailboxes` PRIMARY
  KEY `(message, folder)` plus `INSERT OR IGNORE` means
  duplicate-wrong links would be ignored on re-sync rather than
  re-corrupted, but a row pointing at a deleted message id would
  survive. A future migration scrub is queued in BACKLOG if
  evidence of orphans surfaces.
- Future cache work: any new UPSERT in `internal/cache/` whose
  rowid is read downstream must use `RETURNING id`. The
  go-conventions skill's modern-stdlib idiom table is the right
  home for the rule; added inline at the next pass that touches
  the skill.
- Audit G's read-only walk did not surface this; it was an
  inline-find while testing the Audit G branch on a live cache.
  Pre-beta policy ("fix it inline as part of the current pass")
  applies. This ADR rides Pass 40 alongside ADR-0230 (the audit
  proper).
