# Pass 40.5 — `internal/mailauth` mutation-efficacy lift

## Goal

Lift `internal/mailauth` mutation efficacy past 80% by covering
the seven NOT_COVERED mutants surfaced by `make check-deep` and
raise the `scripts/check-deep.sh` floor to observed − 5pp.

## Baseline (2026-05-12)

`gremlins unleash -t dev --timeout-coefficient 10 --workers 1
./internal/mailauth`:

- Killed 26, Lived 0, Not covered 7, Timed out 75, Not viable 0.
- Efficacy 100% (no surviving mutant has a verdict), but
  mutator coverage is 78.79% — the seven uncovered mutants
  drag it down. We treat coverage gaps the same as survivors
  for floor-setting: a mutation with no test pinning it is a
  test-suite gap.

### Uncovered mutants

| Site                      | Mutator               | Notes                                                  |
|---------------------------|-----------------------|--------------------------------------------------------|
| `devicecode.go:120:16`    | ARITHMETIC_BASE       | `5 * time.Second` default poll interval               |
| `loopback.go:32:59`       | ARITHMETIC_BASE       | `"oauth callback: " + e` error-path concat            |
| `loopback.go:57:33`       | CONDITIONALS_BOUNDARY | `port <= portRange[1]`                                |
| `loopback.go:57:33`       | CONDITIONALS_NEGATION | same condition                                        |
| `loopback.go:57:54`       | INCREMENT_DECREMENT   | `port++` in scan loop                                 |
| `loopback.go:59:10`       | CONDITIONALS_NEGATION | `if err == nil` inside scan                           |
| `xoauth2.go:49:55`        | CONDITIONALS_NEGATION | `if err := json.Unmarshal(...); err != nil` in `Next` |

### Timeouts

Default `gremlins` timeout is 3× base test runtime. The
`internal/mailauth` suite runs ~11s wall-clock, dominated by
`TestAuthorizeDeviceCode_SlowDownBumpsInterval` (7s real-time
sleep on the RFC 8628 5-second `slow_down` bump) and
`TestAuthorizeDeviceCode_HappyPath` (2s). Many mutants alter
timing and tip individual runs over the threshold. Pass 40.4
ran gremlins with `--timeout-coefficient 10 --workers 1` to
sidestep this; the script never adopted the flag. This pass
lifts those flags into `scripts/check-deep.sh` so `make
check-deep` reproduces the measured numbers.

## Approach

Pre-beta — no shims, no new code beyond what a test needs.

### Tasks

1. `devicecode.go` — extract the default poll interval to a
   package var `defaultDevicePollInterval = 5 * time.Second` and
   use it in `PollDeviceCode`. Add `TestDefaultDevicePollInterval`
   asserting the value equals `5 * time.Second`. Mutated to
   `5 + time.Second` (≈1s), the assertion fires.
2. `xoauth2_test.go` — add `TestXOAuth2_NextDecodesError` (valid
   JSON challenge → returns `*Xoauth2Error` with the status
   field populated) and `TestXOAuth2_NextRejectsBadJSON`
   (invalid JSON challenge → returns the unmarshal error). Pins
   both arms of the negated branch.
3. `loopback_test.go` — new file:
   - `TestConsentServer_PassesError`: spin up `runConsentServer`
     with `{0,0}`, POST `?error=access_denied`, assert the
     channel emits a non-nil `err` containing
     `"oauth callback: access_denied"`. Covers `:32` arithmetic
     and the error-branch existence.
   - `TestConsentServer_PassesCode`: POST `?code=C&state=S`,
     assert channel emits `{code: "C", state: "S"}` with nil
     err. Pairs with the error test to pin the if/else.
   - `TestListen_ScansPortRange`: occupy port P with
     `net.Listen("tcp", "127.0.0.1:0")`, then call
     `listen({P, P+1})`, assert the returned listener's port is
     `P+1`. Covers `:57` boundary + increment and `:59` negation
     (skip the busy port, accept the next).
4. `scripts/check-deep.sh` — add `--timeout-coefficient 10
   --workers 1` to the gremlins invocation; update the header
   comment block describing how baselines are measured; raise
   the `internal/mailauth` floor to the new observed efficacy
   minus 5pp.

### Equivalent mutants

None expected after coverage lift. If the post-pass run surfaces
any survivor or new uncovered mutant, document it in the ADR
under "Equivalent mutants" rather than papering over with a
weaker test.

## Verification

- `go test -tags=dev -count=1 ./internal/mailauth` green.
- `make check` green.
- `gremlins unleash -t dev --timeout-coefficient 10 --workers 1
  ./internal/mailauth` shows Not covered ≤ 1, with the new
  observed efficacy recorded in the ADR and `check-deep.sh`.

## Outcome — pass split

Re-running gremlins under the corrected flags revealed that the
prior "76% baseline" in `check-deep.sh` was inflated by 75
timed-out mutants being excluded from the efficacy denominator.
With the seven new tests in place, the honest verdict is:

- Killed 84, Lived 23, Not covered 1, Timed out 0 → efficacy
  **78.50%**, mutator coverage 99.07%.

23 real survivors emerged. Rather than carry an oversized pass,
the work splits:

- **40.5a (this pass).** Lands the seven NOT_COVERED kills, the
  `--timeout-coefficient 10 --workers 1` fix in
  `scripts/check-deep.sh`, the `internal/mailauth` floor at 73,
  and ADR-0236 documenting the calibration finding.
- **40.5b (next pass).** Re-measures the other seven packages
  under the corrected flags, kills mailauth's 23 survivors, and
  revises floors per-package.

The split matches the pass-size budget guidance (8–12 tasks /
one ADR) and the "honest baseline first, lift later" cadence
from Pass 40.3.
