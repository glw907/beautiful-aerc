# Poplar Status

**Current pass:** Pass 14.1 — OAuth refresh (#42). Replace the
14b OAuth strategy stub (`echo TODO-pass-14.1-oauth`) with a real
refresh-token flow for Gmail and Outlook, and a keyring backend
for token persistence.

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
| 14.1 | OAuth refresh (#42, deferred from 14) | in progress |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 14.1)

> **Goal.** Replace the 14b OAuth strategy stub with a real
> refresh-token flow for Gmail and Outlook so those presets can
> finish the wizard end-to-end and the resulting `password-cmd`
> survives token rotation.
>
> **Scope.** The Gmail + Outlook XOAUTH2 path in `mailimap` plus
> the wizard's `StrategyOAuth` credentials form. Keyring storage
> for refresh tokens. CLI surface for re-auth (likely a sibling
> of `--repair`). No new wizard sections beyond the OAuth
> credentials step.
>
> **Settled (do not re-brainstorm):** OAuth lands in its own pass
> (14.1) per the 8–12 task budget; deferred from 14a/b/c. Gmail
> uses installed-app PKCE; Outlook uses MSAL device-code or
> installed-app PKCE — pick one in the brainstorm. Refresh tokens
> persist via the system keyring (`zalando/go-keyring` or similar
> Charm-adjacent lib; survey first).
>
> **Still open — brainstorm these:** keyring library choice;
> Outlook auth shape (PKCE vs device-code); the wizard's OAuth
> UX (browser-launch transcript vs paste-the-code); re-auth CLI
> entry point shape; where the access-token cache lives (Backend
> vs cache vs keyring); offline-token expiry behavior.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-oauth-refresh.md`, then
> implement. Standard pass-end checklist applies.
