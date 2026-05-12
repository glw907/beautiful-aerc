# Poplar Status

**Current pass:** Pass 40.4 lifted `internal/mail` mutation
efficacy from 69.23% to 94.44% (ADR-0235). New tests:
`TestIsConnectionDead`, `TestConfigKey`, `TestClassifyDisposition`,
`TestIsSelfHosted`. Extended `TestMockBackend_QueryFolder` with an
`at end` boundary case + capacity assertion, and added
`TestMockBackend_FetchBody_SeededUIDs` to cover `mockBodies`.
`scripts/check-deep.sh` floor moved 64 → 89. Two equivalent
mutants (`mock.go:117` clamp, `probe.go:28` `iota+1`) documented
and accepted rather than chased — this codifies the policy.

Pass 35.1 still pending Gmail/Outlook creds.

**Beta soak deferred.** Pre-beta rules apply.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 34 | Scaffold through cross-pane mouse | done |
| 35 | Native OAuth final wiring (ADR-0220) | done |
| 35.1 | Live Gmail + Outlook OAuth verification | pending creds |
| 36 / 36.1 | Audit C + remediation (ADR-0221/0222) | done |
| 37 / 37.1 | Audit D + remediation (ADR-0223/0224) | done |
| 38 / 38.1 | Audit E + remediation (ADRs 0225/0226/0227) | done |
| 39 / 39.1 | Audit F + remediation (ADRs 0228/0229) | done |
| 40 | Audit G + FK inline fix (ADRs 0230/0231) | done |
| 40.1 | Audit G remediation — 21 P1 items (ADR-0233) | done |
| 40.2 | Mutation-testing scaffolding (ADR-0232) | done |
| 40.3 | check-deep calibration + AST skipcheck (ADR-0234) | done |
| 40.4 | `internal/mail` mutation lift to 94.44% (ADR-0235) | done |
| 40.5 | Next queue: `internal/mailauth` (76% → 80%+) | next |
| 41 | Audit Final — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 40.5)

> **Goal.** Lift `internal/mailauth` mutation efficacy past 80%
> by writing tests around the surviving and uncovered mutants
> surfaced by `make check-deep`.
>
> **Scope.** `internal/mailauth/*_test.go`; the 71% threshold in
> `scripts/check-deep.sh` gets raised to observed − 5pp at
> pass-end. Out: the other six packages' floors.
>
> **Settled.** ADRs 0232/0234/0235 establish the calibration
> shape: per-package observed − 5pp floors, document equivalent
> mutants rather than chase them.
>
> **Open.** None — survivors are mechanical from
> `make check-deep` output.
>
> **Approach.** Run gremlins against `./internal/mailauth` only,
> classify survivors, write the missing assertions, plan doc at
> `docs/superpowers/plans/2026-05-XX-mailauth-mutation-lift.md`,
> standard pass-end checklist.
