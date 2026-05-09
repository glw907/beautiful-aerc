# Poplar Status

**Current pass:** Pass 9x.2 — chrome family harvest.
Features (10–14) resume after 9x.2–9x.3 + 9y conclude.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9x.1 | Scaffold through bubbles/list harvest (ADRs 0001–0181) | done |
| 9x.2 | Harvest — chrome family (helppopover, statusbar, toast) | pending |
| 9x.3 | Harvest — table/form family (form, list, overlay) | pending |
| 9y | Bubble consolidation — decide if any swap work remains | pending |
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

## Next starter prompt (Pass 9x.2)

> **Goal.** Harvest concrete diffs and design patterns from the
> chrome-family bubbles into poplar's chrome surfaces.
>
> **Scope.** Three surfaces against the Eval B verdicts in
> `docs/superpowers/archive/specs/2026-05-08-bubble-reeval-and-eval-b.md`:
> `internal/ui/helppopover/` vs `bubbles/help`, the status-bar
> assembly in `internal/ui/account/` vs `knipferrc/teacup`, and
> the toast / undo bar in `internal/ui/` vs `daltonsw/bubbleup`.
> Per surface: 0–2 concrete diffs + pattern notes.
>
> **Settled.** All three verdicts are Keep + harvest. Catalog
> format = Pass 9x.1's research doc.
>
> **Still open — brainstorm these:** Whether `bubbles/help`'s
> KeyMap-driven ShortHelp/FullHelp shape replaces any of poplar's
> named-group binding tables; whether teacup's segment assembly
> fits the status-bar's drop-rank shape; whether bubbleup's queue
> model adds anything the toast cascade order doesn't already
> cover.
>
> **Approach.** Brainstorm, write plan at
> `docs/superpowers/plans/YYYY-MM-DD-harvest-chrome.md`,
> implement. Standard pass-end checklist; ADR only on binding-
> fact change. Pass must end with a "top three ways their code
> beats ours" section in the research doc — Pass 9z consumes
> those lists across 9x.1–9x.3.
