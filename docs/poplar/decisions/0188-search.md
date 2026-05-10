---
title: Search via SQLite FTS5 with cross-folder scope toggle
status: accepted
date: 2026-05-09
---

## Context

BACKLOG #38 asked for cross-folder search reading from the local
body cache. Pass 13 had landed the body-sync substrate
(`Backfiller`, `↓ N/M` status segment). This pass turns that
substrate into a usable search surface.

The matrix survey
(`docs/poplar/research/2026-05-09-mail-client-search-survey.md`)
split cleanly. GUI clients ship local FTS indexes (Thunderbird
gloda, Apple Spotlight, Camel, Geary FTS5). TUI clients with body
search delegate to IMAP SEARCH (mutt, aerc); none indexes locally.
Operator strings are everywhere on the web (Gmail, Fastmail) and
in Outlook; GUI desktop apps prefer token builders; TUI ancestors
use command paradigms (`~f`, `-f`, `:filter`). No surveyed TUI
offers cross-folder scope-switching.

Settled in brainstorm before the pass: SQLite FTS5 (no Bleve, no
external index); operator set per the matrix; sidebar shelf as
the entry point; backfill is the substrate (Pass 13.1 reads,
doesn't write the population worker).

The pass's open questions resolved during planning:

- **Sort.** Matrix is overwhelmingly date-desc — Geary, K-9, aerc,
  Apple Mail, Outlook, Fastmail all default this way. Geary is
  hardcoded date-desc with no toggle, the cleanest match for
  poplar's no-config stance. Thunderbird and Gmail (since 2025)
  default to relevance, but those are outliers; mail is
  time-dominant and 1-3 word queries don't benefit from BM25
  ranking. Settled: `sent_at DESC`, no toggle.
- **Origin folder display.** Geary/Gmail/Fastmail all use a
  badge/tag/prefix on each row; only Thunderbird and Outlook
  ship a dedicated column, and both run in much wider GUIs than
  poplar's Spartan tier (80 cells). Settled: `[Folder] sender`
  prefix in the sender column, matching the genre standard.
- **Scope toggle.** Operators (`from:`, `subject:`, `in:`)
  coexist with scope toggles in Outlook, Evolution, Gmail, and
  Fastmail; they're orthogonal axes. Settled: `\` toggles scope
  (folder ↔ all folders); operators subsume the prior
  `[name]/[all]` field selector; Tab retires from the shelf.
  Shelf badge becomes `[folder]` / `[all folders]`.
- **FTS5 schema shape.** Considered contentless FTS5 with
  custom delete-via-INSERT semantics versus a regular stored-
  content table. Contentless's awkward DELETE made the regular
  table the simpler choice; the storage cost is small (header
  text + plaintext body, deduplicated against the inverted
  index). Settled: `messages_fts(subject, from_addr, to_addr,
  cc_addr, body)` regular FTS5 table, schema v11.
- **Index-update transaction boundaries.** Triggers can't
  extract MIME plaintext; the body column has to come from Go.
  Settled: writes happen inside the existing `upsertMessages`
  and `storeBody` transactions via `internal/cache/fts.go`
  helpers.
- **Throttle warn wiring.** The warn input was already plumbed
  into `SetBackfill(done, total, paused, warn)` by Pass 13 with
  a hardcoded `false`. Pass 13.1 surfaces the real value via an
  atomic flag on Backfiller and an extended `BackfillProgress`
  return.

## Decision

`internal/search/parser.go` parses the query language —
`from:`/`to:`/`cc:`/`subject:`/`in:`/`has:attachment` operators,
quoted phrases, bare terms. Bare terms span subject + body +
from + to per the survey synthesis. Unknown `key:value` falls
through as a bare term so operator typos don't silently shrink
results.

Schema v11 adds `messages_fts(subject, from_addr, to_addr,
cc_addr, body)` as a regular FTS5 virtual table with rowid =
`messages.id`. The migration backfills header columns from
existing `messages` rows; bodies populate later as `storeBody`
fires. `internal/cache/fts.go` carries `writeFTSHeadersTx` and
`writeFTSBodyTx`; both DELETE+INSERT inside the caller's
transaction (FTS5 has no UPSERT and column-level UPDATE doesn't
work cleanly across contentless/regular tables, so we use the
same pattern for both).

`(*Account).Search(ctx, q search.Query, scope SearchScope, limit
int) ([]SearchHit, error)` builds an FTS5 MATCH expression from
the parsed query, applies scope/operator constraints as SQL
filters, and sorts `sent_at DESC`. Bare terms use FTS5's
`{subject from_addr to_addr body}: "phrase"` column-set syntax;
each user term wraps as a quoted phrase so syntax characters
don't trip the parser. SearchHit pairs `mail.MessageInfo` with
the origin folder name (cross-folder scope only).

The sidebar search shelf retires the Tab `[name]/[all]` mode
toggle. `\` becomes the sole modal axis: folder-local filter
(in-memory hide-and-filter via `messagelist.SetFilter`) ↔
cross-folder results (cache.Search → `searchResultsMsg` →
`messagelist.SetSearchResults`). The shelf info row badge
shows `[folder]` / `[all folders]`. `SearchUpdatedMsg.Mode`
renames to `Scope` of type `uicore.SearchScope`.

`messagelist.Model` gains `SetSearchResults(msgs, originByUID)`
and `ClearSearchResults`. Results mode disables threading and
prefixes `[<Folder>] ` to the sender column via a renderer
helper `senderWithOrigin`. Esc routes through the existing
`clearSearchIfActive` path, which now calls `ClearSearchResults`
when results mode is active before falling through to
`ClearFilter`.

`Backfiller.throttling atomic.Bool` is set when entering the
shared `internal/backoff.Exponential` sleep on a throttle
error and cleared on the next successful fetch or non-throttle
exit. `BackfillProgress` returns `(done, total int, warn bool,
err error)`; `App.refreshBackfillSegment` passes the warn flag
through to `SetBackfill` instead of the prior hardcoded `false`.

`content.ExtractPlainText(mime []byte) (string, error)` factors
out the MIME→plaintext path that lived inside `account/cmds.go`'s
`walkBody`. The cache layer calls it from `storeBody`; the UI
calendar/unsubscribe walk delegates the text portion. No import
cycle: `internal/content/` and `internal/cache/` don't reference
each other.

## Consequences

ADR-0187 stays current; this pass extends `BackfillProgress`'s
return shape. The status-bar warn substate now reflects real
throttle state.

Folder-local search gains operator support without changing the
in-memory hide-filter shape. `from:alice` typed in Inbox filters
the in-memory rows by the parsed query's From clause.
HasAttachment and `in:` operators are no-ops in folder-local
mode (those rows lack the attachment data); cross-folder scope
honors them via the cache SQL filters.

Cross-folder threading is suppressed in results mode. A thread
spanning Inbox + Archive can't render coherently with origin
badges per row — the user sees individual messages, not threads.
The folder-local path keeps full threading.

The MATCH expression for HasAttachment-only queries uses a
permissive prefix wildcard (`body:* OR subject:* OR …`). FTS5
requires a non-empty MATCH; this is the cheapest workable shape
when the SQL filter carries the constraint. Empty parsed Query
short-circuits before the SQL fires.

Storage: FTS5 stored-content tables duplicate header text and
plaintext body alongside the inverted index. For a 33,000-
message account that's a few hundred MB of plaintext duplicated
into FTS storage. Acceptable; users opting into a body-cache
cap (`[cache] max-size`) can still bound total disk use.

The pre-13.1 `[name]/[all]` Tab cycle is retired. Tests that
asserted date-text matching under `[all]` are removed; a new
test covers operator scoping in folder-local mode. `Tab` is now
free for future use in the shelf (currently unhandled).

Idiomatic-bubbletea check (§10): no state mutation in `View()`;
all I/O lives inside `runSearchCmd`'s `tea.Cmd`. Width math uses
`lipgloss.Width` and the existing `truncateCells`/`padRight`
helpers. `\` declared as a `key.Binding` and dispatched via
`key.Matches`. No deprecated APIs. Conformant.
