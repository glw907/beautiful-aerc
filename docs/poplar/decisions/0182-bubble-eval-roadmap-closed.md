---
title: Bubble-eval roadmap closed; future evals require a flip condition
status: accepted
date: 2026-05-08
---

## Context

Pass 9y consolidated the verdicts from Eval A (five strong
matches), Eval B (four medium matches), and three harvest passes
(9x.1 list family, 9x.2 chrome family, 9x.3 table/form family).
Nine community libraries surveyed against poplar's hand-rolled
equivalents under one rubric. Every verdict was Keep + harvest.
Cumulative net adoption: one inline change (movepicker filter-
match rune highlight, 9x.1) plus one carry-forward "candidate for
later" (`bubbles/help` `shouldAddItem` truncation loop). Zero
structural diffs in the chrome and table/form families. The
roadmap is exhausted.

The risk going forward is repeated rubric-runs against the same
libraries when an adjacent UI question surfaces. The catalogs and
verdicts already name the structural blockers; running the rubric
again recovers the same answer at the same cost.

## Decision

The bubble-evaluation roadmap is closed. Future evaluations of any
of the nine surveyed libraries — `bubbletea-overlay`,
`bubbles/help`, `daltonsw/bubbleup`, `charmbracelet/huh`,
`evertras/bubble-table`, `bubbles/list` (any consumer),
`treilik/bubblelister`, `mistakenelf/teacup` statusbar — do not
re-run the Eval A/B rubric. They name the per-library flip
condition recorded in
`docs/poplar/research/2026-05-08-bubble-consolidation-verdict.md`,
present evidence the upstream change has landed, and present
evidence poplar's consumer-side contract is unchanged or has
deliberately moved into the new shape. Without all three, the
verdict stands.

New libraries — not surveyed in Eval A or Eval B — get a fresh
rubric run under the existing six-dimension rubric. The bar is
the same core question: *does adopting the bubble make poplar
better?*

The Eval A spec
(`docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md`)
and the original adoption-design spec
(`docs/superpowers/specs/2026-05-08-bubble-adoption-design.md`)
are archived alongside Eval B; their roadmap is now closed, and
the verdict doc is the operational reference.

## Consequences

The verdict doc replaces the evals as the load-bearing reference
for bubble-related decisions. Pass 9z's adoption work consumes it
directly: one harvested item (already landed) and one candidate
for later. No swap passes are queued.

Pass 14's first-run wizard remains free to adopt `huh` against a
static field set; the verdict doc explicitly preserves that as
huh's flip condition. That decision belongs to the Pass 14 plan,
not to this ADR.

The `JoinHorizontal` ban (ADR-0084) is the recurring blocker
across `bubbles/help`, `bubble-table`, and `mistakenelf/teacup`.
This ADR does not change ADR-0084; it records that three
independent libraries collide with it identically, which is
evidence the ban is correctly scoped.
