# Poplar Status

**Current pass:** Pass 9y — bubble consolidation verdict.
Pass 9z (adoption) follows; features (10–14) resume after.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9x.3 | Scaffold through table/form harvest (ADRs 0001–0181) | done |
| 9y | Bubble consolidation — final keep/swap verdict across all surveyed bubbles | pending |
| 9z | Adopt top-three findings from 9x.1–9x.3 + 9y catalogs | pending |
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

## Next starter prompt (Pass 9y)

> **Goal.** Read the 9x.1–9x.3 harvest catalogs together with Eval A
> and Eval B, then issue the final keep/swap verdict per bubble.
>
> **Scope.** Six libraries surveyed across three passes:
> `bubbles/list` + `bubbles/textinput` + `bubbles/textarea`
> (9x.1 list family); `bubbles/help` + `mistakenelf/teacup` +
> `daltonsw/bubbleup` (9x.2 chrome family); `charmbracelet/huh` +
> `evertras/bubble-table` (9x.3 table/form). Inputs:
> `docs/poplar/research/2026-05-08-bubbles-list-patterns.md`,
> `docs/poplar/research/2026-05-08-chrome-family-patterns.md`,
> `docs/poplar/research/2026-05-08-table-form-patterns.md`,
> Eval A at `docs/superpowers/archive/specs/2026-04-30-bubble-eval.md`,
> Eval B at
> `docs/superpowers/archive/specs/2026-05-08-bubble-reeval-and-eval-b.md`.
>
> **Settled (do not re-brainstorm):** Eval A and Eval B both produced
> Keep + harvest on every library. Three catalogs landed without
> diffs (9x.1 had one small adoption; 9x.2 zero structural; 9x.3
> zero structural plus a dead-field cleanup). The harvest itself
> is complete.
>
> **Still open — brainstorm these:** Whether the verdict reads
> differently when all three catalogs are on the table at once.
> Specifically: is there a swap candidate where the cumulative
> "did not survive" list now exceeds the "kept" surface area enough
> to flip the verdict? If yes, which? If no, what's the one-line
> argument for each Keep that future-poplar can lean on?
>
> **Approach.** Brainstorm, write a plan at
> `docs/superpowers/plans/YYYY-MM-DD-bubble-consolidation.md`,
> implement. Most likely shape: one ADR confirming Keep across
> all six bubbles with the consolidated rationale, plus a brief
> verdict doc at `docs/poplar/research/`. ADR only if a binding
> fact moves; the consolidated rationale lives in the verdict
> doc otherwise. Standard pass-end checklist.
