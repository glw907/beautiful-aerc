# Pass 40.5 — `make check-deep` gate fix + full recalibration

## Goal

Make `make check-deep` actually measure efficacy on this hardware,
guard against the false-pass case where every mutant times out,
and recalibrate all eight per-package floors against the new
parameters.

## Why this trumps a mailauth-only lift

Today's `./scripts/check-deep.sh` reports `0%` efficacy on every
one of the eight curated packages: every mutant times out under
the default `gremlins --timeout-coefficient` (= 2). Gremlins
treats `Killed=0, Lived=0` as "no measurable case" and exits 0,
so the gate is silently a no-op. The Pass 40.3 baseline numbers
(`mailauth 76%`, `content 78%`, `mail 69%` …) were captured under
different conditions and no longer reproduce.

Lifting `internal/mailauth` in isolation would write tests against
a phantom score. The gate must measure before any single-package
ratchet can ratchet anything.

## Approach

### 1. Add measurement parameters to `scripts/check-deep.sh`

Pass `--timeout-coefficient 10 --workers 1` to `gremlins unleash`.
Pass 40.4 demonstrated these flags produce stable measurements on
this workstation: `internal/mail` reports 94.44% under them; the
full eight-package run completes in roughly 20 minutes wall-time.

Workers = 1 is intentional: parallel workers split the test
budget, so a low timeout coefficient compounds the false-timeout
problem. Single-worker keeps the measurement honest at the cost of
wall-time, which is acceptable for a pass-end gate.

### 2. Guard against the all-timeouts false-pass

Wrap the gremlins call in a parser that checks the summary line.
Fail the package if `(Killed + Lived) == 0` (nothing measurable)
or if `TimedOut > 4 * (Killed + Lived)` (so many timeouts the
measured efficacy is unrepresentative). The guard goes in the
shell script itself — gremlins doesn't expose this check.

### 3. Recalibrate all eight floors

For each curated package, run `gremlins unleash -t dev
--timeout-coefficient 10 --workers 1 ./internal/<pkg>` and record
the observed efficacy. Set the floor to `observed − 5pp`.
Document any equivalent mutants the run surfaces (per ADR-0235
policy).

### 4. ADR for the recalibration

Write ADR-0236 capturing:
- The false-pass discovery and root cause.
- The new gremlins parameters and the workers=1 rationale.
- The all-timeouts guard.
- The new floor table (observed and floor for each package).
- Any equivalent mutants surfaced.

## Tasks

1. Baseline measurement: run gremlins with the new params against
   each of the eight packages, capture observed efficacy +
   surviving / equivalent mutants.
2. Update `scripts/check-deep.sh`:
   - Add the new gremlins flags.
   - Add the all-timeouts guard (post-process gremlins stdout).
   - Update the header comment block.
   - Update the per-package floor map.
3. Verify `./scripts/check-deep.sh` exits 0 with the new floors
   and exits 1 if a floor is lowered by 1pp (smoke test the
   guard).
4. Update `docs/poplar/invariants.md` if the gate description
   needs adjustment.
5. Update `docs/poplar/decisions/INDEX.md` with the ADR-0236 row.
6. Standard pass-end checklist.

## Out of scope

- New tests for any package. The pass is about measuring
  correctly. If a package's observed efficacy is now genuinely
  below the previous floor *because* the previous number was an
  artifact, that's a measurement reality, not a regression — the
  floor lowers to match. A separate follow-up pass writes tests
  if any package falls below ~70% under the new measurement.
- Per-package lift work. `mail` already lifted in Pass 40.4 and
  stays at 89; the rest will land their own ratchet passes once
  the gate is honest.

## Verification

- `./scripts/check-deep.sh` reports non-zero `Killed` for every
  package.
- Guard fires when a floor is artificially raised by 1pp above
  observed.
- `make check` green.
