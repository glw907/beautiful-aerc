---
title: Pre-beta outbound cleanup
status: accepted
date: 2026-05-11
---

## Context

Three pre-beta refactor items shared an outbound-path theme — they
touched `mail.MessageInfo`, the compose package boundary, and the
tidy config key. Bundling them under one ADR keeps the rationale
in one place; splitting the mechanical-cleanup pair (#46, #47) to
Pass 19.1 keeps the review surface scoped to wire-shape work.

### `MessageInfo.Date string` is dead weight (#43)

`SentAt time.Time` has been authoritative for sorts and date-
column rendering since Pass 8.5. The parallel `Date string` field
survived as a "legacy fixture fallback" — every backend wrote it,
every renderer fell back to it when `SentAt.IsZero()`. Pre-beta
the fixtures are ours; the fallback is scaffolding.

### The compose package collides with itself (#44)

`internal/compose/` (domain) and `internal/ui/compose/` (UI) lived
side-by-side. Every importer of the domain package aliased it
`mailcompose` to disambiguate. The alias was already named after
the desired package — directly renaming the directory eliminates
eight `mailcompose "...internal/compose"` lines with zero call-
site cost.

### `internal/tidy` carries CLI scar tissue (#12)

`tidy` was conceived as a standalone CLI before being absorbed
into compose's `Ctrl+T` flow (ADR-0178). The standalone surfaces
— `LoadConfig`, `ApplyRuleOverrides`, `ApplyStyleOverrides`,
`ConfigString` — have no in-tree callers. The package name `tidy`
is too generic for what's now a poplar-internal grammar checker;
"tidytext" tracks the future standalone-tool name.

## Decision

Three changes, one pass:

1. **Drop `mail.MessageInfo.Date string`.** `SentAt` is the only
   date carrier. Cache schema migration v12 drops
   `messages.date_str`; the v1 INSERT/SELECT/scan paths shrink to
   the eleven remaining columns. The reader's `viewerDateString`
   formats `SentAt` directly; compose's reply attribution formats
   `parent.SentAt` as `"Mon, Jan 2 2006 at 3:04 PM"`. Renderers
   that received zero `SentAt` previously fell back to the wire
   string; now they render blank, which matches every other
   missing-field case.

2. **Rename `internal/compose` → `internal/mailcompose`.** The
   package name follows the `mailimap`/`mailjmap`/`mailauth`
   family. The `internal/ui/compose/` sibling is unchanged; the
   App-side `uicompose` alias survives because the UI surface
   still wants the shorter name. Cross-package imports drop
   their aliases: `mailcompose "...internal/compose"` becomes
   bare `"...internal/mailcompose"`.

3. **Rename `internal/tidy` → `internal/tidytext`; strip CLI
   surfaces.** `LoadConfig`, `ApplyRuleOverrides`,
   `ApplyStyleOverrides`, `ConfigString` and their tests are
   gone (about 150 source + 250 test LOC). `Validate(Config)`
   stays — it backs the decode path in `config.LoadUI`. TOML
   `[ui.tidy]` becomes `[ui.tidytext]`.

## Consequences

- Cache files at schema v11 auto-migrate on next open. The
  `ALTER TABLE messages DROP COLUMN date_str` runs in one
  transaction; failure leaves v11 in place.
- `mailcompose.DecodeDraft` is now the canonical name; the App
  side still calls it `mailcompose.DecodeDraft` (no alias).
- Pre-beta users who set `[ui.tidy]` in `config.toml` see the
  section silently ignored on next launch. Acceptable — count
  of pre-beta users is small, no on-disk data is at risk, and
  the wizard never wrote the section.
- ADR-0163 (compose package shape) and ADR-0178 (claude-tidy)
  reference the old names. They stand as historical record;
  invariants and INDEX point at the new names.
- Pass 19.1 follows with #46 (`messagelist.appendThreadRows` →
  `iter.Seq2` consumer) and #47 (strdist Levenshtein
  consolidation).
