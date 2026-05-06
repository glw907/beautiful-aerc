# Draft sync norms across email clients

Snapshot 2026-05-06. Used during Pass 9h.5 to pick a cadence and
storage model for poplar drafts persistence.

## Pattern summary

GUI clients with autosave: server-canonical, periodic upload,
APPEND-and-destroy-old. IMAP can't edit-in-place; JMAP `Email`
objects are immutable too — the destroy-old must be batched with
the create-new. Cadences vary from 10s to 5min. Last-write-wins
is universal — no surveyed client merges concurrent edits.

Terminal clients (mutt, aerc, alpine) are an outlier: postpone-
style, user-driven, no autosave at all.

## Per-client table

| Client          | Storage              | Cadence       | Push model                                   | Conflict        |
|-----------------|----------------------|---------------|----------------------------------------------|-----------------|
| Thunderbird     | Server Drafts        | 5min default  | APPEND new + STORE \Deleted on old           | Last-write-wins |
| Apple Mail      | Server Drafts        | ~30s          | APPEND + delete-old                          | Last-write-wins |
| Outlook (IMAP)  | Server Drafts        | ~3min         | APPEND + delete-old                          | Last-write-wins |
| Geary           | Server Drafts        | ~10s          | APPEND + delete-old                          | Last-write-wins |
| Evolution       | Local; server opt-in | ~60s          | APPEND when server-configured                | Last-write-wins |
| K-9 Mail        | Local then server    | On close only | APPEND + delete-old                          | n/a             |
| mutt            | $postponed mbox/IMAP | Manual        | None (^X postpone)                           | n/a             |
| aerc            | [postpone] folder    | Manual        | None                                         | n/a             |
| alpine          | postponed-msgs       | Manual (^O)   | None                                         | n/a             |
| Fastmail web    | Server Drafts        | ~real-time    | JMAP Email/import + destroy via creation-ref | Last-write-wins |
| Gmail web       | Server Drafts        | ~1–3s         | Gmail API patch (server-mutable)             | Last-write-wins |

## Poplar's choice

- Local SQLite as the 1s-debounce edit buffer (no UID churn on
  every keystroke, fast restore on Esc).
- Server push on close + 5min idle timer (Thunderbird's cadence).
- APPEND-and-destroy-old is the universal pattern; JMAP gives us
  atomicity via a single `Email/import` + `Email/set destroy`
  request. IMAP (Pass 9h.6) lands without atomicity — orphans
  possible on partial failure, mitigated by the cache pointer.
- Last-write-wins; no merge.

Reference: ADR-0164.
