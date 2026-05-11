# Poplar Status

**Current pass:** v0.9.0 cut — beta soak begins.

Data formats are frozen at `v0.9.0`. Only bug-fix releases land
on the `v0.9.x` line; features queue on `1.1`. Soak rules are in
`docs/poplar/release-stance.md` — load before the first post-tag
pass.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 16d | Scaffold through slog adoption (ADRs 0001–0197) | done |
| 17a | Sidebar folder hierarchy on a v2 tree component (ADR-0198) | done |
| 17b | `messagelist` on `bubbles/v2/list` (ADR-0199) | done |
| 17c | `bubbles/v2/help` audit + bubbles-deviation ADRs (0200, 0201) | done |
| 18 | Polish II — retire underlay dim, footer ellipsis, helppopover zero-arg View, KeyMap exports, sidebar render cache (ADR-0202) | done |
| 19 | pre-beta refactor (outbound) — drop MessageInfo.Date string, compose→mailcompose, tidy→tidytext + CLI strip (ADR-0203) | done |
| 19.1 | pre-beta refactor (mechanical) — #46 reconciled, #47 strdist consolidation (ADR-0204) | done |
| 20 | v0.9.0 prep — README, docs sweep, hero screenshot, tag (ADR-0205) | done |
| 21 | Beta-soak bug fix — restore compose body Focus() (ADR-0206); compose hero screenshot; Nerd Font in VHS tapes | done |
| 22 | Wizard signature step — catkin editor between identity and label, multi-line TOML render, sentinel round-trip | done |
| Beta soak | Bug-fix releases on `v0.9.x`; data formats frozen; features queue on `1.1` | **active** |
| v1.0.0 | Tag when soak settles | pending |
| 23 | First-launch safety — #49 wizard preset, #29 name default, #51 MockBackend | queued |
| 24 | Outbox safety + small-refactor sweep — #52 IMAP send/append gate, #50 ansix Measurer, options collapse, backoff/humanize fold | queued |
| 1.1 | catkin-all-value → Editor wrapper deletion → app-decomposition → v2-view-fields → mouse-support → native OAuth (#42); plus neovim companion (#6), raw RFC822 (#21) | post-beta |

## Next steps

Soak is bug-fixes only on master; small reviewable refactors OK;
new features queue on `1.1`. Two soak passes are queued before
v1.0.0 tag:

**Pass 23 — First-launch safety.** Three first-launch / config
hazards converging on `internal/config/accounts.go`:
- **#49** wizard probe runs against unresolved preset (dies on
  "session URL is empty" for every hosted preset; first-run
  unusable). Fix: extract `config.ResolvePreset(*AccountConfig)`,
  call from both the decoder and `wizard.Apply`.
- **#29** config-template `name` defaults to email (drop the
  `name == ""` validator check; default `Name` to `Email`).
- **#51** `MockBackend` ships in production and silently swallows
  `Send`. Fix: `//go:build dev` tag on `mail.NewMockBackend`
  registration in `cmd/poplar/backend.go`; reject `provider = ""`
  and `provider = "mock"` in the production validator.

**Pass 24 — Outbox safety + small-refactor sweep.** Soak-safe
fixes and deletions with no user-visible behavior change:
- **#52** IMAP outbox `Append` can dispatch while sibling `Send`
  is failed (never-sent message lands in Sent). Gate
  `nextOutboxRow` with a `NOT EXISTS` subquery on the `draft_id`-
  linked sibling. No schema change.
- **#50** ansix `Measurer` (drop the `spuaCellWidth` package global)
- Collapse triplicate `WithLogger` functional-options in
  `mailjmap` / `mailimap` / `cache` to plain `*slog.Logger` args
  (amend ADR-0197)
- Fold `internal/backoff/` + `internal/humanize/` into their
  primary callers; drop the defensive `<= 0` clamps

**Reactive bug-fix passes** as soak surfaces issues. Tag
**v1.0.0** when soak settles.

**1.1 sequence (post-v1.0.0):**
1. `catkin-all-value` — Elm conformance (deepest; everything else
   in 1.1 sits on this)
2. Compose `Editor` wrapper deletion (subsumes
   `overengineering-cleanup` item 1; mechanical post-#1)
3. `app-decomposition` — split 874-line `App.Update`
4. `v2-view-fields` — `ProgressBar` + `ReportFocus` +
   `KeyboardEnhancements` (Kitty protocol sequenced after #1)
5. `mouse-support` (reader + attachments + scroll)
6. `mouse-support` (sidebar + cross-pane) — splits from #5 if
   task count argues for it
7. **#42** Native OAuth for Gmail / Outlook IMAP (BYO client ID)

Full project descriptions in `ROADMAP.md`.

## Notes for the 16-series (modernization)

ADR-0196 binds the convention; 16b–d apply it. Audit appendix in
the archived 16a plan has the full file:line list. Pass 16d
landed ADR-0197 (slog adoption).
