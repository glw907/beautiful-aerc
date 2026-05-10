# Poplar Status

**Current pass:** Pass 14c — first-run integration. Auto-launch the
wizard when `config.Load` returns `ErrFirstRun`; format
`config.ConfigError` in `runRoot` and point users at
`poplar --repair=<account>`. New flags: `--repair=<name>`,
`--no-wizard`, `POPLAR_NO_WIZARD=1`.

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
| 14c | First-run integration (#27 part 3) — `runRoot` auto-launch, `--repair=<name>`, `--no-wizard` / `POPLAR_NO_WIZARD=1` | in progress |
| 14.1 | OAuth refresh (#42, deferred from 14) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 14c)

> **Goal.** Wire 14b's wizard into `runRoot` so a missing config
> auto-launches the wizard (with opt-out) and a malformed config
> surfaces a typed `ConfigError` plus a `--repair=<name>` hint.
>
> **Scope.** Task 13's first-run + repair subset from the master
> plan `docs/superpowers/archive/plans/2026-05-09-pass-14-firstrun.md`.
> No new wizard sections; this pass is pure integration glue.
>
> **Settled (do not re-brainstorm):** opt-out is `--no-wizard` /
> `POPLAR_NO_WIZARD=1` (matches the master plan); `--repair=<name>`
> calls `Model.WithSections([]string{"account"})` plus a new
> `Model.WithRepair(name)` that pre-populates state from the
> existing account; `ConfigError` formatter prints
> `poplar: <msg>\nRun \`poplar --repair=<acct>\` to fix this account
> interactively.\nOr edit the file by hand and rerun poplar.`
>
> **Still open — brainstorm these:** None pre-coded — the master
> plan task is fully specced.
>
> **Approach.** Read the master plan §Task 13 (steps 13.3–13.4) and
> §Task 14 (live tmux smoke). Standard pass-end checklist applies.
> ADR-0192 records the runRoot integration shape if it deviates from
> the master plan.
