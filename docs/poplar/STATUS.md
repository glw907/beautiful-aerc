# Poplar Status

**Current pass:** Pass 20 — v0.9.0 prep (feature freeze, docs
sweep, README, tag).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 16d | Scaffold through slog adoption (ADRs 0001–0197) | done |
| 17a | Sidebar folder hierarchy on a v2 tree component (ADR-0198) | done |
| 17b | `messagelist` on `bubbles/v2/list` (ADR-0199) | done |
| 17c | `bubbles/v2/help` audit + bubbles-deviation ADRs (0200, 0201) | done |
| 18 | Polish II — retire underlay dim, footer ellipsis, helppopover zero-arg View, KeyMap exports, sidebar render cache (ADR-0202) | done |
| 19 | pre-beta refactor (outbound) — drop MessageInfo.Date string, compose→mailcompose, tidy→tidytext + CLI strip (ADR-0203) | done |
| 19.1 | pre-beta refactor (mechanical) — #46 reconciled, #47 strdist consolidation (ADR-0204) | done |
| **20** | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | **pending — next** |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 20)

> **Goal.** Cut `v0.9.0`. Feature freeze, docs sweep, README,
> tag.
>
> **Scope.**
> - Audit `BACKLOG.md` High/Medium for any remaining pre-beta
>   refactors that should land before freeze (the "showcase
>   before freeze" initiative carved off the bubbles-adoption
>   work in passes 15.x/17.x; #44 `mailcompose` rename is the
>   one remaining named entry).
> - Sweep `docs/poplar/` for stale references (e.g. dropped
>   `Date` field, renamed packages, retired Pass numbers).
> - Write the v1 README (target audience: developer evaluating
>   poplar from the GitHub landing page).
> - Tag `v0.9.0` from master. Beta soak begins on tag; data
>   formats freeze.
>
> **Settled (do not re-brainstorm):** Release model is fixed
> (ADR-0105). Tag from master, no release branch.
>
> **Still open — brainstorm these:**
> - Final pre-beta refactor scope (apply #44 in this pass or
>   defer to a 19.2-style mechanical pass first?).
> - README scope and shape — feature tour vs. quickstart vs.
>   architecture overview.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-v0.9.0-prep.md`, then
> implement. Standard pass-end checklist applies.

## Notes for the 16-series (modernization)

ADR-0196 binds the convention; 16b–d apply it. Audit appendix in
the archived 16a plan has the full file:line list. Pass 16d
landed ADR-0197 (slog adoption).
