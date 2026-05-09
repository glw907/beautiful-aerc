# Poplar Status

**Current pass:** Pass 9x — Eval C lesson harvests.
Feature passes (10–14) resume after the bubble adoption effort
concludes at Pass 9y.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9w.2 | Scaffold through bubble re-eval (ADRs 0001–0181) | done |
| 9x | Bubble Eval C — lesson harvests against kept-custom code | pending |
| 9y | Bubble consolidation roadmap — single ordered swap+harvest plan | pending |
| 9y.1+ | Bubble swap/harvest passes per 9y roadmap | pending |
| 10 | Outbox delivery controls — undo + schedule send (#35) | pending |
| 11 | List-Unsubscribe (#36) | pending |
| 12 | `.ics` viewer (#37) | pending |
| 13 | Search (#38) | pending |
| 14 | First-run wizard (#27) + OAuth refresh + config template (#29) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 9x)

> **Goal.** Harvest concrete lessons from the bubble eval into
> kept-custom code — find diffs, not theories.
>
> **Scope.** Read-only analysis + targeted edits. For each
> Keep + harvest verdict in the archived eval spec
> (`docs/superpowers/archive/specs/2026-05-08-bubble-reeval-and-eval-b.md`),
> identify one concrete improvement with a diff. No new ADRs.
>
> **Settled:** All bubble verdicts are Accept/Keep. Fork-vs-accept
> is Accept (ADR-0002/0075 stand). The eval spec is archived.
>
> **Still open — brainstorm these:** Which harvest findings are
> concrete enough to act on (require a diff, not a comment). Which
> kept-custom surfaces benefit most from library design patterns.
>
> **Approach.** Read each verdict's "harvest" rationale, identify
> the strongest concrete-diff opportunity per candidate, apply.
