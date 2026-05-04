---
title: Inline-vs-attachment classification rule
status: accepted
date: 2026-05-04
---

## Context

Both backends parse a body-structure tree and need to decide
which leaves are user-visible attachments vs which are
displayable body or inline-rendered fragments. JMAP exposes
both `bodyStructure` and a server-precomputed `attachments`
subset; the latter is documented as non-authoritative for
inline classification (RFC 8621). IMAP exposes only
`BODYSTRUCTURE` extensions.

## Decision

Single canonical rule in `mail.ClassifyDisposition(rawDisposition,
contentID string) Disposition`, called by both backends at the
protocol→mail boundary:

1. If `Content-Disposition` is set and parses to `attachment`
   or `inline`, trust it.
2. Otherwise, if `ContentID` is non-empty, classify as inline
   (the part is referenced from the displayable body).
3. Otherwise, default to attachment.

Body skip rule (separate from disposition): when walking the
top-level multipart, drop direct-child `text/plain` and
`text/html` leaves — they are the displayable body, not
attachments. Parts nested inside inner multiparts (e.g. a
`multipart/related` payload referenced by `multipart/alternative`)
are never suppressed; the disposition rule alone classifies them.

JMAP's `attachments` field is therefore unused; we walk
`bodyStructure` only and drop `attachments` from the
`Email/get` property list.

## Consequences

- One classification function across both backends.
- No reliance on server precomputation that disagrees with
  Disposition headers.
- Inline images attached to plain-text-only messages still
  surface as attachments (Disposition absent, no ContentID).
