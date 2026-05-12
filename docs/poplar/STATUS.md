# Poplar Status

**Current pass:** Pass 40.1 closed Audit G's 21 P1 queue (ADR-0233).
Dominant fix family: per-method `*Err error` injection on six
silent-success fakes plus the matching error-path tests. Eleven
single-site assertion tightenings replaced `cmd != nil` /
SUT-derived expectations / `Render("x") != ""` checks with
type-switches, hard-coded values, and resolved-style attribute
assertions. Four real `contacts.Sync` tests against an httptest
`carddav.Handler` retired the four `t.Skip` placeholders.

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
| 40.3 | check-deep calibration + AST skipcheck (BACKLOG #59) | next |
| 41 | Audit Final — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 40.3)

> **Goal.** Run `make check-deep` for the first time against the
> post-40.1 suite, calibrate per-package mutation thresholds from
> the observed efficacy, and land the AST-based unconditional-
> `t.Skip` detector queued as BACKLOG #59.
>
> **Scope.** `scripts/check-deep.sh` for threshold wiring; a new
> `scripts/skipcheck.go` (or similar) for the AST walker. The
> calibration target per package is the observed efficacy minus
> a small buffer (5 pp); document the calibration rationale per
> package in the ADR. Detector lives in `scripts/` and runs in
> `make check` (so a future `t.Skip(...)` without `runtime.GOOS`
> guard or test-build-tag fence fails the gate).
>
> **Settled.** ADR-0232 already named the eight packages, the
> threshold mechanism, and the BACKLOG entry. Pre-beta rules
> apply.
>
> **Open.** None; calibration is mechanical against observed
> output.
>
> **Approach.** Plan doc at
> `docs/superpowers/plans/2026-05-16-check-deep-calibration.md`;
> run the matrix; write ADR-0234; standard pass-end checklist.
