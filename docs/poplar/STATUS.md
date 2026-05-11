# Poplar Status

**Current pass:** Pass 19 — pre-beta refactor sweep. Bundles #46
iter.Seq2 walk, #43 drop legacy `Date string`, #44 compose package
collision, #47 Levenshtein consolidation, and #12 tidy → tidytext
rename. Must land before the v0.9.0 freeze (now Pass 20).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 16d | Scaffold through slog adoption (ADRs 0001–0197) | done |
| 17a | Sidebar folder hierarchy on a v2 tree component (ADR-0198) | done |
| 17b | `messagelist` on `bubbles/v2/list` (ADR-0199) | done |
| 17c | `bubbles/v2/help` audit + bubbles-deviation ADRs (0200, 0201) | done |
| 18 | Polish II — retire underlay dim, footer ellipsis, helppopover zero-arg View, KeyMap exports, sidebar render cache (ADR-0202) | done |
| **19** | **pre-beta refactor — #46, #43, #44, #47, #12** | **pending — next** |
| 20 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 19)

> **Goal.** Pre-beta refactor sweep. Land the five
> "pre-beta refactor" backlog items before v0.9.0 freeze.
>
> **Scope.** Five items, each bounded:
> - **#46** — `messagelist.appendThreadRows` → `iter.Seq2` walk
>   modeled after `Digital-Shane/treeview`'s
>   `NodeInfo{Node, Depth, IsLast}`. Touches
>   `internal/ui/messagelist/model.go` only.
> - **#43** — drop `mail.MessageInfo.Date string`; regenerate
>   stale fixtures; remove the `lessMessage` / `displayDate`
>   fallback branches. Touches `internal/mail/types.go`,
>   `internal/ui/messagelist/model.go`,
>   `internal/cache/syncer.go`, the messagelist testdata, and
>   the wire-shape paragraph in `docs/poplar/invariants.md`.
> - **#44** — rename `internal/compose/` → `internal/mailcompose/`
>   (or `internal/mail/outbound/`); drop the `uicompose` /
>   `mailcompose` alias dance from `internal/ui/compose/` and
>   the App side. Touches every importer + Architecture section
>   + ADR-0163.
> - **#47** — consolidate the two Levenshtein implementations
>   (`internal/config/accounts.go`, `internal/catkin/spellcheck.go`)
>   into a shared helper. Decide the package boundary first
>   (likely `internal/strdist/`).
> - **#12** — rename `internal/tidy/` → `internal/tidytext/` and
>   drop the CLI machinery (`LoadConfig`, `ApplyRuleOverrides`,
>   `ConfigString`). Rename `[ui.tidy]` config section to
>   `[ui.tidytext]`. Touches every importer + ADR-0178 +
>   invariants Architecture > Compose.
>
> **Settled (do not re-brainstorm):** All five are in scope per
> the pre-beta refactor mandate (CLAUDE.md). All must land
> before v0.9.0. Splits across multiple passes if the slate is
> too large; the pass-size budget is 8–12 tasks.
>
> **Still open — brainstorm these:** Pass split (one pass vs.
> two — count the touched importers and judge the integration
> risk); #44 target package name (`mailcompose` vs.
> `mail/outbound`).
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-pre-beta-refactor.md`,
> then implement. Standard pass-end checklist applies.

## Notes for the 16-series (modernization)

ADR-0196 binds the convention; 16b–d apply it. Audit appendix in
the archived 16a plan has the full file:line list. Pass 16d
landed ADR-0197 (slog adoption).
