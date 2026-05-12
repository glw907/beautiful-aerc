# Poplar Status

**Current pass:** Pass 37.1 — Audit D remediation. Pass 37 closed
ADR-0223: Audit D returns 3 P1 + 1 P2, 0 P0. Orphan
`message_recipients` bias `SuggestAddresses`; `migrateV11`
backfills FTS headers but not bodies; `mailimap.Changes` never
reads UIDVALIDITY despite docstring claim. F2 (orphan
`messages_fts` rows) is P2 storage-only, noted only. Live Gmail +
Outlook verification (Pass 35.1) still queued — no OAuth client
IDs on hand.

**Beta soak deferred.** Pre-beta rules apply.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 32 | Scaffold through v2 declarative chrome | done |
| 33 | Mouse — reader (ADR-0218) | done |
| 34 | Mouse — sidebar + cross-pane (ADR-0219) | done |
| 35 | Native OAuth final wiring (ADR-0220) | done |
| 35.1 | Live Gmail + Outlook OAuth verification | pending creds |
| 36 | Audit C — feature surface (ADR-0221) | done |
| 36.1 | Audit C remediation (ADR-0222) | done |
| 37 | Audit D — database (ADR-0223) | done |
| 37.1 | **Audit D remediation** — F1/F3/F4 | next |
| 38 | Audit Final — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 37.1)

> **Goal.** Land the three P1 findings from Audit D (ADR-0223).
>
> **Scope.** F1 — Schema v13 rebuilds `message_recipients` with
> `REFERENCES messages(id) ON DELETE CASCADE` (SQLite has no
> `ALTER COLUMN`; mirror v9's outbox rebuild). F3 — extend
> `migrateV11` to loop `bodies.bytes` rows, run
> `content.ExtractPlainText`, populate `messages_fts.body` via
> `writeFTSBodyTx` in the same tx. F4 — add `UIDValidity uint32`
> to `mail.Folder`; `mailimap` captures from `SELECT`/`EXAMINE`,
> packs into `SyncToken` bytes 0–3 via existing encode/decode,
> compares in `Changes`, returns `mail.ErrCannotCalculateChanges`
> on mismatch. JMAP unchanged.
>
> **Settled:** Pre-beta endorses schema work + cross-package
> widening. F2 stays noted-only; absorb explicit `DELETE FROM
> messages_fts` alongside F1 only if free.
>
> **Still open — brainstorm:** None. Direct implementation.
>
> **Approach.** Plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-audit-d-remediation.md`,
> ADR-0224 records remediation. Standard pass-end checklist.
