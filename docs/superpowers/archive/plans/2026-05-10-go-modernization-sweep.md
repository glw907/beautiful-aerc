# Pass 16b — Go modernization sweep

Mechanical apply of the modern-stdlib idioms ADR-0196 binds.
Working set comes from `./scripts/modern-go-check.sh` (59 findings
on 2026-05-10) plus the 16a audit appendix. No new ADR.

## Conventions

- **M1 sort.Slice/SliceStable** → `slices.SortFunc` /
  `slices.SortStableFunc` with `cmp.Compare`; multi-key uses
  `cmp.Or(cmp.Compare(...), cmp.Compare(...))`.
- **M2 sort.Strings/Ints/Float64s** → `slices.Sort`.
- **M3 three-clause `for i := 0; i < N; i++`** with `i` unused in
  body → `for range N`. False positives (loops that read `i`,
  loops with custom step, loops with compound condition) get
  left alone — they are flag-only per ADR-0196.
- **M4 sync.Once + result var** → `sync.OnceValue` /
  `sync.OnceFunc`.
- **Map-keys-then-sort** → `slices.Sorted(maps.Keys(m))`.
- Drop any leftover `x := x` loop-var shadow noticed inline (1.22+).

## Working set by file

### M1 — Sorting (11 sites)

- `internal/mailimap/messages.go:37`
- `internal/ui/compose/attachpicker.go:160`
- `internal/ui/contacts/list.go:218`
- `internal/ui/contacts/fixtures.go:325`
- `internal/ui/sidebar/model.go:383` — multi-key (rank+name) → `cmp.Or`
- `internal/ui/messagelist/model.go:190`
- `internal/ui/messagelist/model.go:399`
- `internal/contacts/vcard.go:93`
- `internal/catkin/spellcheck.go:209`
- `internal/catkin/annotate.go:42`

### M2 — Slice sort of `[]string` (3 sites)

- `internal/theme/themes.go:307` — pair with map-keys candidate
- `internal/ui/compose/attachpicker_test.go:73`
- `internal/config/accounts.go:634`

### M3 — `for range N` (40+ sites)

Production:
- `internal/content/parse.go:326`
- `internal/content/render_footnote.go:147`
- `internal/ansix/ansix.go:48`
- `internal/mailjmap/jmap.go:441`
- `internal/ui/helppopover/model.go:319`
- `internal/ui/compose/attachpicker.go:351`
- `internal/ui/reader/linkpicker.go:139`
- `internal/ui/messagelist/model.go:459`
- `internal/ui/messagelist/model.go:844`
- `internal/ui/status_bar.go:115`
- `internal/ui/top_line.go:28`
- `internal/catkin/spellcheck.go:127`
- `internal/catkin/spellcheck.go:149`
- `internal/catkin/style.go:114`
- `internal/catkin/commands.go:35`
- `internal/catkin/commands.go:61`
- `internal/catkin/reflow.go:144`
- `internal/catkin/buffer.go:43`
- `internal/config/accounts.go:275`

Test:
- `internal/content/render_test.go:155`
- `internal/cache/backfill_test.go:114,185`
- `internal/cache/backfill_progress_test.go:14,17`
- `internal/ui/golden_test.go:63`
- `internal/ui/helppopover/model_test.go:244,259,272`
- `internal/ui/compose/schedulepicker_test.go:33,60`
- `internal/ui/compose/model_test.go:402`
- `internal/ui/movepicker/model_test.go:89,106,305`
- `internal/ui/contacts/list_test.go:75`
- `internal/ui/uicore/modal_shell_test.go:34`
- `internal/ui/sidebar/column_test.go:46`
- `internal/ui/sidebar/model_test.go:152`
- `internal/ui/account/golden_test.go:32`
- `internal/ui/account/model_test.go:337`
- `internal/ui/conflict_overlay_test.go:19`
- `internal/catkin/popover_test.go:253,287`
- `internal/catkin/undo_test.go:50`

Each site checked individually: if the body never reads `i` (or
`j`/`r`/`d`), rewrite. If it does, leave with a brief look at
whether `for i := range N` (range-int form) is cleaner.

### M4 — OnceValue/OnceFunc (2 sites)

- `internal/term/font.go:16,37` — collapse `hasNerdFontOnce`,
  `hasNerdFontResult`, and `HasNerdFont()` to
  `var HasNerdFont = sync.OnceValue(func() bool { ... })`.
  Call sites are `term.HasNerdFont()` — already function-shape, no
  changes needed there.
- `internal/catkin/spellcheck.go:26` — `once sync.Once` struct
  field + `delIdx` paired with `buildIndex`. Rewrite to
  `OnceFunc(buildIndex)` initialized in the constructor (or kept
  as a method-bound closure).

## Order of attack

1. M1 sorts (10 files) — straightforward, no behavior change.
2. M2 string sorts (3 files) — trivial; fold `theme/themes.go`
   into a `slices.Sorted(maps.Keys(...))` if the surrounding code
   makes it natural.
3. M4 OnceValue/OnceFunc (2 files) — biggest semantic delta, do
   while attention is high.
4. M3 `for range N` — bulk pass, file-by-file. Tests last.
5. `make check` + `MODERN_GO_STRICT=1 ./scripts/modern-go-check.sh`
   green before pass-end ritual.

## Verification gates

- `make check` green.
- `MODERN_GO_STRICT=1 ./scripts/modern-go-check.sh` exits 0.
- No behavior changes — diff is pure rewrites.
- Pass-end ritual via `poplar-pass` (no ADR, no invariants edit
  needed — ADR-0196 already binds).

## Out of scope

- `iter.Seq` (16c)
- `slog` (16d)
- Any renaming or signature changes
- BACKLOG #46 messagelist `iter.Seq2` (17b)
