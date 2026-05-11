# Pass 19 — pre-beta refactor (outbound cluster)

Pure implementation pass. Three orthogonal items collapsed under
one ADR: wire-shape cleanup + two package renames.

## Settled decisions

- Pass split (outbound vs. mechanical) — outbound here; #46 and
  #47 land in Pass 19.1.
- #44 target package name is `mailcompose`. Matches existing
  siblings `mailimap`, `mailjmap`, `mailauth`. The existing
  `mailcompose "...compose"` alias at 8 importers becomes the
  unaliased package name; zero call-site rewrites beyond the
  import path.
- One consolidated ADR (0203): "Pre-beta outbound cleanup."
  Three sections, each ~one paragraph.

## Tasks

1. **#43 drop `MessageInfo.Date string`.** Remove the field on
   `internal/mail/backend.go`. Wire-shape consumers:
   - `internal/mailimap/realclient.go` — drop the `m["date"]`
     pre-rendered branch; only `sentAt` remains.
   - `internal/mailimap/messages.go` — drop the `info.Date, _ =
     v.(string)` decode line.
   - `internal/cache/reads.go:321` — drop `m.Date` from the
     INSERT column list (schema column stays for now; we just
     write empty — confirm schema and migrate cleanly if a
     migration is justified). Actually: check the schema. If
     the column exists, decide whether to migrate it out or
     just stop writing.
   - `internal/content/headers.go` + `render.go` — these are
     *MIME* headers, not `MessageInfo.Date`. Different type
     (`content.Headers.Date`). Leave alone.
   - `internal/ui/messagelist/model.go` — drop the
     `cmp.Compare(a.Date, b.Date)` fallback at line 449 and
     the `msg.Date` fallback at line 1068.
   - `internal/ui/reader/model.go` — drop the
     `viewerDateString` `msg.Date` fallback (line 453–454).
     Format `SentAt` directly.
   - Fixtures: any messagelist testdata with non-zero
     `SentAt` is fine; any that relied on `Date` string-only
     gets `SentAt` set.

2. **#43 update invariants.** The wire-shape paragraph in
   `docs/poplar/invariants.md` currently says "Date is a legacy
   display fallback (`lessMessage` uses it only when both
   `SentAt` are zero)." Rewrite: `SentAt` is the only date
   carrier; field is mandatory.

3. **#44 rename `internal/compose` → `internal/mailcompose`.**
   - `git mv internal/compose internal/mailcompose`
   - `sed -i 's|package compose|package mailcompose|'` on the
     moved files
   - Update 8 importers: drop the `mailcompose "...compose"`
     alias; bare import of `mailcompose`.
   - In `internal/ui/compose/` the *inner* alias
     `mailcompose "...internal/compose"` becomes a bare import
     too; the `uicompose` alias on the App side stays (it
     disambiguates from `internal/ui/compose`).

4. **#44 invariants + ADR.** Update Architecture > Send +
   Append + Compose references from `internal/compose/` →
   `internal/mailcompose/`. ADR-0163 gets a Consequences pointer
   to ADR-0203.

5. **#12 rename `internal/tidy` → `internal/tidytext`.**
   - `git mv internal/tidy internal/tidytext`
   - `sed -i 's|package tidy|package tidytext|'`
   - Update 5 importers (config/ui.go, ui/app.go, ui/compose/
     tidy.go, ui/compose/tidy_test.go, ui/compose/model.go).
   - Rename the type symbol `tidy.Config` → `tidytext.Config`
     etc. at call sites (or whatever the import-path-relative
     references use).

6. **#12 drop CLI leftovers.** Audit the renamed package for
   `LoadConfig`, `ApplyRuleOverrides`, `ConfigString` — these
   were the standalone-tool surfaces. If they are imported only
   from now-dead test/main paths, delete them.

7. **#12 rename `[ui.tidy]` → `[ui.tidytext]`.**
   - `internal/config/ui.go` TOML tag rewrite.
   - `internal/config/render.go` if it round-trips the section.
   - Any test fixtures.

8. **Pass-end ritual.** ADR-0203, invariants update, decisions
   INDEX row, STATUS.md to 19.1, archive plan, `make check`,
   `/simplify`, commit, push, install.

## Risk notes

- **#43 + cache schema.** If `outbox`/`messages` SQLite schema
  has a `date` TEXT column, dropping the wire field is fine —
  the column either stays (writes empty) or we migrate. Decide
  inline by reading `internal/cache/schema.go`. Migration would
  be a schema-version bump (cache-invariants); the pre-beta
  refactor stance welcomes schema work.
- **#44 alias dance** — `internal/ui/compose/` shadows the
  domain `compose` package. After the rename to `mailcompose`,
  the shadow goes away; the `uicompose` alias on the App side
  can stay (cleaner than a `compose` ↔ `compose` collision) or
  drop. Keep it for now — App-side reads `uicompose.Model`,
  `uicompose.SendMsg` etc.; renaming to bare `compose.X` would
  collide with the rare cases where `internal/compose`'s old
  name lingered. Verify after move.

## Out of scope

- #46 iter.Seq2 walk consumption (Pass 19.1).
- #47 strdist extraction (Pass 19.1).
- Any new compose/tidytext features.
- bubbles adoption work (Pass 15.x series).
