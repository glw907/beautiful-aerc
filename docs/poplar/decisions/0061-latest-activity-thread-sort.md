---
title: Threads sort by latest activity; children always ascending
status: accepted
date: 2026-04-13  # Pass 2.5b-3.6
---

## Context

A threaded view has two orthogonal sort orders: how to order
threads against each other, and how to order replies within a
thread. The two don't have to match — and shouldn't.

For inter-thread order, the question is whether to sort by the
thread's root date or by its most recent activity. Sorting by
root date means a long-running conversation falls down the list
even when fresh replies arrive. Sorting by latest activity keeps
active threads at the top, matching Gmail, Apple Mail, and
Fastmail web.

For intra-thread order, the question is whether replies should
mirror the folder direction (newest-first reverses inside the
thread too) or always read top-to-bottom oldest-to-newest.
Mirroring breaks the conversation into reverse-chronological
chunks that are hard to read; ascending order matches every
client that does threading well.

## Decision

`MessageList.SortOrder` is the inter-thread direction
(`SortDateDesc` default, `SortDateAsc` opt-in via
`[ui.folders.<name>] sort = "date-asc"`). `rebuild` sorts buckets
by `latestActivity(bucket)` — the maximum date string across all
messages in the thread — in the configured direction.

Children inside a thread are always sorted ascending by date,
regardless of the folder's sort direction.

## Consequences

- A thread with a recent reply jumps to the top of a date-desc
  folder, even if the original message is months old. This is
  the expected web-mail behavior.
- Folder-level sort config still takes effect on the user's terms
  (newest-first vs. oldest-first), but the conversational reading
  order inside each thread stays stable.
- Sort comparisons are lexicographic on the wire `Date string`
  field until Pass 3 introduces real `time.Time` on
  `MessageInfo`. Mock data uses ISO-like strings in tests so the
  lex order matches chronology end-to-end.
- The `latestActivity` helper is called once per bucket per
  comparison inside `sort.SliceStable`'s comparator — minimal
  cost at realistic thread counts.
