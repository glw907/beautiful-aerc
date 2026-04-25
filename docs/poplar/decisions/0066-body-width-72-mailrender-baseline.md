---
title: Body width cap 72 to match mailrender baseline
status: accepted
date: 2026-04-25
---

## Context

The content renderer capped body width at 78 cells, inherited from
the initial migration in Pass 2.5-render. The mailrender training
corpus (the reference for what "good rendering" looks like for
poplar) was tuned at 72 cells. Live verification of the viewer
prototype showed 78-cap output looking visibly slacker than the
training references at the same terminal width.

## Decision

`maxBodyWidth = 72` in `internal/content/render.go`. Headers
continue to wrap at the panel content width (not the body cap)
because the user reads address lists differently than prose.

## Consequences

- Every rendered email body is now 6 cells narrower. Adversarial
  wrap tests (`TestRenderBodyWidthCap`, `TestRenderBodyWrapStress
  Paragraph`, `TestRenderBodyNestedQuoteWrap`,
  `TestRenderBodyListHangingIndent`, `TestRenderBodyFootnoteEdge`)
  guard the cap.
- Body width is no longer a tunable. Future readability
  experimentation requires changing the constant and rerunning
  the wrap tests, not config.
- A long URL containing hyphens may still break mid-token because
  `ansi.Wordwrap` treats `-` as a breakpoint. Documented in
  `TestRenderBodyLongURL`. Acceptable for v1.
