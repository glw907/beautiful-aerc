# Poplar Status

**Current pass:** Pass 13.1 — Search (#38) on top of the body-sync
substrate landed in Pass 13.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9z | Scaffold through bubble adoption (ADRs 0001–0182) | done |
| 10a | Outbox delivery controls — undo send (#35 part 1; ADR-0183) | done |
| 10b | Schedule send + sidebar Outbox (#35 part 2; ADR-0184) | done |
| 11 | List-Unsubscribe (#36; ADR-0185) | done |
| 12 | `.ics` invite viewer (#37; ADR-0186) | done |
| 13 | Background body sync + status indicator (substrate for #38; ADR-0187) | done |
| 13.1 | Search (#38) | pending |
| 14 | First-run wizard (#27) + OAuth refresh + config template (#29) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 13.1)

> **Goal.** Cross-folder search (#38) reading from the local
> body cache that Pass 13's `Backfiller` populates.
>
> **Scope.** New SQLite FTS5 virtual table indexed off
> `messages` + `bodies`, populated synchronously by `storeBody`
> and (where applicable) by header upserts. Operator parser for
> the search query (`from:`, `to:`, `subject:`, `has:attachment`,
> bare terms = subject + body); the survey doc names the
> matrix-aligned operator set. UI surface: extend the existing
> sidebar search shelf to support `\` (cross-folder mode toggle)
> and route hits to a synthetic results pane. The `↓ N/M`
> backfill segment (already in chrome) covers progress feedback
> while indexing is incomplete.
>
> **Settled (do not re-brainstorm):** SQLite FTS5 (no Bleve,
> no external index); operator set per the matrix survey;
> sidebar shelf as the entry point (no new top-level overlay);
> backfill is the substrate (Pass 13.1 reads, doesn't write
> the population worker). See `docs/poplar/research/2026-05-09-
> mail-client-search-survey.md`.
>
> **Open — brainstorm:** FTS5 schema shape (separate virtual
> table vs FTS-shadowed messages); index-update transaction
> boundaries (inside `storeBody` tx vs trigger); throttle-state
> wiring for the `↓ ⚠` warn substate left unwired in Pass 13;
> results-pane keymap + sort.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-pass-13-1-search.md`,
> implement. Standard pass-end checklist applies.
