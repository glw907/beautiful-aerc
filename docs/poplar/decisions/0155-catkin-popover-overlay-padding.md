---
title: Catkin popover overlay pads short body rows
status: accepted
date: 2026-05-05
---

## Context

Pass 9d.4 verified the spellcheck popover at the right and bottom
edges of an 80×24 viewport. The position math in
`popoverState.position` already follows convention: shift left
when the popover would clip the right edge, flip above the cursor
when it would clip the bottom. That part needed no change.

A real bug surfaced in `overlay()`. When the popover lands at a
column whose row in the rendered body is shorter than that column
(common for the rows below the cursor in a single-line document,
or any row past the natural end of body content), `ansiSpliceAtCol`
returned `"" + popLine + ""` and the popover collapsed to col 0.
At the right edge this meant the popover floated against the left
wall instead of anchoring under the misspelling.

## Decision

`overlay()` right-pads each body row with spaces to the requested
splice column before delegating to `ansiSpliceAtCol`. The
primitive is left untouched — its callers in the cursor and
annotation render paths splice within line bounds, so they never
needed padding. The overlay layer is the only call site that
splices past the natural end of a row.

Position strategy stays as it was — shift horizontally, flip
vertically. Both match the convention across major terminal and
GUI editors; no rationale to deviate.

## Consequences

The popover anchors under the misspelling at every viewport
position. Two render-level tests
(`TestPopoverRendersAtRightShiftedColumn`,
`TestPopoverFlipsAboveAtBottomEdge`) and a unit test on `overlay`
(`TestOverlayPadsShortLines`) guard the behavior. No on-disk
format change. No public API change. No new invariant — the
behavior is local to `overlay()`.
