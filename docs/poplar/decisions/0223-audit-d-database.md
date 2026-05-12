---
title: Audit D — database
status: accepted
date: 2026-05-11
---

## Context

Pass 37 ran Audit D against the seven `docs/poplar/audit-plan.md`
§"Phase D" focuses: schema migration ladder; transactional
boundaries; FTS5 consistency; schema-version probe + refusal;
UIDVALIDITY re-key + IMAP scan-and-diff; file-on-disk shape;
drainer conflict matrix state side. Trigger: Audit C remediation
(Pass 36.1, ADR-0222) shipped.

The walk surface and per-focus findings live in
`docs/superpowers/archive/plans/2026-05-11-audit-d.md`.

## Decision

Four findings. None reach P0 — no inline remediation this pass.
Pass 37.1 lands before Audit Final (Pass 38).

- **F1 — Orphan `message_recipients` on message delete (P1).**
  The v8 schema declares `message_recipients.message_uid INTEGER
  NOT NULL` with no FK to `messages`. `DELETE FROM messages`
  (drainer `finalizeSuccess` for `DestroyArgs`; syncer
  `applyDelta` for `Removed`) cascades to bodies, attachments,
  and `message_mailboxes` but leaves recipient rows behind.
  `SuggestAddresses` reads `message_recipients` without joining
  `messages`, so deleted-message recipients keep contributing to
  recency-decayed autocomplete ranking. Visible bias. Fix is a
  v13 migration adding `REFERENCES messages(id) ON DELETE CASCADE`
  (defense in depth) or explicit cleanup at both delete sites.
- **F2 — Orphan `messages_fts` rows on message delete (P2).**
  Same root cause: virtual-table rows don't participate in FK
  cascade. `Search` uses `JOIN messages m ON m.id = fts.rowid`,
  so orphans never surface as false hits — the cost is storage
  only. Noted, not queued (P2).
- **F3 — v11 FTS body backfill gap (P1).** `migrateV11` inserts
  one `messages_fts` row per existing message with
  `body = ''`. The Backfiller populates the body column only via
  `storeBody`, which fires on `FetchBody` cache misses
  (`WHERE b.bytes IS NULL`). Users with cached bodies pre-v11
  silently lose body-search coverage for those messages until
  the body is evicted and re-fetched. Fix is to extend
  `migrateV11` to run `content.ExtractPlainText` on existing
  `bodies.bytes` rows and populate the body column in the same
  tx.
- **F4 — IMAP UIDVALIDITY change goes undetected (P1).**
  `mailimap/changes.go` documents UIDVALIDITY re-anchor and
  reserves `SyncToken` bytes 0–3 for `uidvalidity`, but never
  reads them; `mail.Folder` has no `UIDValidity` field. After a
  server-side UIDVALIDITY change the cache silently diverges —
  new UIDs reset to 1 and the `n > prevMax` filter excludes
  every row, leaving the cache anchored to stale UIDs that no
  longer resolve server-side. Fix spans `internal/mail/`
  (`mail.Folder.UIDValidity uint32`), `internal/mailimap/folders.go`
  (capture from `SELECT`/`EXAMINE`), and
  `internal/mailimap/changes.go` (pack into token bytes 0–3,
  compare, return `mail.ErrCannotCalculateChanges` on mismatch).
  No production user is on IMAP yet (Fastmail/JMAP is the only
  account on disk); the divergence is real but currently
  hypothetical. Classified P1.

## Consequences

- Pass 37.1 lands F1, F3, F4 before Audit Final. STATUS records
  `Audit D 2026-05-11 → 3 P1 + 1 P2 findings, queued for Pass
  37.1`.
- F1's CASCADE fix carries a schema bump (v13). Pre-beta endorses
  schema work; v12 wasn't long ago and the migration shape is
  established.
- F3's fix runs once on upgrade and may extract plaintext for
  thousands of cached bodies. Migration tx will hold the write
  lock for the duration; acceptable for a one-shot upgrade. If
  the lock-time cost surfaces in practice, the alternative is a
  post-migration Backfiller re-pass that loops over
  `(bodies JOIN messages_fts WHERE messages_fts.body = '')`.
- F4's plumbing widens `mail.Folder` and touches three packages.
  The fix mirrors the JMAP path's existing
  `mail.ErrCannotCalculateChanges` semantics — no new sentinel.
- F2 is left in place. If body-deletion volume ever becomes high
  enough that orphan FTS rows measurably bloat the database,
  Pass 37.1 can absorb the explicit `DELETE FROM messages_fts`
  for free alongside F1.
- The audit-plan §"Phase D" focus list survives unchanged; the
  walk surfaced real findings, so the focus mechanics are
  working.
