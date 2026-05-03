---
title: Trim mail.Backend — remove Search and Copy
status: accepted
date: 2026-05-03
---

## Context

Pass 8.4a-cutover (ADR-0121) shrank `mail.Backend` to the methods the
cache layer actually needs. `Search` and `Copy` survived that pass on
the interface but had no consumers through the interface itself:

- Sidebar search is in-memory (`internal/ui/sidebar_search.go`), not a
  backend call. No code path called `Backend.Search`.
- The IMAP `Backend.Copy` wrapper was unreached. The internal `Move`
  implementation calls `cmd.Copy` (the lower-level `imapClient` method)
  as the no-`MOVE` fallback; the public wrapper added nothing.
- The JMAP `Backend.Copy` had no caller at all — JMAP `Move` patches
  `mailboxIds` directly, not via Copy.
- `MockBackend` carried Search/Copy stubs solely to satisfy the
  interface.

Pass 8.5's overengineering audit flagged both as vestigial.

## Decision

Remove `Search` and `Copy` from `mail.Backend`. The concrete impls are
deleted along with the interface methods:

- `internal/mailjmap/jmap.go`: drop `Backend.Search` stub (and its
  unit test); drop `Backend.Copy` and its three Copy unit tests; drop
  the `checkEmailSetCreated` helper (Copy was its only caller).
- `internal/mailimap/messages.go`: drop `Backend.Search`; drop the
  three Search test cases in `messages_test.go`.
- `internal/mailimap/actions.go`: drop the public `Backend.Copy`
  wrapper. The internal `Move` continues to use `cmd.Copy` as the
  no-`MOVE` fallback unchanged.
- `internal/mail/mock.go`: drop `MockBackend.Search` and
  `MockBackend.Copy` stubs.

## Consequences

The `mail.Backend` surface is one wire-format-shaped contract and
nothing else. Future server-side search will introduce a new method
(or a separate `Searcher` interface) with concrete consumers in the
same change. Future cross-account copy support will likewise add the
method back when there's a UI/cache caller for it.
