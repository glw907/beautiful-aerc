# Poplar Status

**Current pass:** Pass 9w.2 next — bubble re-eval post-ansix.
Feature passes (10–14) resume after the bubble adoption effort
concludes at Pass 9z.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9w.1 | Scaffold through ansix shim (ADRs 0001–0181) | done |
| 9w.2 | Bubble re-eval post-ansix — Eval A revisit (a) ansix-unlocked swaps, (b) fork-vs-accept on `lipgloss.Width`-gated bubbles | pending |
| 9x | Bubble Eval B — partial matches (`bubbles/list` × 3 sites, `teacup` statusbar) | pending |
| 9y | Bubble Eval C — lesson harvests (messagelist threading, reader phases, App orchestration, domain chrome) | pending |
| 9z | Bubble consolidation roadmap — single ordered swap+harvest plan | pending |
| 9z.1+ | Bubble swap/harvest passes per 9z roadmap | pending |
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

## Next starter prompt (Pass 9w.2)

> **Goal.** Re-run Eval A's five candidates (`bubbletea-overlay`,
> `bubbles/help`, `bubbleup`, `huh`, `bubble-table`) against the
> post-ansix world and settle the fork-vs-accept question for the
> `lipgloss.Width`-gated set.
>
> **Scope.** Read-only research + writing. Two output lists in a
> new spec under `docs/superpowers/specs/`: (a) verdicts ansix
> alone flips from "Keep + harvest" to "Adopt" or "Adopt-with-fork"
> — these become candidates for 9z's swap roadmap; (b) bubbles
> still gated on internal `lipgloss.Width` calls (`bubbles/help`,
> `bubble-table`, `glamour`) — for these, decide once between
> **fork** (replace x/ansi or lipgloss via `go.mod replace`,
> permanent rebase, contradicts ADR-0002/0075) and **accept** the
> gating (these stay hand-rolled).
>
> **Settled (do not re-brainstorm):** Eval A's original verdicts
> in `docs/superpowers/specs/2026-05-08-bubble-eval-a-strong-matches.md`
> are the pre-ansix baseline. The original adoption design
> (`2026-05-08-bubble-adoption-design.md`) is the multi-pass
> framework. ADRs 0002/0075 ban replacing x/ansi or lipgloss; the
> fork option for (b) explicitly supersedes them if chosen.
>
> **Still open — brainstorm these:** Which Eval A "Keep" verdicts
> flip under ansix-aware width math. The fork-vs-accept binary for
> (b) — the cost/benefit math now that ansix exists. Whether
> Eval B (`bubbles/list` × 3 + `teacup`) folds into this re-eval
> or stays as a separate Pass 9x.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-bubble-reeval.md`, then
> execute the eval. Standard pass-end checklist applies.
