# Poplar Status

**Current pass:** Pass 15a — `bubbles/v2/list` adoption for the
picker family (`movepicker`, `attachpicker`, `linkpicker`,
`helppopover`). First of four bubbles-adoption passes that must
land before Polish II and the v0.9.0 freeze, so poplar ships as a
first-class bubbletea v2 + bubbles showcase.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9z | Scaffold through bubble adoption (ADRs 0001–0182) | done |
| 10a | Outbox delivery controls — undo send (#35 part 1; ADR-0183) | done |
| 10b | Schedule send + sidebar Outbox (#35 part 2; ADR-0184) | done |
| 11 | List-Unsubscribe (#36; ADR-0185) | done |
| 12 | `.ics` invite viewer (#37; ADR-0186) | done |
| 13 | Background body sync + status indicator (substrate for #38; ADR-0187) | done |
| 13.1 | Search (#38; ADR-0188) | done |
| 13.1.5 | Claude infrastructure v2 prep (skills, conventions, pass ritual) | done |
| 13.2a | `charm.land/v2` substrate — mechanical migration + AdaptiveColor (ADR-0189a) | done |
| 13.2b | `charm.land/v2` reframes — paste; chrome + cursor absorbed by 13.2a (ADR-0189b) | done |
| 14a | Probe + config substrate (#27 part 1, #29; ADR-0190) | done |
| 14b | Wizard domain + huh UI (#27 part 2; ADR-0191) — `internal/wizard/`, `internal/ui/wizard/`, account + theme + confirm sections, `config init --interactive` subcommand | done |
| 14c | First-run integration (#27 part 3; ADR-0192) — `runRoot` auto-launch, `--repair=<name>`, `--no-wizard` / `POPLAR_NO_WIZARD=1` | done |
| 14.1 | OAuth refresh (#42; ADR-0193) | done |
| 15a | `bubbles/v2/list` adoption — `movepicker`, `reader/linkpicker`, `reader/attachpicker` | pending |
| 15a.5 | `bubbles/v2/filepicker` adoption — `compose/attachpicker` (multi-select gap to solve) | pending |
| 15b | Sidebar folder hierarchy on a v2 tree component (`Digital-Shane/treeview` candidate; ADR the dep) | pending |
| 15c | `messagelist` on `bubbles/v2/list` (custom item renderer for thread-prefix walk; ADR if it survives) | pending |
| 15d | `bubbles/v2/help` audit + ADRs for any bubbles deviations that survive 15a–c (`helppopover`, `schedulepicker`) | pending |
| 16 | Polish II — popover dim (#14) + items surfaced during 10–15d | pending |
| 17 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 15a)

> **Goal.** Adopt `charm.land/bubbles/v2/list` across the picker
> family so the four picker surfaces compose from the upstream
> component instead of hand-rolled list rendering. First of four
> bubbles-adoption passes (15a–15d) that land before Polish II,
> so v0.9.0 ships as a first-class bubbletea v2 + bubbles
> showcase.
>
> **Scope.** `internal/ui/movepicker/`, `internal/ui/attachpicker/`,
> `internal/ui/linkpicker/`, `internal/ui/helppopover/` (the
> popover's list portion; the help-key audit lands in 15d).
> Each surface keeps its current keybindings, theming via the
> per-subpackage `Styles`, and overlay cascade behavior. Custom
> item rendering is fine where the design demands it; the win
> is delegating selection state, filtering, and key dispatch to
> `bubbles/v2/list`.
>
> **Settled (do not re-brainstorm):** Adopt `bubbles/v2/list` —
> not a new in-tree abstraction. Per-subpackage `Styles` stays
> the styling seam (project from `*theme.CompiledTheme` as
> today). Overlay cascade unchanged. Modifier-free single-key
> bindings preserved (memory: no-modifier-keybindings).
>
> **Still open — brainstorm before coding:** whether to share a
> `uicore` helper that wires `theme.CompiledTheme` →
> `list.Styles` once, or each subpackage projects its own;
> whether the helppopover's list portion is a good fit for
> `list` at all (it may be small enough that custom is cleaner —
> if so, ADR the deviation in 15d); how to handle the
> messagelist's thread-prefix walk in 15c (out of scope for 15a
> but the answer shapes the shared-helper question).
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-bubbles-list-pickers.md`,
> then implement one picker at a time so each lands as a
> reviewable diff. Standard pass-end checklist applies.
