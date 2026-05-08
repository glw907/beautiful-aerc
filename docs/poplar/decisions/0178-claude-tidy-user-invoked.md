---
title: Claude Tidy — user-invoked, in-place, character-highlighted
status: accepted
date: 2026-05-08
---

## Context

ADR-0159 introduced `TidyFn` as a function-pointer seam on `App`,
defaulting to `identityTidy` and slated to swap in a Claude-API
rewrite that ran on the send path before MIME assembly. The
plan was: user types, presses Ctrl+X, tidy rewrites, MIME
assembles, queue.

Two problems with the send-path shape:

1. Surprise edits. The user pressed send and the bytes went out
   different from what they typed. No preview, no undo window
   inside compose, no way to know what changed.
2. Failure mode. A tidy error would have to either block the
   send (bad — the user's already pressed go) or silently fall
   through to the un-tidied text (worse — invisible behavior).

The user-feedback memory captured during planning: AI rewrite
features must bind to explicit keys, show changes in-place, and
never run silently on send.

## Decision

Tidy is user-invoked from inside compose. `Ctrl+T` runs
`tidy.Tidy` against the current body in a `tea.Cmd`; on return
compose replaces the catkin buffer with the rewrite text and
feeds character-range diffs to a new `tidyAnnotator` in catkin
that paints the changed spans in `Styles.TidyChange`
(`AccentPrimary` + underline). Highlights clear on the first
body-mutation keystroke — the annotator stores the rewrite
src and returns no annotations once the buffer diverges from
it. Tidy never touches the send path.

The pipeline:

- `tidy.DiffRanges(old, new) []ByteRange` — pure rune-level LCS
  with a replace-block expansion so transpositions highlight the
  full disturbed window. Coalesces adjacent change positions into
  one byte range in newText.
- `catkin.tidyAnnotator` — `Set(src, ranges)` stores state;
  `Annotate(src)` returns annotations only while src matches
  exactly. Registered automatically by `catkin.New`.
- `compose.Editor` interface gains `SetStyles(catkin.Styles)`
  and `SetTidyHighlights(string, []catkin.Range)`.
- `compose.Model` exposes `SetTidy(enabled, apiKey, cfg)` and
  intercepts `Ctrl+T` ahead of the editor delegation. The result
  cmd lands as `tidyResultMsg{oldBody, res, err}` and
  `applyTidyResult` routes by `tidy.Status`.
- `[ui.tidy]` config block: `enabled`, `[ui.tidy.api]`,
  `[ui.tidy.rules]`, `[ui.tidy.style]` decode partial overrides
  onto `tidy.DefaultConfig()` and validate via `tidy.Validate`.
- `App` resolves the API key once at construction
  (`tidy.ResolveAPIKey`) and threads `(enabled, apiKey, cfg)` into
  every `compose.Model` via `SetTidy`. The footer adds a gated
  `^T tidy` hint at rank 6, visible only when `tidyEnabled` and
  the body is focused.

The `TidyFn` type, `App.WithTidy` setter, `identityTidy`,
`App.tidy` field, the `tidy: identityTidy` initialization, and
the `tidy TidyFn` parameter on `composeSendCmd` are all gone.
`composeSendCmd` is back to a straight assemble→queue.

## Consequences

The user sees what tidy changed — `AccentPrimary` underlined
spans in the body — and can keep typing, undo with catkin's
ring, or send as-is. Failure modes surface inline as `c.err`
without blocking anything else.

Tidy stays optional and gated: defaults to disabled, requires an
API key from `[ui.tidy.api] api_key` or `$ANTHROPIC_API_KEY`,
and the missing-key case sets a clear error rather than
attempting a doomed call.

The `internal/tidy/` package is closer to a standalone shape
(spinoff candidate per
[project_tidytext_spinoff](memory)) — the package now exposes
`Tidy`, `Validate`, `ResolveAPIKey`, `DiffRanges`, `ByteRange`,
`Result`, `Status*` constants, and `Default/Rules/Style/APIConfig`
as its public surface, with no poplar-specific dependencies.

ADR-0159's `TidyFn` clause is superseded.

The help popover does not advertise Ctrl+T at 80×24 — the layout
has no slack for a one-row Compose group, and adding it broke
the "Go To" group's render. The footer hint is the
discoverability surface that matters in compose context anyway
(the help popover only opens from account/viewer context).
