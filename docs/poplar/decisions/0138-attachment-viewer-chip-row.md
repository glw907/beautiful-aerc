---
title: Attachment chip row in viewer
status: accepted
date: 2026-05-04
---

## Context

Pass 8.6 added attachment metadata to the cache and backend. Pass
8.7 needs to surface that metadata to the user. The two natural
candidates were a separate "attachments" tab and an inline strip
near the body. The strip is the established pattern in mutt, aerc,
and most terminal mail clients; it keeps the viewer self-contained
so `n`/`N` thread navigation doesn't need to remember a separate
view state.

## Decision

The viewer renders a single chip row between the header panel and
the body. Each chip carries `<icon> <N>. <filename> (<size>)` and
chips greedy-wrap to the available width. The row is hidden when
the message has no attachments — `Viewer.renderChipRow` returns
`("", 0)` and `layout()` subtracts zero from the body height.

`AccountTab.openMessage` batches `loadAttachmentsCmd` alongside
`loadBodyCmd`. The result `attachmentsLoadedMsg` is dropped on a
UID mismatch (same pattern as `bodyLoadedMsg`).

## Consequences

- Body height adjusts dynamically with chip count — a ten-attachment
  message gives up two body rows. Acceptable; the alternative is
  truncating chip names, which loses information.
- The metadata fetch fires on every viewer open, even for messages
  with no attachments. `cache.Account.Attachments` does not cache
  zero-length results, so attachment-free messages incur a backend
  round-trip per open. A `has_attachments` hint on the messages
  table would eliminate this; deferred to a future pass.
- New invariant: chip row width math goes through
  `displayCells`/`displayTruncate` (icons live in SPUA-A range).
