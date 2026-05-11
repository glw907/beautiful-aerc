# Pass 25 — Small-refactor sweep

**Goal.** Mechanical net-deletion sweep ahead of Audit A. No
user-visible behavior change.

## Three changes

### 1. ansix `Measurer` (#50)

Drop the `spuaCellWidth` package global. Introduce
`ansix.Measurer` — a value type with `Width`, `SpuaCount`,
`Truncate`, `TruncateEllipsis`, `PadOrTruncate` methods. The
package-level `Width(s)` etc. and `SetSPUACellWidth`/
`SPUACellWidth` are removed entirely (pre-beta: no shims).

`cmd/poplar/root.go` builds a `Measurer` from `term.Resolve` and
passes it into `ui.NewApp`. The App stores it and threads it into
each subpackage `Model` via `New(...)`. Per-subpackage models
gain a `Measurer` field. Test setup constructs measurers directly
(no package global to mutate).

### 2. `WithLogger` collapse (amend ADR-0197)

`mailjmap.New`, `mailimap.New`, `cache.Open` drop the
`...Option`/`WithLogger` functional pattern. The slog logger
becomes a plain `*slog.Logger` parameter (nil → `slog.Default()`
with package `component` tag, matching today's behavior).

Updates: cmd/poplar wiring and the two tests that build with
`WithLogger`. The `Option` types and `WithLogger` functions are
deleted.

### 3. Fold `internal/backoff/` + `internal/humanize/`

`backoff.Exponential` is 14 lines; the four callers replicate a
small unexported `expBackoff` helper each (or inline). The
defensive `initial<=0` / `max<=0` clamps go — internal callers
pass positive constants. Delete `internal/backoff/`.

`humanize.Bytes` is 20 lines, used in three packages
(`cmd/poplar`, `internal/ui/compose`, `internal/ui/reader`). Each
gets a private `humanBytes` (or `formatBytes`) helper. Delete
`internal/humanize/`.

## Tasks

1. Introduce `ansix.Measurer`; remove package globals + free
   functions.
2. Thread `Measurer` from `cmd/poplar/root.go` into `ui.NewApp`.
3. Carry `Measurer` through App into each `internal/ui/*`
   subpackage `Model`; replace all `ansix.Width(s)` etc. with
   `m.Width(s)`. Update tests.
4. Collapse `WithLogger` in `mailjmap.New` — `*slog.Logger`
   parameter; update callers + tests.
5. Same for `mailimap.New`.
6. Same for `cache.Open`.
7. Inline `backoff.Exponential` into four callers
   (mailjmap/push.go, mailimap/idle.go, cache/drainer.go,
   cache/backfill.go); delete `internal/backoff/`.
8. Inline `humanize.Bytes` into three caller packages; delete
   `internal/humanize/`.
9. `make check` green; run `/simplify`.
10. Write ADR-0209 (Measurer + plain-logger args); amend
    ADR-0197.
11. Update invariants.md, STATUS.md.
12. Archive plan; commit, push, install.

## Pass-end checklist

Standard `poplar-pass` consolidation applies. ADR + invariants
update is required; UI surface unchanged so the §10 idiomatic-
bubbletea checklist is a quick spot-check on `App.View()`.
