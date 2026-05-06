---
title: Cache QueueOutbound and Backend.IsJMAP() — protocol branch in cache, not UI
status: accepted
date: 2026-05-06
---

## Context

Pass 9g landed `cache.Account.QueueSend` and `QueueAppend` (ADR-0158)
as the payload-bearing entry points for outbound mail. Those are
protocol-aware primitives: JMAP submission lands the Sent copy
atomically inside Send, while IMAP needs a separate APPEND to the
Sent folder. ComposeTab's send handler should not need to know which
protocol it's dealing with — App threads the same `Draft` through
the same cache call regardless of backend.

The question was where to put the branch. Three options: at the App
level (read backend type from cache.Account, decide which queue
calls to issue), inside each backend's QueueSend (let the backend
decide whether to also enqueue an Append, but cache is the layer
that owns the outbox so this is structurally wrong), or at the
cache layer behind a single entry point.

## Decision

Cache exposes `(*Account).QueueOutbound(ctx, sentFolder, env, mime)`
as the single entry point. It calls QueueSend, then conditionally
calls QueueAppend with `FlagSeen` against the same sent folder
unless the backend reports JMAP. Send-op enqueue failure short-
circuits the Append.

`mail.Backend` gains `IsJMAP() bool`. The two real backends and the
test fakes implement it; the cache reads it through
`a.Backend.IsJMAP()` inside QueueOutbound.

The predicate is on the public Backend interface (rather than a
narrow capability interface asserted at the call site) because
this is the only consumer and the cost of widening is one method.
The capability-interface alternative would add a type assertion
for no net simplicity.

## Consequences

- ComposeTab's send path is protocol-agnostic. App's
  `composeSendCmd` is a single `acct.QueueOutbound(...)` call and
  needs no IMAP/JMAP branch.
- `mail.Backend` interface gains one method. Test fakes
  (`fakeBackend`, `pagingFakeBackend`, `blockingBackend`,
  `noSentBackend`) all return `false` by default; mock backends
  expose `SetJMAP(bool)` for tests that want JMAP behavior.
- The Sent folder is resolved at the App layer via
  `resolveSentFolder` walking the cached classified-folder list
  (`cf.Canonical == "Sent"`); QueueOutbound takes the resolved
  name as a parameter. This keeps the cache from caring about
  which folder is "Sent" — that's an App-level UX concern.
