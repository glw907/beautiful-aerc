# Poplar Status

**Current pass:** Pass 13.2a — `charm.land/v2` substrate. Mechanical
migration only; the architectural reframes (declarative chrome,
hoisted cursor, paste handling) split out into Pass 13.2b.

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
| 13.2a | `charm.land/v2` substrate — mechanical migration + AdaptiveColor (ADR-0189a) | in progress |
| 13.2b | `charm.land/v2` reframes — declarative chrome, hoisted cursor, PasteMsg (ADR-0189b) | pending (blocked on 13.2a) |
| 14 | First-run wizard (#27) + config template (#29) | pending (blocked on 13.2b) |
| 14.1 | OAuth refresh (deferred from 14) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 13.2a)

> **Goal.** Land the `charm.land/{bubbletea,lipgloss,bubbles}/v2`
> substrate in poplar. Tree compiles on v2; `make check` green;
> every theme renders; tmux goldens for the still-v1-shaped
> App.View() unchanged. The architectural reframes (declarative
> chrome, hoisted cursor, paste handling) defer to Pass 13.2b.
>
> **Scope.** Module-wide import rewrite + KeyPressMsg
> field-access drift + bubbles field→method drift +
> `lipgloss.AdaptiveColor` removal. Touches every file in
> `internal/ui/`, `internal/catkin/`, `internal/theme/`,
> `internal/ansix/`, `cmd/poplar/`, every per-subpackage
> `styles.go`, every test. Does NOT touch the imperative chrome
> on the `tea.NewProgram` call (still `tea.WithAltScreen()`),
> per-input `cursor.Model` blink Cmds, cursored subpackages'
> `View() string` return type, or compose paste handling — all
> 13.2b territory. ADR-0189a required.
>
> **Settled (do not re-brainstorm):** The 13.2a / 13.2b split is
> done; specs and plans for both passes are written. 13.2a's
> tasks 1–3 are already complete from the original 13.2
> execution before the split.
>
> **Open work:** Tasks 4 (bubbles field→method), 5 (drop
> AdaptiveColor), 6 (test fixture sweep), 7 (`make check`
> green), 8 (ADR-0189a + invariants + bubbletea-conventions
> refresh), 9 (STATUS pivot to 13.2b + archive).
>
> **Approach.** Subagent-driven, no-scars discipline (CLAUDE.md
> "Migrations and breaking changes"). Tasks 4 + 5 bundle if a
> fresh implementer can hold both — they share theme +
> per-subpackage `styles.go` files. Otherwise sequence.
> Standard pass-end checklist applies.
>
> **Pass 14 status.** Plan
> (`docs/superpowers/plans/2026-05-09-pass-14-firstrun.md`) and
> spec (`docs/superpowers/specs/2026-05-09-pass-14-firstrun-design.md`)
> remain valid as written; both target the v2 stack. Pass 14
> blocked on 13.2b. The untracked `art/poplar-logo.ans` belongs
> to Pass 14 — leave it untracked through 13.2a + 13.2b.
