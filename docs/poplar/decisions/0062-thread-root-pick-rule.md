---
title: Thread root: empty InReplyTo, earliest-by-date fallback
status: accepted
date: 2026-04-13  # Pass 2.5b-3.6
---

## Context

To render a thread tree, `pickRoot` has to identify which
message in a thread bucket is the root — the message no other
reply in the bucket points to. Real mail data is messy: cross-
folder threads, deleted parents, broken `In-Reply-To` chains
where every message in the bucket references a UID outside the
bucket. The pick rule needs to produce a reasonable root in
every case rather than crashing or producing `nil`.

## Decision

`pickRoot(bucket)` returns the index within `bucket` of the
chosen root, using two rules in order:

1. **Preferred:** the first message with `InReplyTo == ""` —
   that's the conventional thread root.
2. **Fallback:** the earliest message by `Date` lexicographic
   comparison. Used when every message in the bucket references
   an external parent (broken chain).

Other top-level orphans (messages whose `InReplyTo` parent is
also missing from the bucket) attach to the chosen root as
depth-1 children inside `appendThreadRows`. The renderer treats
them as siblings of the root's real replies.

## Consequences

- Broken parent chains never crash. They render with the orphans
  visually attached to whichever message wound up as the synthetic
  root, which is occasionally wrong but always understandable.
- The Date fallback is lexicographic on wire-string format until
  Pass 3 introduces real `time.Time`. For prototype mock data
  (where all messages in a bucket share the same date string)
  it falls back to input order via `sort.SliceStable`.
- A future "real backend" pass that surfaces orphan groups
  separately (e.g., one synthetic root per orphan tree instead of
  flattening) will need to revisit `appendThreadRows` rather than
  `pickRoot`.
