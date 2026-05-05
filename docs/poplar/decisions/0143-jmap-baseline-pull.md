---
title: JMAP per-folder baseline pull on nil SyncToken
status: accepted
date: 2026-05-04
---

## Context

Fresh cache + Fastmail: opening Inbox left `messages` empty though
`folders.exists_total = 62`. The cache syncer reads `sync_token =
NULL`, calls `Backend.Changes(folder, since=nil)`. The JMAP impl
issued `Email/changes(sinceState="")`, which RFC 8621 §4.3 answers
with empty Created/Updated/Destroyed and a fresh state — there is
no "give me everything that exists now" mode in `Email/changes`.
The cache then wrote the new sync_token and applied a zero-length
delta, locking out the baseline forever.

IMAP scan-and-diff is unaffected: `decodeIMAPToken(nil) == 0`,
every UID > 0 becomes Added.

## Decision

The `since == nil` branch of `mailjmap.Backend.Changes` runs a
per-folder baseline pull instead of `Email/changes`:

1. Page `Email/query` filtered by `inMailbox` for the requested
   folder, sorted `receivedAt` desc, page size 500.
2. Piggyback an `Email/get` with a sentinel id
   (`stateProbeID`) on every page so the response carries the
   Email-type state. Single-page pulls (the common case) finish
   in one roundtrip; multi-page pulls overwrite state per page
   and the last page wins. The sentinel is required because
   `omitempty` drops a nil/empty IDs slice and Fastmail returns
   a state-less response when `ids` is missing.
3. Return `(ChangeSet{Added: ids}, SyncToken(state), nil)`.

`Backend.FetchHeaders` chunks at 500 ids so an `Email/get` from a
large baseline pull stays under typical `maxObjectsInGet` caps
(Fastmail allows 4096; matching the page size keeps paging
symmetric).

## Consequences

First open of any JMAP folder now populates `messages` in one or
two roundtrips. The `mail.ChangeTracker` contract is unchanged —
callers see the same "first call returns the world as Added" shape
the IMAP impl already provides. Future incremental syncs run
through the existing `Email/changes` path.

The sentinel-id workaround is the cleanest path absent JMAP wire
support for "state-only Email/get"; if a future go-jmap exposes a
`StateOnly` flag or omitempty changes, drop the sentinel.
