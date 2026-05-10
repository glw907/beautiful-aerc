# Poplar Status

**Current pass:** Pass 14 — First-run wizard (#27) + OAuth refresh
+ config template (#29).

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
| 14 | First-run wizard (#27) + OAuth refresh + config template (#29) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 14)

> **Goal.** First-run UX: an interactive wizard that builds
> `~/.config/poplar/config.toml` for new users (#27), wired to
> the existing provider preset registry. Pair with OAuth
> refresh-token handling (#29) so Gmail/Outlook XOAUTH2 accounts
> stay connected past the first access-token expiry.
>
> **Scope.** New `poplar config init --interactive` (or first-run
> auto-launch when no config exists) walks: provider pick →
> credentials → IMAP/JMAP probe → identity setup → write file.
> OAuth refresh: shared `internal/mailauth` helper that exchanges
> a refresh token for a fresh access token before each Connect,
> persisted via `password-cmd` integration. Config template
> rewrite (#29) folds the new fields into the canonical example.
>
> **Settled (do not re-brainstorm):** Wizard runs in bubbletea
> (no separate CLI prompt loop); reuses the existing provider
> preset machinery in `internal/config/`; OAuth lives at the
> auth layer per ADR-0098, not the backends.
>
> **Open — brainstorm:** Wizard flow shape (single screen vs
> stepped); OAuth refresh storage format (extend `password-cmd`
> protocol vs new keyring path); how the wizard handles probe
> failures (retry inline vs save-and-quit).
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-pass-14-firstrun.md`,
> implement. Standard pass-end checklist applies.
