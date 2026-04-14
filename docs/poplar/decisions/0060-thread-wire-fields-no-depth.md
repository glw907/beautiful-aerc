---
title: Wire fields ThreadID + InReplyTo only; depth derived in UI
status: accepted
date: 2026-04-13  # Pass 2.5b-3.6
---

## Context

Threading needs three pieces of information per message: which
conversation it belongs to, who its parent is, and how deep it
sits in the reply tree. JMAP and IMAP both surface the first two
directly (`Email.threadId`, `Email.inReplyTo`); depth is not
carried by either protocol — it has to be computed from the
parent chain.

The question was whether `MessageInfo` should carry a `Depth`
field on the wire (computed by the backend adapter and shipped
to the UI) or only `ThreadID`/`InReplyTo` (with depth derived
in the UI's prefix walk).

## Decision

`MessageInfo` gains exactly two new fields: `ThreadID UID` and
`InReplyTo UID`. There is no wire `Depth`. Depth is computed in
`internal/ui/msglist.go` during the `appendThreadRows` walk and
stored on the transient `displayRow` only.

A non-threaded message is a thread of size 1 with
`ThreadID == UID` and `InReplyTo == ""`. The mock backend follows
this convention.

## Consequences

- One source of truth for depth — the prefix walk. A buggy
  backend can't disagree with the renderer about how deep a
  message sits.
- The wire format stays minimal. Two backends to populate
  (Fastmail JMAP, Gmail IMAP), each with one obvious mapping.
- `displayRow.depth` is `uint8` — 256 levels is far beyond any
  realistic thread depth; the tighter type is documentation.
- Forecloses backends that want to ship pre-computed nesting
  metadata (e.g., a notmuch-style threaded query that returns
  rows already in display order). If such a backend appears,
  it will need to populate `InReplyTo` synthetically and let the
  UI re-derive depth — or invalidate this ADR.
