---
title: Small-refactor sweep — Measurer, plain-logger args, package fold
status: accepted
date: 2026-05-11
---

## Context

Pass 25 cleared the no-behavior-change deletions queued ahead of
Audit A (`docs/poplar/audit-plan.md` §Phase A):

- `internal/ansix/` carried a mutable package global
  (`spuaCellWidth`) set once at startup by `cmd/poplar`. Three
  audit-relevant problems: every test that needed a non-default
  width had to mutate the global, the `SetSPUACellWidth(1)` /
  `(2)` shape advertised reconfiguration as a runtime affordance
  (it isn't), and the package's free functions read the global
  implicitly so call sites carried no evidence of the dependency.
- `mailjmap.New`, `mailimap.New`, and `cache.Open` each took
  `...Option` with a single `WithLogger` constructor (ADR-0197).
  No second option ever materialized.
- `internal/backoff/` and `internal/humanize/` were single-
  function helper packages — each one function called from a
  handful of sites, each with its own defensive clamps shielding
  internal callers from values they never pass.

## Decision

**`ansix.Measurer`** is the new SPUA-aware width type. Constructed
once in `cmd/poplar/root.go` via `ansix.NewMeasurer(cellWidth)`
(panics on a value other than 1 or 2; zero value behaves as 1
so tests can use `var m Measurer` for ASCII-only paths) and
threaded through `ui.NewApp` into every subpackage Model that
renders icon-bearing strings. Package-level `Width`, `Truncate`,
`TruncateEllipsis`, `PadOrTruncate`, `SetSPUACellWidth`, and
`SPUACellWidth` are removed; `SpuaCount` stays as a free function
because it is cell-width-independent. `uicore.FillRowToWidth`
takes a `Measurer` parameter.

**Plain `*slog.Logger` args.** `mailjmap.New(cfg, *slog.Logger)`,
`mailimap.New(cfg, *slog.Logger)`, `mailimap.NewWithOAuth(cfg, c,
*slog.Logger)`, and `cache.Open(..., cfg Config, *slog.Logger)`
take the logger directly. nil falls back to `slog.Default()`
tagged with the package name. The `Option` types and `WithLogger`
constructors are deleted. ADR-0197 amends accordingly: the
logger seam is still threaded through `WithLogger` *in
intention*, but the syntax is a plain trailing parameter.

**Package fold.** `internal/backoff/` and `internal/humanize/`
are removed. Each former call site holds a small unexported
helper (`expBackoff` in `internal/cache/`,
`internal/mailjmap/`, `internal/mailimap/`; `humanBytes` in
`cmd/poplar/cache.go`, `internal/ui/compose/`,
`internal/ui/reader/`). The defensive `<= 0` clamps in
`backoff.Exponential` are gone; internal callers pass positive
constants and the overflow path was unreachable in practice.

## Consequences

- The Elm invariant that all state flows through models holds
  more cleanly: the SPUA-A cell-width fact is now a value field
  threaded through the tree, not a package global. Tests
  construct a Measurer per test mode rather than mutating a
  global with `defer SetSPUACellWidth(1)`.
- Three single-function packages disappear; `internal/` shrinks
  to its compose+app+mail+cache trunk without satellite leaves.
  Two duplicated `humanBytes` helpers live in
  `internal/ui/compose/` and `internal/ui/reader/` (subpackages
  cannot share); the duplication is ~20 LOC per copy, smaller
  than the package overhead it replaces.
- Tests that previously called `ansix.SetSPUACellWidth(width)`
  now construct `ansix.NewMeasurer(width)` and pass it to the
  subject under test. The seam shape changes; semantics do not.
- Audit A can run against a clean tree: no global mutability in
  `internal/ansix/`, no unused functional-option surface, no
  single-call helper packages with defensive clamps.

This pass amends ADR-0197 (slog adoption): the `WithLogger`
functional-option seam is replaced by a plain trailing
`*slog.Logger` parameter. The intent — that backend
constructors accept an explicit logger and default to the
package-tagged `slog.Default()` — is unchanged.
