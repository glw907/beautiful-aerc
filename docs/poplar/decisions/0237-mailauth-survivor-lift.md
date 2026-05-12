---
title: `internal/mailauth` mutation lift + seven-package floor recalibration
status: accepted
date: 2026-05-12
---

## Context

ADR-0236 calibrated `make check-deep` to run gremlins with
`--timeout-coefficient 10 --workers 1` and pinned `internal/mailauth`
at an honest 78.50% baseline. The 23 LIVED mutants surfaced by
that calibration were real, not artifacts. See the table in
`docs/superpowers/archive/plans/2026-05-12-mailauth-survivor-kill.md`.

This pass kills those survivors and re-measures the seven other
curated packages (filter, content, cache, tidytext, mailcompose,
config — `internal/mail` was already honest per ADR-0235) under
the same calibrated invocation, so every floor in the gate is
backed by a known number.

## Decision

`internal/mailauth` efficacy lifts to **99.07%** (Killed 106,
Lived 1, Not covered 1). Floor: **94**.

The one remaining LIVED mutant is `oauth.go:213:48` — boundary
mutation `time.Until(c.expires) > 5*time.Minute` → `>=
5*time.Minute`. The two forms differ only at exact 5-minute
equality; under any non-clock-controlled test, the boundary is
indistinguishable. The 5-minute refresh buffer is intentionally
fuzzy (a conservative cushion, not a load-bearing threshold), so
the natural fix — wiring a clock seam through `*Client` — buys
no behavioral guarantee, only a mutation-score win. Documented
as equivalent, analogous to ADR-0235's `mock.go:117` clamp.

Recalibrated floors for the remaining seven curated packages:

| Package | Observed | Floor | Prior floor |
|---------|---------:|------:|------------:|
| filter | 82.14% | 77 | 77 |
| content | 77.05% | 72 | 73 |
| mail | 94.44% | 89 | 89 |
| cache | 100.00% | 95 | 72 |
| tidytext | 79.39% | 74 | 74 |
| mailcompose | 83.20% | 78 | 78 |
| config | 83.86% | 78 | 78 |

`cache` jumped from 72 to 95: the prior 77.54% baseline was
itself an artifact of the pre-ADR-0236 flag set; the calibrated
run kills every covered mutant.

## Consequences

- `make check-deep` now runs against a gate where every per-
  package floor is backed by an honest measurement under the same
  invocation. Future regressions surface against a real baseline.
- Total wall-time grows: the seven-package recalibration ran in
  roughly 35 minutes; mailauth alone is ~17 minutes (108
  mutants). Pass-end / nightly cadence stays appropriate; the
  gate is not on the inner loop.
- The single NotCovered mutant in mailauth (1 of 108) remains.
  When a future pass cares about closing the last 0.93% of
  mutator coverage, the line will be visible in
  `gremlins unleash --output-statuses ln` output.
- Mailauth's test coverage gained ~150 lines of targeted assertions
  in `devicecode_test.go`, `loopback_test.go`, `oauth_test.go`,
  and `tokenstore_test.go`. The `tokenstore_test.go` additions
  introduce a process-global `keyring.MockInit` / `MockInitWithError`
  pattern with a `resetKeyring(t)` cleanup helper. Tests in the
  file are sequential, so the global is safe; if a future author
  adds `t.Parallel()`, the helper would need to gain a mutex.
