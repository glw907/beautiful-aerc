---
title: 80×24 is the design polish bar
status: accepted
date: 2026-05-01
---

## Context

80×24 is the default first-launch terminal size on every VT100-lineage
terminal — macOS Terminal.app, GNOME Terminal (Ubuntu/Linux Mint),
Konsole, Alacritty, Kitty, iTerm2, xterm. Only Windows Terminal
defaults wider (120×30). For poplar, 80×24 is therefore the
default-launch user experience for ~95% of the target audience.

BACKLOG #15 ("Help popover responsive layout for narrow terminals")
imagined progressive reflow strategies for sub-80 widths (single-
column stacking, column dropping). After the Pass 7 audit, sub-80
terminals are deemed an uncommon use case for an email client; the
existing `tooNarrow` fallback string in `HelpPopover.Box` covers them
adequately.

## Decision

80×24 is the design polish bar. Every overlay, panel, and rendering
path must look intentional at 80×24 — date columns intact, threaded
rows complete, folder labels truncated cleanly, no border collisions,
no clipped subjects.

Below 80×24, rendering is best-effort. The help popover's `tooNarrow`
fallback fires when its natural box width exceeds the terminal width
(`Terminal too narrow for help popover`). No further reflow strategy
is implemented.

Help popover natural-width budget: account context ≤62 cells, viewer
context ≤58 cells. Both fit at 80 cols once the sidebar narrows
(ADR-0096).

Closes BACKLOG #15.

## Consequences

Pass 7's responsive sidebar (ADR-0096) is the load-bearing change for
this bar. Future passes that touch overlays must verify at 80×24 as
part of the pass-end checklist. The pass-end consolidation ritual in
the `poplar-pass` skill already enforces a 120×40 capture; this ADR
adds an 80×24 capture as a peer requirement when UI is touched.

#15 is closed without further code change.
