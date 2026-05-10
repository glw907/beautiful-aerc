# Poplar Status

**Current pass:** Pass 15 — Polish II. Popover dim (#14) plus
items surfaced during passes 10–14.1.

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
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14.1 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 15)

> **Goal.** Land Polish II: the popover-dim treatment (#14) plus
> the items surfaced during passes 10–14.1 that fit a single
> polish pass.
>
> **Scope.** Dim the underlay when any overlay opens (cascade in
> ui-invariants.md: confirm > conflict > outbox > help > link >
> attach > move). Plus a curated list of small UX/polish items
> surfaced during 10–14.1 — collect from the pass notes and any
> ADRs that flagged "polish in 15."
>
> **Settled (do not re-brainstorm):** Overlay cascade order;
> dim via `uicore.DimANSI` + `uicore.PlaceOverlay` (already in
> tree); single foreground-only color treatment matching the
> existing error-banner dim.
>
> **Still open — brainstorm these:** which surfaced items make
> the cut for #15 vs. defer to post-v0.9.0; whether to dim
> chrome too or only the message-list/viewer panes; whether
> popovers retain full color while underlay dims (Charm precedent)
> or both dim.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-polish-ii.md`, then
> implement. Standard pass-end checklist applies.
