# Pass 8.10 — First-sync header population

## Problem

Fresh cache, Fastmail Inbox: `folders.exists_total = 62`, msglist
empty. Trace: `cache.SyncFolder` reads `sync_token = NULL`, calls
`Backend.Changes(folder, since=nil)`. JMAP impl issues
`Email/changes(sinceState="")` which Fastmail answers with empty
Created/Updated/Destroyed and a fresh state — there is no notion of
"all current emails" in `Email/changes`. The cache writes the new
sync_token and applies a zero-length delta, so `messages` stays
empty.

IMAP scan-and-diff is unaffected: `decodeIMAPToken(nil) == 0`, every
UID > 0 becomes Added. The bug is JMAP-only.

## Decision

Initial baseline pull lives inside `mailjmap.Backend.Changes` on the
`since == nil` branch. The `mail.ChangeTracker` contract is unchanged
— callers see the same "first call returns the world as Added" shape
the IMAP impl already provides.

### JMAP baseline path

1. Resolve `folder → mailbox id` from `b.folders` (re-using existing
   map). Unknown folder → wrapped error.
2. Page `Email/query` with `Filter.InMailbox = id`, sorted
   `receivedAt desc` (matches `QueryFolder` for visual consistency on
   first paint), `CalculateTotal = true`, `Limit = 500`. Loop on
   `Position += len(IDs)` until `len(IDs) == 0` or
   `Position + len(IDs) >= Total`. Accumulate IDs into `Added`.
3. After the final page, capture the current Email-type state by
   issuing `Email/get` with `IDs: nil` — its `state` field is the
   sync token to return. Done in a separate request so a server that
   coalesces request invocations doesn't race the query.
4. Return `(ChangeSet{Added: ids}, SyncToken(state), nil)`.

### Header chunking

`Backend.FetchHeaders` currently issues one `Email/get` for every
UID it's handed. With 500-id pages from baseline pull (or larger
incremental change windows), one request can exceed
`maxObjectsInGet` (Fastmail: 4096, conservative cap). Chunk inside
`FetchHeaders` with `headerFetchChunk = 500`, identical translation
loop per chunk, append into one `[]MessageInfo`. Existing single-
request tests still pass because chunking only kicks in at >500 IDs.

### IMAP

Untouched. Verified by inspection: `decodeIMAPToken(nil) == 0`.

## Open questions — settled inline

These were marked "still open" in STATUS, but each has a default
that flows from existing patterns; reserving brainstorm bandwidth
for real tradeoffs.

- **Page size — 500.** Fastmail's `Email/query` allows up to 4096
  but typical sweet spot is a few hundred; 500 keeps per-page latency
  bounded while needing only one page for any inbox under that size
  (covers the common case in one round-trip). Same constant for the
  follow-up `FetchHeaders` chunk so paging is symmetric.
- **Loading state — none for now.** Existing `pumpUpdatesCmd` runs
  the sync inside a `tea.Cmd`; the msglist already shows "no
  messages" until `RefreshSource` re-reads. A spinner is a separate
  UX pass and out of scope for a bug fix.
- **Interrupted-pull recovery — implicit.** `applyDelta` upserts on
  `protocol_id`, and `writeSyncToken` only fires after a clean
  return. An interrupted pull leaves partial messages but no token,
  so the next call replays from scratch. Idempotent upsert means no
  duplicates.

## Files

- `internal/mailjmap/changes.go` — branch on `since == nil`, new
  helper `(b *Backend) baselinePull(ctx, folder)`.
- `internal/mailjmap/jmap.go` — `FetchHeaders` chunking loop.
- `internal/mailjmap/changes_test.go` (new) — table-driven cover for
  the baseline path: empty mailbox, single page, multi-page, state
  capture.
- `internal/mailjmap/jmap_test.go` — extend `FetchHeaders` cover
  with a >chunk-size case.

## Verification

- `make check`.
- Live: `rm -rf ~/.cache/poplar/`, `poplar`, open Inbox, confirm 62
  rows render (subjects + dates), verify cache via
  `poplar cache stats`.

## Pass-end ritual

ADR-0143 (baseline-pull contract). Invariants edit: replace the
"JMAP Email/changes since=nil returns a fresh state and no
baseline" gap with the new contract. STATUS: mark 8.10 done, draft
Pass 9 starter prompt. Archive plan. Commit, push, install.
