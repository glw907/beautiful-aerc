# Poplar outgoing-HTML allowlist

**Date:** 2026-07-27
**Status:** The CO-5 committed artifact, fixed in Phase 4 from the
survey's recipient-behavior research (Gmail CSS support
documentation, caniemail, transactional-email practice; sources in
`docs/poplar/research/2026-07-27-phase4-library-survey.md`). The
CO-5 acceptance test validates every generated HTML part against
this list; the list changes only by amending this document. A
one-time visual round-trip confirmation in Fastmail web and Gmail
web is recorded as a Phase 5 artifact, not run as a gate.

## Structure

`<!DOCTYPE html>`, `<html>`, `<head>` containing exactly
`<meta charset="utf-8">` and optionally
`<meta name="color-scheme" content="light dark">`, `<body>`, one
wrapping `<div>` carrying `max-width` and the base font styles.
No `<style>` block anywhere (Gmail mobile strips it). No `class`
or `id` attributes (Gmail strips them in several paths). All
styling is inline `style=` attributes.

## Elements

| Element | Attributes | Notes |
|---|---|---|
| `p`, `h1`-`h6` | `style` | heading margins via inline style |
| `strong`, `em`, `s`, `del` | — | semantic emphasis; CSS is fallback only |
| `a` | `href`, `style` | absolute http/https/mailto only |
| `ul`, `ol`, `li` | `style` | |
| `blockquote` | `style` | `border-left` + `padding-left` + muted color |
| `pre`, `code` | `style` | the element carries whitespace semantics; see CSS notes |
| `hr` | `style` | |
| `table`, `tr`, `td`, `th` | `style` | simple content tables only; no layout tables, no colspan |
| `div`, `span` | `style` | containers and inline styling spans |
| `br` | — | |
| `img` | `src` (cid: only), `alt`, `width`, `height`, `style` | attached/inline images only; poplar never generates remote references |

Everything else is forbidden, including `script`, `form`
elements, `iframe`, `svg`, and semantic elements with poor mail
support (`details`, `figure`).

## CSS properties (inline only)

Allowed: `font-family` (monospace stack on `pre`/`code`:
`ui-monospace, SFMono-Regular, Menlo, Consolas, monospace`),
`font-size`, `font-weight`, `font-style`, `text-decoration`,
`color`, `background-color` (small spans and code blocks only,
never `body` or the wrapper), `margin`, `padding` (positive
values only), `border`, `border-left`, `border-collapse`,
`text-align`, `max-width`, `white-space: pre` (enhancement only;
`<pre>` carries the load where clients ignore it), `line-height`.

Forbidden: `position`, negative margins, `float`, `display`
values other than default flow, animations/transitions,
`background-image`, any URL-carrying property.

## Postures

- **Dark mode**: no forced body background, tolerate client
  inversion; the `color-scheme` meta is best-effort. Syntax-
  highlight span colors must stay readable under Gmail's
  auto-invert; the Phase 5 round-trip checks this visually.
- **Plain part**: fixed wrap at 72 columns; no format=flowed
  (Gmail has never honored it). Fenced code and unfenced
  diff-shaped runs pass through verbatim, never wrapped
  (technical design section 10).
- **Width**: one `max-width: 640px` wrapper; no responsive
  breakpoints, no media queries.
