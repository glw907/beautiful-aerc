---
title: Help popover future-binding policy (option C1)
status: accepted
date: 2026-04-25
---

## Context

The help popover advertises poplar's keybindings, but most of those
bindings (triage, reply, compose) are not wired yet — they ship in
later passes. Three options for handling unwired keys were on the
table (Audit-3 plan-shape findings §"Pass 2.5b-5"):

- **A. Show only currently-wired keys.** Sparse popover that churns
  every pass as bindings come online.
- **B. Show all eventual keys, treat as silent dead keys.** No
  feedback when an unwired key is pressed; toast feedback isn't
  available until Pass 2.5b-6.
- **C. Show all keys with a "future" marker.** Discoverable but
  needs a styling choice.

## Decision

**Option C, with C1 styling.** The popover advertises the full
planned vocabulary on day one. Each row carries a `wired bool` flag
in the static binding tables (`accountGroups`, `viewerGroups`,
`accountBottomHints`, `viewerBottomHints`). The flag drives styling:

- **Wired rows.** Key column rendered in `HelpKey`
  (`FgBright` + bold). Description rendered in `Dim` (`FgDim`, no
  bold).
- **Unwired rows.** Entire row rendered in `Dim` (`FgDim`, no bold).
  No glyph, no legend.
- **Group headings.** Always `HelpGroupHeader` (`FgBright` + bold)
  regardless of how many rows in the group are wired.

The contrast between bright-bold key columns on wired rows and the
flat-dim treatment on unwired rows is the *only* visual signal. It
matches the existing footer precedent in `keybindings.md` §"Future
hints shown" — the footer already advertises future bindings as
aspirational vocabulary.

Pass-by-pass, the wiring pass for each binding flips its `wired`
flag from `false` to `true` and the dim treatment lifts
automatically. No manual styling churn.

## Consequences

- The help popover is a discoverability surface from day one, not
  a sparse table that fills in over time. Users learn the planned
  vocabulary even before the bindings work.
- The dim treatment is a *promise*, not a tease — every dim row has
  a known future implementation pass.
- The popover and the footer (`footer.go`) maintain independent
  binding tables. Both describe the same planned vocabulary but
  serve different surfaces; convergence is expected by v1 and is
  not forced now (per the spec, "they diverge during the
  future-binding period and converge by v1").
- Pass 6 (triage actions), Pass 9 (compose), and the search-result
  pass each flip a known set of `wired` flags. Forgetting to flip
  a flag would be visually obvious during the live verification of
  the wiring pass.
