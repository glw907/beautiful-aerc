---
title: Catkin render-time soft wrap (Typora / iA Writer model)
status: accepted
date: 2026-05-19
---

## Context

Before this pass, catkin wrapped at `WithWidth` by mutating the
source buffer via `Reflow`. Long quoted paragraphs grew past the
pane mid-edit and required a window resize to re-fit; the source
was no longer pristine, so byte-faithful roundtripping through a
draft save was impossible.

## Decision

Soft wrap is a render-time operation. `RenderAnnotated` calls
`visualWrap` per source line: it word-wraps via `ansi.Wrap`
(overlong tokens hardwrap, ANSI preserved across breaks), and for
quoted lines strips the prefix, wraps the body against
`width - ctx.PrefixWidth`, and re-prepends the styled prefix to
every emitted row. `styledQuotePrefix` shares face selection with
`styleLine` via the `quoteFace(depth, st)` helper. Scroll-off
operates in visual-row space via `cursorVisualRow`, which sums
per-line `visualHeight` values; `viewportTop` is a visual-row
index, and `Render` skips the first `top` visual rows when slicing
into the viewport. `Model.WithWidth` no longer calls `Reflow`;
`Reflowed()` remains for callers that want the legacy wrap-into-
source behaviour.

## Consequences

- Source is pristine. Typing in a deep quote no longer overflows
  the pane, and a draft saved mid-edit roundtrips byte-faithfully.
- Resize redraws into the new width instantly; no buffer mutation,
  no recomputation of the source.
- `viewportTop` is now a visual-row index. No external consumer
  read the field today, so the migration is silent.
- `renderFences` is called with `(0, len(lines))` rather than a
  source-line viewport range; the chroma `sync.Map` cache (per
  catkin invariants) absorbs the repeat-render cost. Restoring a
  visual→source-line viewport guard is a future efficiency pass
  once a profile justifies it.
- List-item continuation styling, code-fence wrap behaviour, and
  binding `Reflowed()` to a manual command remain future work.
- Supersedes the `softWrap via ansi.Hardwrap` clause in
  `.claude/rules/catkin-invariants.md`.
