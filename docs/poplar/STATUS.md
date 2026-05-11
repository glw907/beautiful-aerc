# Poplar Status

**Current pass:** Pass 18 — Polish II. Closes the popover-dim
backlog (#14) and the items surfaced during 10–17c. The bubbles-
adoption arc closed in 17c (ADRs 0198, 0199, 0200, 0201).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 16d | Scaffold through slog adoption (ADRs 0001–0197) | done |
| 17a | Sidebar folder hierarchy on a v2 tree component (ADR-0198) | done |
| 17b | `messagelist` on `bubbles/v2/list` with custom item delegate; iter.Seq2 thread walk (ADR-0199) | done |
| 17c | `bubbles/v2/help` audit + bubbles-deviation ADRs (0200, 0201); `account.keys.MsgList*` dedup | done |
| **18** | **Polish II — popover dim (#14) + items surfaced during 10–17c** | **pending — next** |
| 19 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 18)

> **Goal.** Polish II. Close issue #14 (popover dim) and sweep
> the small adjacent items surfaced during the 10–17c arc.
>
> **Scope.** `internal/ui/` polish only. Issue #14 — the dim
> underlay behind modal overlays should respect ADR-0072's
> wired/unwired vocabulary so the dim doesn't wash out the
> help popover's already-dim unwired rows. Plus whatever else
> appears in BACKLOG.md tagged "polish" that touches the
> overlay cascade, the chrome banner row, or the status bar.
>
> **Settled (do not re-brainstorm):** Overlay cascade order
> (confirm > conflict > outbox > help > linkpicker >
> attachpicker > movepicker > form > popover) stays. ModalShell
> family stays at six consumers + the help popover deviation
> (ADR-0201). `uicore.DimANSI` is the dimming primitive.
>
> **Still open — brainstorm these:** the exact BACKLOG.md
> selection for this pass (read backlog first; size to the
> 8–12 task budget); whether #14 wants a contrast-tier ADR or
> can ride on the existing dim invariant.
>
> **Approach.** Read BACKLOG.md, brainstorm the open questions,
> write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-polish-ii.md`, then
> implement. Standard pass-end checklist applies.

## Notes for the 16-series (modernization)

ADR-0196 binds the convention; 16b–d apply it. Audit appendix
in the archived 16a plan has the full file:line list. Pass 16d
landed ADR-0197 (slog adoption).
