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
| Beta soak | Bug-fix releases on `v0.9.x`; data formats frozen; features queue on `1.1` | **active** |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); native OAuth (#42); other post-beta | post-beta |

## Next steps

First post-tag work is bug-fix-only on the `v0.9.x` line. Pick up
the next pass with `release-stance.md` open — soak rules differ
from pre-beta (no schema work, no breaking renames). Candidates:

- Compose hero screenshot (deferred from Pass 20 — needs a
  hand-driven capture; tape skeleton at
  `scripts/screenshots/compose.tape`).
- BACKLOG bug entries flagged with `#bug` / `#poplar`.
- `CONTRIBUTING.md` + CI workflow (#41, #40) — v1.0 prep.

## Notes for the 16-series (modernization)

ADR-0196 binds the convention; 16b–d apply it. Audit appendix in
the archived 16a plan has the full file:line list. Pass 16d
landed ADR-0197 (slog adoption).
