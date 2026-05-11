---
title: Modern Go stdlib defaults — convention bump + grep-tier check
status: accepted
date: 2026-05-10
---

## Context

Toolchain is `go 1.26.1` (go.mod `1.26.0`), but a 2026-05-10
audit of the tree turned up ~60 sites still using pre-1.21
idioms: `sort.SliceStable` everywhere (zero `slices.SortFunc`),
two `sync.Once` + variable pairs `OnceValue` would collapse,
push-callback iterators that `iter.Seq` would simplify, raw
`fmt.Fprintf(os.Stderr, ...)` for structured logging inside
`internal/mailjmap/`, and a long tail of `for i := 0; i < N;
i++` loops where the index is unused. Without a convention bump,
the queued bubbles-adoption remainder (17a sidebar tree, 17b
messagelist on `bubbles/v2/list`, 17c help audit) and Polish II
(18) would keep growing that gap — every new file would land in
the pre-1.21 dialect that 16b–d would immediately have to
rewrite.

## Decision

The `go-conventions` skill now carries a "Modern Stdlib Idioms"
section between Anti-Patterns and Project Structure. It names
the preferred form for each common pattern:

- `slices.SortFunc` / `slices.SortStableFunc` + `cmp.Or` over
  `sort.Slice` / `sort.SliceStable`; `slices.Sort` over
  `sort.Strings` / `sort.Ints`.
- `sync.OnceValue[T]` / `sync.OnceFunc` over `sync.Once` paired
  with a package-level result var.
- `iter.Seq[T]` / `iter.Seq2[K,V]` for push iterators with ≥ 2
  consumers.
- `log/slog` inside `internal/`; `fmt.Fprintln(os.Stderr, ...)`
  stays in `cmd/` for user-facing startup errors only.
- `for range N` over `for i := 0; i < N; i++` when `i` is unused.
- `maps.Keys` + `slices.Sorted` over manual collect-then-sort.
- `min` / `max` / `clear` builtins over conditional helpers.
- `cmp.Or(a, b, c)` over nil/zero-coalescing chains.
- `errors.Join(errs...)` for multi-error accumulation.
- Delete leftover `x := x` loop-variable shadows (1.22+).

`/simplify` Agent 3 (Efficiency) gets a new checklist item that
surfaces the semantic candidates (sync.Once+var, push-callback
iterators, raw stderr logging in internal, hand-rolled
`errors.Join`, nil-coalescing chains, leftover loop shadows) as
findings — not auto-fixes. Bias is precision-over-recall, matching
the voice agent.

`scripts/modern-go-check.sh` is the grep-tier counterpart to
`scripts/voice-check.sh`. It scans for the mechanical idioms
(`sort.Slice(Stable)`, `sort.Strings`/`Ints`/`Float64s`, the
three-clause `for i := 0; i < N` header, package-scope
`sync.Once`) and is wired into `make check` between `voice` and
`test`. **Soft-warn through the 16-series:** the script exits 0
even on findings, so `make check` stays green while 16b
(mechanical sweep), 16c (`iter.Seq` adoption), and 16d (`slog`)
apply the modern forms. `MODERN_GO_STRICT=1` flips to hard-fail
locally. 16d's closing step removes the soft-warn gate after the
tree is clean.

## Consequences

Three follow-up passes apply the new defaults to the existing
~60 sites the audit found:

- **16b** — mechanical sweep: `slices.SortFunc`,
  `slices.SortStableFunc` + `cmp.Or`, `slices.Sort`,
  `for range N`, `sync.OnceValue`, `maps.Keys`. No new ADR.
- **16c** — `iter.Seq` for `catkin/style.go`'s `walkSpans` + its
  three callers. BACKLOG #46 (`messagelist` `iter.Seq2`) is
  absorbed by 17b's `bubbles/v2/list` rewrite, not 16c.
- **16d** — `log/slog` adoption + a logging-convention ADR; this
  is also where the soft-warn flips to hard-fail.

New code in 17a/17b/17c/18 lands already-modern because the
skill is now invoked before every Go file edit and the
script flags any regressions during `make check`.

### Alternatives considered

- **One giant modernization pass.** Rejected: mixes mechanical
  rewrites with judgment-heavy iterator and logging work, blows
  the 8–12-task pass budget, and produces a single un-reviewable
  diff. Splitting along the four natural axes (infra → sort/loop
  → iter → slog) keeps each pass focused.
- **Rely on `/simplify` Agent 3 alone.** Rejected: grep-tier
  checks have zero false negatives, run in `make check` for
  free, and don't require an LLM to surface mechanical idioms.
  The agent handles the semantic cases the script can't see.
- **Hard-fail the script immediately.** Rejected: 60 findings on
  master means no commit can ship until 16b–d apply. Soft-warn
  surfaces the gap, makes the queue real, and lets 16a land as
  a clean infra-only commit. Hard-fail returns at the end of 16d.
