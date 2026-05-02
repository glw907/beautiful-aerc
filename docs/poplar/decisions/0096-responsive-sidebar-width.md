---
title: Responsive sidebar width
status: accepted
date: 2026-05-01
---

## Context

Pre-Pass-7, sidebar width was a fixed `const sidebarWidth = 30`. At
the design polish bar of 80×24 (ADR-0097), this left the message-list
pane only 48 cells, below the natural minimum for a threaded row.
Visible drift: threaded child rows lost the date column ("Thu
2026-04-") and same-day timestamps lost AM/PM.

## Decision

Sidebar width is `sidebarWidthFor(termWidth) = clamp(termWidth - 56,
24, 30)`. Linear from 24 at termWidth=80 up to 30 at termWidth=86,
flat at 30 above. The 56-cell offset is the message-list natural
minimum: flag(2) + icon(4) + sender(20) + thread-prefix(4) +
subject(8) + gap(2) + date(14) + sep(1) + right-border(1).

Folder labels in the sidebar truncate with `…` via
`displayTruncateEllipsis` when their natural width exceeds the
per-row label budget. Every rendered folder row preserves a 1-cell
right margin before the chrome divider, regardless of width.

## Consequences

The 80×24 polish bar is met. Long custom folder labels truncate at
narrower terminals (e.g., "Membership Committee" → "Membership
Commit…" at sidebar=24). Truncation is consistent within a session
because terminal width does not change without a resize. The
half-width fallback `min(sidebarWidthFor(width), width/2)` continues
to handle pathologically narrow widths.
