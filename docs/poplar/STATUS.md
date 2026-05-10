# Poplar Status

**Current pass:** Pass 13.2 — Migrate to `charm.land/{bubbletea,
lipgloss,bubbles}/v2`. Substrate for Pass 14's huh dependency.

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
| 13.2 | Migrate to `charm.land/v2` stack (substrate for #27 huh integration) | pending |
| 14 | First-run wizard (#27) + config template (#29) | pending (blocked on 13.2) |
| 14.1 | OAuth refresh (deferred from 14) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 13.2)

> **Goal.** Migrate poplar's UI stack from
> `github.com/charmbracelet/{bubbletea,lipgloss,bubbles}` (v1) to
> `charm.land/{bubbletea,lipgloss,bubbles}/v2`. Substrate for
> Pass 14's `charm.land/huh/v2` adoption — huh/v2 only links
> against the v2 stack, and trying to mix v1 and v2 inside one
> tea program produces incompatible `tea.Msg`/`tea.Cmd` types.
>
> **Scope.** Module-wide import rewrite + API drift. Touches
> every file in `internal/ui/` (App + 7 bubbles-shaped
> subpackages + `uicore`), `internal/catkin/`, `internal/theme/`,
> `internal/ansix/`, `cmd/poplar/`, every per-subpackage
> `styles.go`, `internal/term/`, every test that constructs a
> `tea.Model` or sends `tea.WindowSizeMsg`. ADR-required.
>
> **Settled (do not re-brainstorm):** Migration is mandatory —
> Pass 14 is blocked without it; rolling back to huh v1 was
> rejected since the spec is already written against huh/v2.
> Migration target is the canonical Charm v2 stack
> (`charm.land/...`), not a fork.
>
> **Open — brainstorm:**
> 1. **Sequencing.** Big-bang single commit (whole module flips
>    at once) vs incremental subpackage-by-subpackage with
>    adapter shims. Pre-beta posture says no shims, so big-bang
>    is the default — but does the tree even compile in
>    intermediate states under that approach?
> 2. **API drift surface area.** Where do v2's API changes hit
>    poplar concretely? `tea.Program` options, `tea.Cmd`
>    signatures, `lipgloss.Style` ergonomics, `bubbles/textinput`
>    + `viewport` API churn, the new color profile model. What
>    needs codemod-able sed-passes vs hand-edits?
> 3. **SPUA / ansix.** `internal/ansix/` wraps
>    `charmbracelet/x/ansi`. v2 lipgloss has its own width
>    story; does `ansix.SetSPUACellWidth` still need to exist
>    or does the v2 stack handle cell-width math directly?
> 4. **Test fixtures.** Are existing `tea.Model.Update` test
>    patterns portable, or do tests need bulk rewriting?
> 5. **Pass-budget reality.** This is plausibly larger than
>    12 tasks. If the brainstorm shows it, split into 13.2a /
>    13.2b before coding.
>
> **Approach.** Read each charm.land/v2 package's CHANGELOG +
> migration guide first (use the `bubbletea-conventions` doc as
> the contract; only the dependency tier changes). Brainstorm
> the open questions, write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-pass-13-2-charm-v2.md`,
> implement. Standard pass-end checklist applies.
>
> **Pass 14 status.** Plan
> (`docs/superpowers/plans/2026-05-09-pass-14-firstrun.md`) and
> spec (`docs/superpowers/specs/2026-05-09-pass-14-firstrun-design.md`)
> remain valid as written; both target the v2 stack already.
> Pass 14 will execute against the migrated tree without
> re-planning. The untracked `art/poplar-logo.ans` belongs to
> Pass 14 — leave it untracked through 13.2.
