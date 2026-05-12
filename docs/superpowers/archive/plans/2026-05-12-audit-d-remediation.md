# Pass 37.1 — Audit D remediation

Land the three P1 findings from ADR-0223 (Audit D) before Audit
Final. F2 is a P2 storage-only orphan; the v13 rebuild for F1
absorbs an explicit `DELETE FROM messages_fts` for the orphan FTS
rows in the same migration step (free, since v13 already scans the
relevant ids).

## F1 — Orphan `message_recipients` (P1)

Root cause: v8 declared `message_recipients.message_uid INTEGER
NOT NULL` with no FK to `messages(id)`. The column stores
`messages.id` (autoincrement PK, not the protocol UID — the name
is historical). Deletes from `messages` leave recipient rows in
place, biasing `SuggestAddresses`.

Fix: schema v13 rebuilds the table with `REFERENCES messages(id)
ON DELETE CASCADE`. SQLite has no `ALTER COLUMN`, so the
migration mirrors v9's outbox rebuild:

1. `CREATE TABLE message_recipients_new (… message_uid INTEGER
   NOT NULL REFERENCES messages(id) ON DELETE CASCADE, …, PRIMARY
   KEY (message_uid, role, address))`.
2. `INSERT INTO message_recipients_new SELECT mr.* FROM
   message_recipients mr JOIN messages m ON m.id = mr.message_uid`
   (the join drops pre-existing orphans — defensible cleanup as
   part of the FK introduction).
3. Drop the old table; rename `_new` to `message_recipients`.
4. Recreate `message_recipients_by_addr_sent` index.
5. Absorb F2: `DELETE FROM messages_fts WHERE rowid NOT IN
   (SELECT id FROM messages)`.

`writeRecipientsTx` is unchanged — the migration only narrows the
schema; the runtime insert path already keys on `messages.id`.

## F3 — v11 FTS body backfill gap (P1)

`migrateV11` inserts an `messages_fts` row per message with
`body = ''`. The fix extends the migration to loop existing
`bodies.bytes` rows in the same transaction, run
`content.ExtractPlainText`, and call `writeFTSBodyTx` for each.

Implementation: after the header backfill block, iterate
`SELECT message, bytes FROM bodies` inside the migration tx,
extract plaintext, and call `writeFTSBodyTx(ctx, tx, msgID,
body)`. `writeFTSBodyTx` is already tx-scoped and safe to call
during migration (its missing-row branch covers the case where
header backfill hasn't yet inserted — but for a fresh upgrade,
the header backfill INSERT runs first in the same migration, so
the row exists). Pass `context.Background()` — the migration
runner doesn't thread a ctx.

Extraction errors on a single body are skipped (log via slog
debug, not fatal) — a malformed cached body shouldn't block the
upgrade. The migration tx holds the write lock for the duration;
acceptable for a one-shot upgrade. If a future user reports
lock-time pain, the fallback is the post-migration Backfiller
re-pass noted in ADR-0223.

## F4 — IMAP UIDVALIDITY undetected (P1)

`mailimap/changes.go` documents the UIDVALIDITY contract but
never reads `since[0:4]`. Three coordinated edits:

- `internal/mail/types.go`: add `UIDValidity uint32` to
  `mail.Folder`. JMAP leaves it zero; only IMAP populates it.
- `internal/mailimap/realclient.go`: `Select` reads
  `data.UIDValidity` from `imap.SelectData` and puts it on the
  returned `mail.Folder`.
- `internal/mailimap/folders.go`: `OpenFolder` doesn't return the
  Folder today, so add a small lookup — `cmd.Select` returns
  `mail.Folder`, but we currently ignore it. Capture `uidvalidity`
  on the Backend so `Changes` can read it without re-selecting.
  Specifically: `Backend` gains a `currentUIDValidity uint32`
  field, set in `OpenFolder` from the Select result. `Changes`
  then compares `decodeIMAPToken` bytes 0–3 against this value.

`encodeIMAPToken` / `decodeIMAPToken` widen to return `(uidvalidity,
maxuid)`. `Changes` semantics:

- If `since` is empty (nil/zero) → initial sync; no mismatch
  check; encode `(currentUIDValidity, newMax)`.
- If `prevUIDValidity != 0 && prevUIDValidity !=
  currentUIDValidity` → return `mail.ErrCannotCalculateChanges`.
- Else proceed; encode `(currentUIDValidity, newMax)` on return.

JMAP backend untouched.

## Testing

- `cache_test.go`: TestV13CascadesRecipientRows — open v12,
  insert a message + recipient rows, migrate, delete the message,
  assert recipient rows gone (covers FK runtime), plus a
  pre-migration orphan scenario asserts the migration scrub
  drops orphans.
- `fts_test.go` / migration test: TestV11BackfillsCachedBodies —
  open db at v10, insert a message + body bytes, migrate to v11,
  assert `messages_fts.body` non-empty and FTS5 MATCH finds a
  body-only term.
- `mailimap/changes_test.go`: TestChangesUIDValidityMismatch —
  fake client returns differing uidvalidity on Select; assert
  `mail.ErrCannotCalculateChanges`. Plus a roundtrip test that
  encoded tokens preserve both fields.

## Out of scope

- Live Gmail/Outlook OAuth verification (Pass 35.1; awaits creds).
- CONDSTORE/VANISHED in IMAP Changes.
- Backfiller re-pass fallback for F3 (only if lock-time pain
  surfaces).
