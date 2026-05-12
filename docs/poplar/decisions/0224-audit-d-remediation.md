---
title: Audit D remediation — F1, F3, F4
status: accepted
date: 2026-05-12
---

## Context

Pass 37 (ADR-0223) surfaced three P1 findings on the database
surface: an orphan-row bias on `message_recipients` after message
deletes (F1), an FTS body backfill gap in `migrateV11` (F3), and
an unwired UIDVALIDITY check in IMAP `Changes` (F4). F2 was P2
storage-only (orphan `messages_fts` rows from historical deletes).
Pre-beta posture endorses schema work and cross-package widening,
so the three lands in one pass before Audit Final.

## Decision

- **F1.** Schema v13 rebuilds `message_recipients` with
  `REFERENCES messages(id) ON DELETE CASCADE`. SQLite has no
  `ALTER COLUMN`, so the migration mirrors v9's outbox rebuild:
  create `_new`, copy via `JOIN messages` so pre-existing orphans
  are dropped, swap, recreate the
  `message_recipients_by_addr_sent` index. The `message_uid`
  column name is unchanged (storing `messages.id` — the name is
  historical from v8 and not worth renaming through the FK
  boundary).
- **F2.** Absorbed for free in the v13 step: `DELETE FROM
  messages_fts WHERE rowid NOT IN (SELECT id FROM messages)`.
  FTS5 virtual tables don't participate in FK cascade, so this
  scrub plus the runtime `messages_fts` cleanup at delete sites
  (still TODO if it shows up; not in this pass) is the
  defense-in-depth posture.
- **F3.** `migrateV11` is extended with `backfillFTSBodies`:
  iterate `bodies.bytes`, run `content.ExtractPlainText`,
  call `writeFTSBodyTx` on each. Parse and write failures
  skip rather than abort — a corrupt body that was already on
  disk shouldn't trap users out of the new schema. Write skips
  log via `slog.Default().Debug`.
- **F4.** `mail.Folder.UIDValidity uint32` carries the RFC 3501
  value; JMAP backends leave it zero. `mailimap.realClient.Select`
  reads `imap.SelectData.UIDValidity` into the returned Folder.
  `Backend.OpenFolder` captures it under `b.mu` into
  `b.currentUIDVal`. `Changes` decodes `SyncToken` bytes 0–3 as
  the prior UIDVALIDITY, compares against `currentUIDVal`, and
  returns `mail.ErrCannotCalculateChanges` on mismatch
  (skipping the prior-zero initial-sync case). `encodeIMAPToken`
  widens to `(uidvalidity uint32, maxuid uint64)`;
  `decodeIMAPToken` returns the pair.

## Consequences

- Schema bumps to v13. The v13 migration runs once per account;
  body-bearing migrations hold the write lock for the duration,
  acceptable per ADR-0223.
- F1's FK boundary is defense in depth. Existing delete sites in
  `drainer.finalizeSuccess` and `syncer.applyDelta` already issue
  `DELETE FROM messages`; the FK now removes the explicit cleanup
  obligation those sites carried implicitly.
- Audit D returns clean (F2 storage-only is also scrubbed). Pass
  38 (Audit E) is unblocked.
- JMAP backend untouched. UIDVALIDITY plumbing is IMAP-only;
  JMAP returns `cannotCalculateChanges` natively per RFC 8621.
- The token layout (12 bytes BE: bytes 0–3 uidvalidity, bytes
  4–11 maxuid) was already reserved in the documented contract.
  This pass closes the gap between docstring and behavior.
