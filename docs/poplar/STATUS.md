# Poplar Status

**Current pass:** Pass 9x.1 — bubbles/list family harvest.
Feature passes (10–14) resume after the harvest sub-passes
(9x.1–9x.3) conclude and 9y decides what's left.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9w.2 | Scaffold through bubble re-eval (ADRs 0001–0181) | done |
| 9x.1 | Harvest — bubbles/list family (pickers, sidebar, dropdown) | pending |
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

## Next starter prompt (Pass 9x.1)

> **Goal.** Harvest concrete diffs from `bubbles/list`'s design
> into poplar's five list-shaped surfaces. Vibes get a one-line
> note; only diffs land.
>
> **Scope.** Five surfaces, all paired with `bubbles/list` in the
> archived eval
> (`docs/superpowers/archive/specs/2026-05-08-bubble-reeval-and-eval-b.md`):
> `internal/ui/movepicker`, `internal/ui/reader/linkpicker.go`,
> `internal/ui/reader/attachpicker.go`, `internal/ui/sidebar/`,
> `internal/ui/compose/dropdown.go`. Read `bubbles/list` source
> (`list.go`, `defaultitem.go`, `keys.go`) once; visit each
> surface; produce 0–2 concrete diffs per surface. Reserve the
> final task for cross-cutting patterns the per-surface work
> made obvious.
>
> **Settled:** All five verdicts are Keep + harvest. The
> delegate pattern + `list.Styles` struct shape are the named
> candidates. `JoinHorizontal` does not appear in the library's
> render path (Task 7 finding); width math is not the harvest.
>
> **Still open — brainstorm these:** Whether the `ItemDelegate`
> shape simplifies any current row renderer, or just adds an
> indirection layer. Whether `list.Styles` field shape is worth
> mirroring in any subpackage's `Styles` constructor. Whether
> any picker's filter / cursor / scroll machinery has obvious
> overlap with the library's that the eval missed.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-harvest-bubbles-list.md`,
> then execute. Per-surface task: read library + read surface,
> propose diff(s) or write a one-line skip note in the pass
> commit. Standard pass-end checklist applies; no ADR if no
> binding facts change.
