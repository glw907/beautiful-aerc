# Poplar Status

**Current pass:** Pass 13 — Background body sync + status
indicator (substrate for #38). Pass 13.1 then delivers the
search UI on top.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9z | Scaffold through bubble adoption (ADRs 0001–0182) | done |
| 10a | Outbox delivery controls — undo send (#35 part 1; ADR-0183) | done |
| 10b | Schedule send + sidebar Outbox (#35 part 2; ADR-0184) | done |
| 11 | List-Unsubscribe (#36; ADR-0185) | done |
| 12 | `.ics` invite viewer (#37; ADR-0186) | done |
| 13 | Background body sync + status indicator (substrate for #38) | pending |
| 13.1 | Search (#38) | pending |
| 14 | First-run wizard (#27) + OAuth refresh + config template (#29) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 13)

> **Goal.** Background body sync per account so search (Pass
> 13.1) is snappy across the full mailbox, plus a status-bar
> indicator showing sync progress.
>
> **Scope.** New `internal/cache/backfill.go` worker:
> newest-first via `LEFT JOIN bodies WHERE bytes IS NULL ORDER
> BY sent_at DESC`, batch ceiling (~2 MB) with timer-slack
> between batches, idle-gated by a `lastActivity` timestamp
> threaded from `tea.KeyMsg`. Honors server back-pressure
> (`[THROTTLED]` / JMAP rate-limit) with exponential back-off
> (cap 60s, mirroring the outbox drainer). `[cache] max-size =
> 0` reinterpreted as unlimited (matrix-aligned default).
> Status bar gains a sibling segment `↓ N/M` (dim) with
> `paused` / `⚠` substates. Attachments stay lazy.
>
> **Settled.** Newest-first; implicit SQL queue (no watermark);
> idle-burst throttle from Thunderbird `nsAutoSyncManager`;
> status-bar sibling segment, not glyph overload; default
> `max-size = 0`. Two-pass split — 13.1 carries FTS5 +
> operator parser. See `docs/poplar/research/2026-05-09-*.md`.
>
> **Open — brainstorm:** exact `idleThreshold`; whether
> `pumpUpdatesCmd` new-mail UIDs route through backfill or
> fetch headers-only as today; back-off curve specifics; how
> the segment renders in the Spartan tier (80–89 cells).
>
> **Approach.** Brainstorm, write
> `docs/superpowers/plans/2026-05-09-pass-13-backfill.md`,
> implement. Standard pass-end checklist applies.
