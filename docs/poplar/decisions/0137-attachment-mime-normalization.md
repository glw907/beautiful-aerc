---
title: Attachment MIME-type normalization
status: accepted
date: 2026-05-04
---

## Context

`Content-Type` arrives in mixed case from both protocols
(`Application/PDF`, `image/PNG`) and may carry parameters
(`text/plain; charset=utf-8`). Filename can appear in either
the disposition `filename=` parameter or the content-type
`name=` parameter. ContentID is conventionally wrapped in
angle brackets (`<logo@x>`) but consumers want the inner
form.

## Decision

At the protocol→`mail.Attachment` boundary:

- `MIMEType` is stored as `type/subtype` lowercased with all
  parameters dropped. Charset is irrelevant for binary parts;
  for text parts (rare as attachments) the encoding is
  decoded by the backend client adapter before bytes reach
  the cache.
- `Filename` is its own column. Backends pull from the JMAP
  `Name` field (which already merges disposition `filename`
  and content-type `name`) or the IMAP body-structure
  `Filename()` accessor.
- `ContentID` is stored without surrounding angle brackets
  (`strings.Trim(s, "<>")` at the boundary). Consumers
  comparing CIDs against `cid:` URLs in HTML body do not need
  to strip again.
- `Disposition` is the typed `mail.Disposition` enum, stored
  in the cache as its `String()` form.

## Consequences

- Cache rows are search-friendly: lowercase MIME enables
  case-insensitive filter without `LOWER()` in queries.
- The viewer (Pass 8.7) can render `cid:` references by
  direct lookup against `attachments.content_id`.
- Backends are responsible for the boundary normalization;
  the cache trusts what it receives.
