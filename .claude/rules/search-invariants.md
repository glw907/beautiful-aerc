---
description: Search-layer binding facts for poplar's FTS5 + sidebar shelf
paths:
  - "internal/search/**/*.go"
  - "internal/cache/search.go"
  - "internal/cache/fts.go"
  - "internal/ui/sidebar/search.go"
  - "internal/ui/messagelist/model.go"
  - "docs/superpowers/plans/**/*.md"
  - "docs/superpowers/specs/**/*.md"
---

# Poplar Search Invariants

Binding facts for the search surface: FTS5 substrate, query
parser, cache search query, sidebar shelf, results-mode
messagelist. Loaded when editing search-adjacent files or
planning passes that touch search. Codified in ADR-0188.

## Parser + plaintext

- `internal/search/Parse(input) Query` parses the query
  language: `from:`/`to:`/`cc:`/`subject:`/`in:`/`has:attachment`
  operators, quoted phrases, bare terms. Unknown `key:value`
  falls through as a bare term so operator typos don't silently
  shrink results. Pure, no I/O.
- `content.ExtractPlainText(mime) (string, error)` is the shared
  MIME→plaintext extractor. Cache calls it from `storeBody` to
  populate the FTS body column; UI's `walkBody` delegates the
  text portion (calendar/unsubscribe walking stays in the UI).

## FTS5 substrate

- Schema v11 adds `messages_fts(subject, from_addr, to_addr,
  cc_addr, body)` as a regular FTS5 virtual table with
  rowid = `messages.id`. Header columns backfill from existing
  `messages` rows in the migration; body columns populate as
  `storeBody` runs. Writes go through `internal/cache/fts.go`
  helpers (`writeFTSHeadersTx`, `writeFTSBodyTx`) inside the
  caller's transaction. FTS5 lacks UPSERT, so both helpers
  DELETE+INSERT.
- Contentless FTS5 was rejected (delete-via-INSERT semantics
  too awkward); the storage cost of a regular table is small
  and acceptable.

## Cache search

- `(*Account).Search(ctx, q search.Query, scope SearchScope,
  limit int) ([]SearchHit, error)` runs against `messages_fts`,
  applies scope + `in:` + `has:attachment` as SQL filters, sorts
  `sent_at DESC`. Bare terms span `{subject from_addr to_addr
  body}` via FTS5's column-set syntax; each user term wraps as
  a quoted phrase so syntax characters never trip the parser.
- `SearchHit` pairs `mail.MessageInfo` with the origin folder
  name. Folder is populated only in cross-folder scope; folder-
  scope hits leave it empty.
- HasAttachment-only / In-only queries use a permissive prefix
  wildcard for the MATCH expression (`body:* OR …`); the SQL
  filters carry the real constraint. FTS5 requires a non-empty
  MATCH; this is the cheapest workable shape. Empty parsed
  Query short-circuits before SQL fires.

## Backfill progress + warn

- `BackfillProgress() (done, total int, warn bool, err error)`.
  `warn = true` while the Backfiller is sleeping in throttle
  backoff; cleared on the next non-throttle outcome.
  `App.refreshBackfillSegment` passes warn through to
  `SetBackfill(done, total, paused, warn)` so the status-bar
  `↓ ⚠` substate reflects real backend back-pressure.

## Sidebar shelf + UI routing

- `\` toggles `ScopeFolder` ↔ `ScopeAll`. Tab is retired from
  the shelf input. Shelf info-row badge: `[folder]` /
  `[all folders]`. `SearchUpdatedMsg.Scope` is
  `uicore.SearchScope`.
- Folder-scope routes through `messagelist.SetFilter(query)` —
  in-memory match on subject + from + to + cc for bare terms,
  honoring From/To/Cc/Subject operators in-memory.
  HasAttachment + In are no-ops folder-locally (those rows
  lack the data; cross-folder via cache.Search covers them).
- Cross-folder routes through `runSearchCmd` → `cache.Search`
  → `searchResultsMsg` → `messagelist.SetSearchResults(msgs,
  originByUID)`. Results mode disables threading and prefixes
  the sender column with `[<Folder>] ` via `senderWithOrigin`.
- Esc clears via `clearSearchIfActive`, which calls
  `ClearSearchResults` (when in results mode) then
  `ClearFilter`. The shelf returns to `SearchIdle`.

## Sort + display rationale

- Sort: `sent_at DESC`, no toggle. Matches Geary, K-9, aerc,
  Apple Mail, Outlook, Fastmail. Geary precedent (hardcoded,
  no-config) is the closest analog to poplar's stance.
  Thunderbird and Gmail (since 2025) default to relevance;
  outliers in the matrix.
- Origin folder display: `[Folder] ` prefix in the sender
  column. Matches Geary badge / Gmail label / Fastmail prefix —
  the genre standard for compact layouts. Survives the Spartan
  tier (80 cells) by truncating the sender within the column
  budget.
