# Poplar Status

**Current pass:** Pass 40.5a covered `internal/mailauth`'s seven
NOT_COVERED mutants (coverage 78.79% → 99.07%) and fixed
`scripts/check-deep.sh` to run gremlins with
`--timeout-coefficient 10 --workers 1` (ADR-0236). The corrected
flags revealed 23 real survivors hidden as timeouts; honest
baseline 78.50%. Floor 71 → 73; other packages held for 40.5b.

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
| 40.5a | mailauth coverage + check-deep flag fix (ADR-0236) | done |
| 40.5b | Re-measure floors + kill mailauth's 23 survivors | next |
| 41 | Audit Final — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 40.5b)

> **Goal.** Kill `internal/mailauth`'s 23 real survivors and
> re-measure the seven other package floors under the calibrated
> `--timeout-coefficient 10 --workers 1` invocation (ADR-0236).
>
> **Scope.** `internal/mailauth/*_test.go` + per-package floor
> updates in `scripts/check-deep.sh`.
>
> **Settled.** ADR-0236 calibration protocol; survivor + floor
> lists come from `make check-deep` output.
>
> **Open.** None — mechanical from gremlins output.
>
> **Approach.** Run `gremlins unleash -t dev
> --timeout-coefficient 10 --workers 1 --output-statuses l`
> per package; classify; write targeted tests; plan doc at
> `docs/superpowers/plans/2026-05-XX-mailauth-survivor-kill.md`;
> standard pass-end checklist.
