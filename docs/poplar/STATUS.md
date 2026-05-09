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
> **Scope.** Three surfaces, all with eval verdicts in
> `docs/superpowers/archive/specs/2026-05-08-bubble-reeval-and-eval-b.md`:
> `internal/ui/helppopover/` (vs `bubbles/help`), the status-bar
> assembly in `internal/ui/account/` (vs `knipferrc/teacup`), and
> the toast / undo bar in `internal/ui/` (vs
> `daltonsw/bubbleup`). Read each library's render path once,
> walk the poplar surface, produce 0–2 concrete diffs per
> surface plus pattern notes for the catalog.
>
> **Settled (do not re-brainstorm):** All three eval verdicts are
> Keep + harvest. The 9x.1 pattern catalog at
> `docs/poplar/research/2026-05-08-bubbles-list-patterns.md` is
> the format reference — extend it (or fork a sibling chrome
> doc) per the next planning call.
>
> **Still open — brainstorm these:** Whether the `bubbles/help`
> KeyMap-driven ShortHelp/FullHelp shape can replace any part of
> poplar's named-group binding tables. Whether teacup's segment
> assembly is closer to the status-bar's drop-rank shape than the
> hand-rolled join. Whether bubbleup's queue model offers anything
> the toast cascade order doesn't already cover.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-harvest-chrome.md`, then
> implement. Standard pass-end checklist applies; ADR only if
> binding facts change.
