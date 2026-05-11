# Poplar Status

**Current pass:** Pass 19.1 — pre-beta refactor (mechanical
cluster). Lands #46 (iter.Seq2 walk consumer) and #47 (strdist
Levenshtein consolidation).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 16d | Scaffold through slog adoption (ADRs 0001–0197) | done |
| 17a | Sidebar folder hierarchy on a v2 tree component (ADR-0198) | done |
| 17b | `messagelist` on `bubbles/v2/list` (ADR-0199) | done |
| 17c | `bubbles/v2/help` audit + bubbles-deviation ADRs (0200, 0201) | done |
| 18 | Polish II — retire underlay dim, footer ellipsis, helppopover zero-arg View, KeyMap exports, sidebar render cache (ADR-0202) | done |
| 19 | pre-beta refactor (outbound) — drop MessageInfo.Date string, compose→mailcompose, tidy→tidytext + CLI strip (ADR-0203) | done |
| **19.1** | **pre-beta refactor (mechanical) — #46, #47** | **pending — next** |
| 20 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 19.1)

> **Goal.** Land the mechanical-cleanup pre-beta refactors.
> One ADR.
>
> **Scope.**
> - **#46** — refactor `messagelist.appendThreadRows` to
>   consume the existing `walkThread` `iter.Seq2` iterator
>   (`internal/ui/messagelist/walk.go`) instead of its bespoke
>   recursion. Touches `internal/ui/messagelist/model.go` only.
> - **#47** — consolidate the two Levenshtein implementations
>   (`internal/config/accounts.go`, `internal/catkin/spellcheck.go`)
>   into `internal/strdist/`. Decide whether one shared signature
>   covers both call sites or whether they need narrower variants.
>
> **Settled (do not re-brainstorm):** Both items are bounded;
> #47 lands the new package at `internal/strdist/`.
>
> **Still open — brainstorm these:** None.
>
> **Approach.** Pure implementation pass. Plan doc at
> `docs/superpowers/plans/2026-05-11-pre-beta-mechanical.md`.
> Standard pass-end checklist applies.

## Notes for the 16-series (modernization)

ADR-0196 binds the convention; 16b–d apply it. Audit appendix in
the archived 16a plan has the full file:line list. Pass 16d
landed ADR-0197 (slog adoption).
