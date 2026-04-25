---
title: Footnote harvesting — `[^N]` markers + bottom URL list
status: accepted
date: 2026-04-25
---

## Context

The viewer needs an affordance to expose URLs without the visual
noise of inline `(https://...)` parentheticals or a separate link
picker invocation. Per the Pass 2.5b-4 spec the picker is deferred
to Pass 2.5b-4b, but the viewer should still ship a usable link
launch path.

## Decision

`content.RenderBodyWithFootnotes(blocks, theme, width) (string,
[]string)` walks blocks and rewrites every `Link` span with `Text
!= URL` to append ` [^N]` (no-break space + marker) to its text.
Duplicate URLs share a footnote number; auto-linked bare URLs
(`Text == URL`) render inline in link style without a marker. After
the body, a horizontal rule separates a `[^N]: <url>` list, one
per harvested URL.

The viewer harvests this list and binds keys `1`–`9` to launch the
Nth URL via `xdg-open` (fire-and-forget; errors drop until the
toast surface lands in Pass 2.5b-6). `Tab` is reserved for the
link picker (Pass 2.5b-4b) and is currently a no-op.

## Consequences

- Markers stay glued to their preceding word: ` ` (no-break
  space) joins the marker to the link text's last word so
  `ansi.Wordwrap` cannot orphan `[^N]` to the next line.
- Bodies with zero outbound links produce no rule and no list —
  the appendix only appears when it has content.
- `1`–`9` covers the common case (most messages have ≤9 links);
  the picker will handle the long tail and provide a discoverable
  affordance for arbitrary indices.
- Subsequent passes that present links elsewhere (e.g. a help
  popover or a modal picker) reuse the same harvesting walk —
  link enumeration is centralized in `RenderBodyWithFootnotes`.
