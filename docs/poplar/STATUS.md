# Poplar Status

**Current pass:** Pass 14b — wizard domain (`internal/wizard/`) +
huh-driven UI (`internal/ui/wizard/`) + the
`poplar config init --interactive` subcommand. 14a's substrate
(`mail.ProbeResult`, `mailimap.Probe`, `mailjmap.Probe`,
`Provider.CredentialStrategy`, `config.ConfigError`,
`config.Render`) is in. 14c wires first-run auto-launch.

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
| 14a | Probe + config substrate (#27 part 1, #29; ADR-0190) — `mail.ProbeResult`, `mailimap`/`mailjmap.Probe`, `Provider.CredentialStrategy`, `config.ConfigError`, `config.Render`, template fix | done |
| 14b | Wizard domain + huh UI (#27 part 2) — `internal/wizard/`, `internal/ui/wizard/`, account + theme sections, `config init --interactive` subcommand | in progress |
| 14c | First-run integration (#27 part 3) — `runRoot` auto-launch, `--repair=<name>`, `--no-wizard` / `POPLAR_NO_WIZARD=1` | pending |
| 14.1 | OAuth refresh (#42, deferred from 14) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 14b)

> **Goal.** Build the first-run wizard on top of 14a's
> substrate: `internal/wizard/` UI-free domain (`Model`,
> `Section`, `Strategy` interface + non-OAuth implementations,
> `Apply`, `Probe` dispatcher), `internal/ui/wizard/`
> (`charm.land/huh/v2`-driven UI with theme adapter +
> typographic logo + per-section sub-models + custom probe
> screen), and `poplar config init --interactive` cobra
> subcommand. First-run auto-launch + `--repair` defer to 14c.
>
> **Scope.** Tasks 6, 7, 10, 11, 12, 14 of the master plan
> `docs/superpowers/archive/plans/2026-05-09-pass-14-firstrun.md`,
> plus the standalone-subcommand subset of task 13. Pointer
> plan: `docs/superpowers/plans/2026-05-10-pass-14b-wizard-ui.md`.
> Spec (shared): `docs/superpowers/specs/2026-05-09-pass-14-firstrun-design.md`.
>
> **Untracked art.** `art/poplar-logo.ans` is Pass 14b's; embed
> via `//go:embed` and commit alongside the wizard work.
>
> **Settled (do not re-brainstorm):** UI library is
> `charm.land/huh/v2`; wizard organization is sections (not
> steps); strategies dispatch on `config.Provider.CredentialStrategy`;
> JMAP probe is 3 steps not 5 (library boundary, ADR-0190).
>
> **Still open — brainstorm these:** None pre-coded — master
> plan tasks 6/7/10/11/12/14 are fully specced inline. The
> theme-section live-preview cliff is the only place the plan's
> wireframe might disagree with bubbletea reality; verify on
> first render and ADR any deviation.
>
> **Approach.** Read the plan + spec, invoke `elm-conventions`
> and load `docs/poplar/bubbletea-conventions.md` before any
> `internal/ui/` file edit, execute. Pass-end ritual writes
> ADR-0191 (wizard architecture + huh adoption), updates
> invariants, archives the 14b pointer plan, pivots STATUS
> to 14c.
