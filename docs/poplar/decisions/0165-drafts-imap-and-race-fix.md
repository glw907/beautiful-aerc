---
title: IMAP PushDraft, race-immune drafts, draft-superseded banner
status: accepted
date: 2026-05-06
---

## Context

ADR-0164 introduced JMAP-only drafts persistence behind an `IsJMAP()`
gate. IMAP accounts had no draft persistence, the gate scattered
through the compose lifecycle, and #39 surfaced a race in the
discard path: `cache.UpsertDraft` from an in-flight autosave Cmd
could re-create a row right after `DeleteDraft` ran, leaving an
orphan local draft. We also lacked a UI signal for the case where
PushDraft succeeded server-side but the local row was gone.

## Decision

**IMAP PushDraft.** `mail.Backend.PushDraft` is no longer
backend-conditional. The IMAP impl (`internal/mailimap/smtp.go`)
runs `APPEND \Draft` on the cmd connection and reads the APPENDUID
from `imapclient.AppendCommand.Wait()`. UIDPLUS is asserted at
Connect, so the UID is always present. When `prevUID` is non-empty,
the prior server image is best-effort expunged via `SELECT folder` →
`UID STORE +FLAGS.SILENT \Deleted` → `UID EXPUNGE prevUID`. APPEND
failure aborts; an EXPUNGE failure orphans the prior image but the
new draft is good — symmetric to JMAP's destroy-best-effort policy.
The `imapClient.Append` interface widens to return `(mail.UID,
error)`; `Backend.Append` (cache outbox path) discards the UID.

**Gate lift.** The four `m.acct.Backend().IsJMAP()` branches in
`internal/ui/app.go` are gone. The Save? modal, Drafts-folder
Enter routing, and `c`-key compose-open path are all
backend-agnostic. The `pendingComposeDiscard` field and the legacy
"Discard draft?" modal are deleted.

**Race-immune drafts.** `cache.UpsertDraft` is split into
`CreateDraft` (insert-or-update on Open) and `UpdateDraft`
(UPDATE-only; 0 rows = no-op). Compose `Init` calls Create once;
the autosave timer calls Update. After `DeleteDraft`, any
in-flight Update is a 0-row no-op rather than resurrecting the
row. Ticker-cancellation was rejected — the in-flight `tea.Cmd`
already captured state, so a model-side flag cannot reach it.

**Superseded banner.** `cache.ErrDraftSuperseded` is the typed
sentinel `MarkDraftPushed` returns when 0 rows match. The drainer
treats it as terminal success (the server push completed) but
publishes a `CacheEvent` with `Note = "draft superseded by another
client"`. The App routes any non-empty `CacheEvent.Note` into the
error-banner row.

## Consequences

- IMAP and JMAP compose lifecycles are unified. Drafts behave
  identically across backends.
- Schema v7 stays. No migration.
- The ADR-0164 row in `docs/poplar/invariants.md` is rewritten:
  "Drafts persistence" rather than "Drafts persistence (JMAP)".
- `cache.CacheEvent` gains a `Note` field for advisory banners.
  Currently used only by the superseded path; future advisory
  events (post-1.0 sync hints, etc.) will reuse it.
- BACKLOG #39 closes.
