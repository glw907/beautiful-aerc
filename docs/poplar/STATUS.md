# Poplar Status

**Current pass:** Pass 9z — adopt surviving harvest items. Features
(Pass 10–14) resume after.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9x.3 | Scaffold through table/form harvest (ADRs 0001–0181) | done |
| 9y | Bubble consolidation verdict — nine libraries Keep; ADR-0182 closes the eval roadmap | done |
| 9z | Adopt surviving harvest items from the consolidated catalogs | pending |
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

## Next starter prompt (Pass 9z)

> **Goal.** Adopt the surviving harvest items from the bubble-eval
> roadmap. Pass 9y closed the roadmap; operational reference is
> `docs/poplar/research/2026-05-08-bubble-consolidation-verdict.md`
> and ADR-0182.
>
> **Scope.** Two carry-forward items across the three harvest
> catalogs: (1) `bubbles/help` `shouldAddItem` progressive-
> truncation loop (9x.2 §3) — port if helppopover needs progressive
> truncation today, else document the defer; (2) per-state combined-
> state style naming (9x.1 §3) — re-check the sidebar decline still
> holds. Plus any adjacent cleanups the harvest catalogs flagged but
> didn't land.
>
> **Settled (do not re-brainstorm):** Every Keep stands. ADR-0182
> requires a named flip condition from the verdict doc to revisit
> any surveyed library. Pass 14 may still adopt `huh` for the
> wizard — that's a Pass 14 decision.
>
> **Still open — brainstorm these:** Whether item 1 is live this
> pass or stays deferred; whether the catalogs surfaced adjacent
> cleanups worth landing now.
>
> **Approach.** Brainstorm, write a plan at
> `docs/superpowers/plans/YYYY-MM-DD-bubble-adoption.md`, implement.
> If the only durable adoption is item 1 and it's not live, the
> pass is documentation + housekeeping — that's a legitimate
> outcome; don't manufacture work. Standard pass-end checklist.
