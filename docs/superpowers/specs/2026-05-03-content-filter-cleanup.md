# Pass 8.5d — content/filter cleanup (queued)

**Date:** 2026-05-03
**Status:** queued
**Predecessor:** Pass 8.5c (UI structural cleanup)

## Goal

Two structural cleanups outside `internal/ui/`:

1. **HTML word fusing** (BACKLOG #23). The HTML→plain-text
   converter in `internal/filter/html.go` drops inter-element
   whitespace when adjacent inline elements (`<br>`, `<a>`,
   `<span>`) abut without a separating text node. The joined text
   loses the implicit word boundary the rendered HTML had. Visible
   artifacts in real email: `"Safari toSafari 18.6"`,
   `"to:Dave_99504@yahoo.comThanks,Dave Johnson"`. Affects
   readability of every HTML email with non-trivial inline
   structure. Likely fix: insert a space at element boundaries
   before tag stripping, or post-process to re-introduce spaces
   around fused alphanumeric runs.
2. **Dead `blockKind` / `spanKind` enums** (BACKLOG #13). The
   `Block` and `Span` interfaces in `internal/content/` require
   unexported marker methods (`blockType() blockKind`,
   `spanType() spanKind`) returning private kind constants — the
   sealed-sum-type pattern. Consumers never inspect the kind
   values; discrimination is always via Go type switches. Reduce
   the marker methods to no-args (`isBlock()`, `isSpan()`) and
   delete the constants (~30 LOC).

## Scope

`internal/filter/` (#23) and `internal/content/` (#13). Both
fixes are file-local; no cross-package changes.

## Settled (do not re-brainstorm)

- The sealed-sum-type pattern (interface + marker method) is the
  canonical shape; only the kind-returning aspect is wrong.
- The html-filter pipeline is established (the existing entry/exit
  points are stable; this is a fix inside one stage).

## Open questions (brainstorm before planning)

- Does #23's fix live in the HTML preprocessor (insert spaces
  before tag stripping) or in the post-tokenizer (rejoin fused
  alphanumeric runs)?
- For #13: rename the methods to `isBlock` / `isSpan`, or just
  drop their return values keeping the names?

## Approach

Brainstorm, plan at
`docs/superpowers/plans/YYYY-MM-DD-content-filter-cleanup.md`,
implement. Standard pass-end checklist applies.

For #23, capture before/after diffs of two real email fixtures
showing the word-fusion artifact, so the regression test has
concrete inputs.
