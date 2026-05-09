# Poplar Status

**Current pass:** Pass 9x.3 — table/form family harvest.
Features (10–14) resume after 9x.3 + 9y conclude.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9x.2 | Scaffold through chrome family harvest (ADRs 0001–0181) | done |
| 9x.3 | Harvest — table/form family (form, list, messagelist) | pending |
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

## Next starter prompt (Pass 9x.3)

> **Goal.** Harvest design patterns from the table/form-family
> bubbles into poplar's form and list surfaces.
>
> **Scope.** Two libraries × poplar consumers, per Eval B at
> `docs/superpowers/archive/specs/2026-05-08-bubble-reeval-and-eval-b.md`:
> `charmbracelet/huh` vs `internal/ui/contacts/form.go`, and
> `evertras/bubble-table` vs `internal/ui/messagelist/` plus
> `internal/ui/contacts/list.go`. Per surface: 0–2 concrete diffs
> + pattern notes.
>
> **Settled.** Both Eval B verdicts are Keep + harvest (huh has
> two unfixable blockers: chrome baked into Group.View and
> frozen field slice; bubble-table conflicts with messagelist's
> threading pipeline + responsive column math). Catalog format
> = Pass 9x.1's research doc; subagent-driven shape from 9x.2
> works (per-section subagent → synthesis subagent → ritual).
>
> **Still open — brainstorm these:** Whether huh's `WithTheme`
> per-field blurred/focused style pairs reshape any of poplar's
> `Styles` slot tables; whether bubble-table's `WithRowStyleFunc`
> conditional-row pattern teaches messagelist or contacts/list
> anything beyond what they already do.
>
> **Approach.** Brainstorm, write plan at
> `docs/superpowers/plans/YYYY-MM-DD-harvest-table-form.md`,
> implement. Standard pass-end checklist; ADR only on binding-
> fact change. End with a "top three ways their code beats ours"
> section in the research doc — Pass 9z consumes the lists
> across 9x.1–9x.3.
