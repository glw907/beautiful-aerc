# Poplar Status

**Current pass:** Pass 14 — first-run wizard (#27) + config
template (#29). v2 substrate + reframes are done; the stack is
fully `charm.land/v2`.

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
| 14 | First-run wizard (#27) + config template (#29) | in progress |
| 14.1 | OAuth refresh (#42, deferred from 14) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 14)

> **Goal.** First-run wizard (#27) + config template (#29). On
> missing `~/.config/poplar/config.toml`, walk the user through
> account setup (provider selector, credentials, identity) and
> write a valid config. v2 huh integration is the wizard
> substrate; this is the first pass to use huh in poplar.
>
> **Source docs.** Plan:
> `docs/superpowers/plans/2026-05-09-pass-14-firstrun.md`.
> Spec: `docs/superpowers/specs/2026-05-09-pass-14-firstrun-design.md`.
> Both target the v2 stack — re-read at pass start; the substrate
> they assumed (chrome + cursor + paste) is now real, so nothing
> needs adjustment for that.
>
> **Untracked art.** `art/poplar-logo.ans` is Pass 14's; commit it
> alongside the wizard work.
>
> **Approach.** Read the plan + spec, brainstorm any open
> questions, execute. Standard pass-end checklist (ADR for the
> wizard architecture + huh adoption; invariants update;
> `bubbletea-conventions.md` refresh if huh introduces new
> patterns; plan + spec archive; STATUS pivot to Pass 14.1 or 15).
