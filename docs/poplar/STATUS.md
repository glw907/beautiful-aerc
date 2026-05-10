# Poplar Status

**Current pass:** Pass 13.2b — `charm.land/v2` reframes (declarative
chrome, hoisted cursor, `tea.PasteMsg`). 13.2a code work and ADR
landed; pass-end consolidation (invariants commit) pending.

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
| 13.2a | `charm.land/v2` substrate — mechanical migration + AdaptiveColor (ADR-0189a) | done (consolidation pending) |
| 13.2b | `charm.land/v2` reframes — declarative chrome, hoisted cursor, PasteMsg (ADR-0189b) | in progress |
| 14 | First-run wizard (#27) + config template (#29) | pending (blocked on 13.2b) |
| 14.1 | OAuth refresh (#42, deferred from 14) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 13.2b)

> **Goal.** Land the v2 architectural reframes on top of the 13.2a
> substrate: declarative chrome (drop `tea.WithAltScreen()` from
> `tea.NewProgram`; set `view.AltScreen` / `MouseMode` /
> `ReportFocus` / `WindowTitle` on the returned `tea.View`),
> hoisted cursor (single ticker at App level, focus-chain walk in
> `App.View()`, drop every per-input `cursor.Model` + `cursor.Blink`
> Cmd), and `tea.PasteMsg` handling (compose address fields,
> subject, body — body delegates to a new catkin handler with
> bundle-as-one-Undo + URL-paste wrapping). `make check` green;
> tmux capture against goldens, triage cursor cell shifts.
>
> **Scope.** Cursored subpackages return `tea.View` with `Cursor`
> populated (or expose `Cursor() *tea.Cursor` accessor — pick
> consistently after reading v2 source); `VirtualCursor=false` on
> every textinput/textarea in compose, contacts.Form, messagelist
> search-mode input. ADR-0189b required.
>
> **Source docs.** Plan:
> `docs/superpowers/plans/2026-05-10-pass-13-2b-charm-v2-reframes.md`.
> Spec: `docs/superpowers/specs/2026-05-10-pass-13-2b-charm-v2-reframes-design.md`.
>
> **Prerequisite.** Finish 13.2a consolidation first — there are
> unstaged invariant updates (`.claude/rules/cache-invariants.md`,
> `ui-invariants.md`, `docs/poplar/invariants.md`, `CLAUDE.md`,
> deleted `internal/ui/date_format.go`/`_test.go`) that should ship
> with the 13.2a pass-end commit before 13.2b code starts.
>
> **Approach.** Subagent-driven, no-scars discipline. Standard
> pass-end checklist applies (ADR-0189b, invariants update,
> `bubbletea-conventions.md` refresh for the chrome / cursor /
> paste claims, plan archival, STATUS pivot to Pass 14).
>
> **Pass 14 status.** Plan
> (`docs/superpowers/plans/2026-05-09-pass-14-firstrun.md`) and
> spec (`docs/superpowers/specs/2026-05-09-pass-14-firstrun-design.md`)
> remain valid as written; both target the v2 stack. The untracked
> `art/poplar-logo.ans` belongs to Pass 14 — leave it untracked
> through 13.2b.
