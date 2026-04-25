---
title: Optimistic mark-read on viewer open
status: accepted
date: 2026-04-25
---

## Context

When the user presses Enter on an unread message, two things must
happen: the body needs to be fetched (latency-bound: 0–6s for JMAP
blob retrieval), and the message needs to be marked seen (latency-
bound: round-trip to the backend). Naively serializing makes the
viewer feel sluggish and the read-state flip lag visibly.

## Decision

`AccountTab.openSelectedMessage` flips the local seen flag
immediately via `MessageList.MarkSeen(uid)` before dispatching the
backend `MarkRead` Cmd in the same Update batch. The viewer opens
into the loading phase concurrently. When the user closes the
viewer (`q`/`Esc`) and returns to the list, the row already
displays in read styling regardless of whether the backend write
has completed.

Backend `MarkRead` errors are silently dropped this pass. The
toast surface from Pass 2.5b-6 will eventually surface persistent
sync failures.

## Consequences

- A viewer open + close round-trip on a flaky connection produces
  a "marked locally, backend will retry" gap that is invisible to
  the user. Pass 3 (live backend wiring) and Pass 2.5b-6 (status
  surface) are the right places to add a reconciliation indicator
  if drift becomes a real problem.
- `MessageList.MarkSeen` is the single mutation point for local
  read-state flips. Future triage actions (Pass 6: read/unread
  toggle, archive, delete) follow the same optimistic pattern via
  parallel local-mutate methods.
- The seen-count in the status bar does not refresh until the next
  folder reload. Live verification confirmed the per-row visual
  flip is what users notice; the count is acceptable as eventually-
  consistent for the prototype.
