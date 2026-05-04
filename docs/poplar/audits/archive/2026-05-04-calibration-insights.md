# Calibration Insights — 2026-05-04

Source: single-pass calibration audit on `internal/cache/` (Phase 0.4 of
Pass 8.8). Result: 15 high-confidence findings across T1, T2, T7, T11,
T13, T22, T28. Signal-to-noise was acceptable; precision over recall held.

The exercise surfaced four catalogue refinements applied before Phase 1
dispatch.

## Refinements applied to the guide and skill

### T9 — extended to test function names

Original scope: `name:` field of a table-case struct in a sub-test.
Extension: test *function* names like
`TestQueueOp_Atomicity_FlagAppliesOptimistic` encode the same anti-
pattern at the function level. Convert to a noun-phrase suffix
(`TestQueueOp_OptimisticFlagApply`) or split into multiple smaller
tests.

### T10b — cross-function error chorus (new)

Five `migrateVn` functions each containing `fmt.Errorf("migrate vN:
%w", err)` form a uniform chorus across functions in one file. T10
and T11 don't catch this — T10 targets "failed to", T11 targets
adjacent within-function repetition. New rule: in a file where N
similarly-shaped functions all return errors of identical template,
vary at least every other one.

### T1/T2 precedence

When an unexported helper has a comment that *also* restates the code
(e.g., `// sqlPlaceholders returns "?,?,...,?" with n question
marks`), classify under T1 (the stronger rule — restatement is the
violation regardless of export status). Reserve T2 for cases where
the doc adds new information but is still unnecessary because the
name suffices.

### T22 — boundary-vs.-internal sharpening

A nil check on a struct field embedded by the constructor (e.g.,
`a.Backend == nil` after `Open` requires a non-nil backend) is T22.
A nil check on a function argument that the package's own
constructor doesn't validate is *not* T22 — it's compensating for an
absent boundary. Rule: if the zero value of the field can occur
through the package's own API surface (constructor, factory, public
method), the check is permitted. If only constructed-and-handed-off
code reaches the check, it's T22.

## Audit prompt refinements

The Phase 1.1 dispatch prompt picks these up:

- "Bias toward precision over recall" — explicit guidance lowered
  false-positive rate in calibration.
- For T13: agents should note "no caller branches on this sentinel
  per a Grep of `errors.Is`/`errors.As` against the package's
  exported error returns" before flagging. Mechanical, not gut-feel.
- For T22: agents apply the boundary-vs.-internal test before
  flagging.
- T9 explicitly covers both `name:` fields AND test function names.
- T10b explicitly covers cross-function chorus.

## Scope notes for Phase 1

- `internal/cache/` calibration found ~15 findings in a ~2,500-LOC
  package. Extrapolating naively across the project (~20K LOC under
  audit), expect 100–150 total findings. Triage in Phase 2 will
  resolve the apply/keep split.
- Zero-finding categories in calibration (T3, T4, T5, T6, T14–T19,
  T24–T26, T29, T30, T32) suggest the existing
  `CLAUDE.md` "Human voice" rules already deflected most of those
  before this audit started. Phase 1 may confirm low counts there.
