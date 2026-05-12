---
title: `make check-deep` runs gremlins with `--timeout-coefficient 10 --workers 1`
status: accepted
date: 2026-05-12
---

## Context

Pass 40.5 set out to lift `internal/mailauth` mutation efficacy
past 80%. Running `gremlins unleash -t dev ./internal/mailauth`
under the flags `scripts/check-deep.sh` shipped with (no
`--timeout-coefficient`, default workers) produced this verdict
distribution:

- Killed 0, Lived 0, Not covered 7, Timed out 101 → efficacy 0%,
  mutator coverage 0%.

Every mutant timed out. The "76% baseline (Pass 40.3)" recorded
in the script's header comment was measured *with* the flags
Pass 40.4 happened to pass on the command line
(`--timeout-coefficient 10 --workers 1`), not with the flags the
script itself uses. The script and the recorded baselines were
out of sync. Under the script's own flags, no package's floor
was meaningful.

Re-running with `--timeout-coefficient 10 --workers 1` and the
seven new tests added by this pass — `TestDefaultDevicePoll
Interval`, `TestXOAuth2_NextDecodesError`, `TestXOAuth2_Next
RejectsBadJSON`, `TestConsentServer_PassesError`,
`TestConsentServer_PassesCode`, `TestListen_ScansPortRange`,
`TestListen_ExhaustedRange` — yields:

- Killed 84, Lived 23, Not covered 1, Timed out 0 → efficacy
  78.50%, mutator coverage 99.07%.

The 23 LIVED mutants are real survivors that the prior
invocation hid as TIMED_OUT, with timeouts excluded from the
denominator. The honest baseline is 78.50%, not the 76%
previously claimed.

The non-mailauth package baselines (filter, content, cache,
tidytext, mailcompose, config) carry the same risk: they were
measured under the script's old flag set and may be similarly
inflated. Pass 40.5b will re-measure each under the corrected
invocation and revise the floors.

## Decision

`scripts/check-deep.sh` runs gremlins with
`--timeout-coefficient 10 --workers 1` for every package. The
header comment records the flag rationale and the calibration
caveat. `internal/mailauth`'s floor moves to **73**
(78.50 observed − 5pp). The other seven packages keep their
existing floors until Pass 40.5b re-measures them. A stale
floor still catches regressions, even if it understates the
ceiling.

Pass 40.5 is split: this consolidation (40.5a) lands the seven
NOT_COVERED kills, the timeout-flag fix, and the calibration
record. Pass 40.5b will (i) re-measure each non-mailauth
package under the corrected flags, (ii) kill mailauth's 23 real
survivors, and (iii) revise the floors. The two phases are
distinct work — coverage-gap killing versus survivor killing —
and benefit from running with the calibrated script in hand.

## Consequences

- `make check-deep` runtime grows with the timeout coefficient.
  Mailauth alone runs ~14 minutes (108 mutants). Acceptable:
  `make check-deep` is a pass-end / nightly gate, never on the
  inner loop. Single-worker isolation avoids CPU contention
  skewing per-mutant timing.
- Future floor lifts must record both the observed efficacy and
  the invocation that produced it, so the script and the
  baselines never drift apart again.
- ADR-0234's framing (per-package observed − 5pp floors, equivalent
  mutants documented and accepted) is unchanged. This decision
  refines the *measurement protocol*, not the policy.
- ADR-0235's `internal/mail` 94.44% baseline was measured under
  the same flags (`--timeout-coefficient 10 --workers 1`), so
  that package's floor is already honest and stays put.
