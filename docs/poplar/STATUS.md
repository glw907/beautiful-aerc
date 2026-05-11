# Poplar Status

**Current pass:** Pass 16b — Mechanical Go modernization sweep.
Second of four modernization passes (16a infra ✓, 16b sweep,
16c `iter.Seq`, 16d `log/slog`) before the bubbles-adoption
remainder (17a sidebar tree, 17b messagelist on `bubbles/v2/list`,
17c help audit). Polish II (18) and v0.9.0 prep (19) follow.

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
| 14b | Wizard domain + huh UI (#27 part 2; ADR-0191) | done |
| 14c | First-run integration (#27 part 3; ADR-0192) | done |
| 14.1 | OAuth refresh (#42; ADR-0193) | done |
| 15a | `bubbles/v2/list` adoption — `movepicker`, `reader/linkpicker`, `reader/attachpicker` (ADR-0194) | done |
| 15a.5 | `compose/attachpicker` filepicker-lessons — symlink + atomic id + size column; ADR-0195 names the deviation | done |
| 16a | Claude infra: modern-Go defaults (skill bump + simplify Agent 3 + `modern-go-check.sh`; ADR-0196) | done |
| **16b** | **Mechanical Go modernization sweep — `slices.SortFunc`, `slices.Sort`, `for range N`, `sync.OnceValue`, `maps.Keys`; ~60 sites, no ADR** | **pending — next** |
| 16c | `iter.Seq` for `catkin/style.go` `walkSpans` + 3 callers; no ADR | pending |
| 16d | `log/slog` adoption + logging-convention ADR (mailjmap push-loop, error transcript shape) | pending |
| 17a | Sidebar folder hierarchy on a v2 tree component (plan + spec already on disk; ADR-0197) | pending |
| 17b | `messagelist` on `bubbles/v2/list` (custom item renderer; absorbs BACKLOG #46 `iter.Seq2`; ADR if it survives) | pending |
| 17c | `bubbles/v2/help` audit + ADRs for bubbles deviations that survive 15a/17a/17b (`helppopover`, `schedulepicker`) | pending |
| 18 | Polish II — popover dim (#14) + items surfaced during 10–17c | pending |
| 19 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 16b)

> **Goal.** Apply the mechanical modern-stdlib rewrites the
> 16a audit surfaced: `sort.SliceStable` / `sort.Slice` →
> `slices.SortStableFunc` / `slices.SortFunc` with `cmp.Or`;
> `sort.Strings`/`Ints` → `slices.Sort`; `for i := 0; i < N;
> i++` (unused `i`) → `for range N`; `sync.Once` + package var
> → `sync.OnceValue`/`OnceFunc`; manual map-key-collect+sort
> → `maps.Keys` + `slices.Sorted`. ~60 sites across ~20 files.
> No new ADR — ADR-0196 already binds the convention.
>
> **Scope.** Production + test files. Drive the working set
> from `./scripts/modern-go-check.sh` (M1–M4) plus the 16a plan
> appendix (production-only file:line list). Strict mode locally
> to verify: `MODERN_GO_STRICT=1 ./scripts/modern-go-check.sh`
> should exit 0 at the end of the pass; flip the soft-warn gate
> off in `scripts/modern-go-check.sh` only at the end of 16d
> (after `iter.Seq` + `slog` land).
>
> **Settled (do not re-brainstorm):** Multi-key sorts use
> `cmp.Or(cmp.Compare(...), cmp.Compare(...))`. Leftover
> `x := x` loop shadows get deleted as found (1.22+).
> `internal/term/font.go` collapses two package vars +
> `HasNerdFont()` to one `var HasNerdFont = sync.OnceValue(...)`.
> `internal/catkin/spellcheck.go` struct-field `once sync.Once`
> + `delIdx` becomes `OnceFunc(buildIndex)`. M3 false positives
> (loops that read `i`) get left alone — flag-only.
>
> **Out of scope:** `iter.Seq` (16c), `slog` (16d), any
> renaming or signature changes beyond the modernization itself.
> BACKLOG #46 (`messagelist` `iter.Seq2`) stays deferred to 17b.
>
> **Approach.** Brainstorm not needed — the plan is mechanical.
> Write `docs/superpowers/plans/2026-05-1X-go-modernization-sweep.md`
> grouping the ~60 hits by file and applying them in dependency
> order. Standard pass-end ritual via `poplar-pass`. Pass ships
> green: `make check` + `MODERN_GO_STRICT=1 make modern-go-check`
> both exit 0 by the end.

## Notes for the 16-series (modernization)

ADR-0196 binds the convention; 16b–d apply it. Audit appendix
in the archived 16a plan has the full file:line list.
