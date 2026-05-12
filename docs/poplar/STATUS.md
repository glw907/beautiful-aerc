# Poplar Status

**Current pass:** Pass 40.3 calibrated `make check-deep` against
the post-40.1 baseline and landed `scripts/skipcheck` (ADR-0234,
closing BACKLOG #59). Per-package efficacy floors set to observed
minus a 5pp buffer: mailauth 71, content 73, filter 77, mail 64,
cache 72, tidytext 74, mailcompose 78, config 78. `internal/mail`
at 69% baseline is the queue item — `classifyErr` sentinel routing
carries the most surviving mutants. AST skipcheck wired into
`make check` between modern-go-check and test; tree clean (seven
existing skips all guarded or build-tag-fenced).

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
| 40.4 | Lift `internal/mail` efficacy past 80% | next |
| 41 | Audit Final — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 40.4)

> **Goal.** Lift `internal/mail`'s mutation efficacy past 80% by
> writing tests around the surviving mutants — `classifyErr`
> sentinel routing is the dominant cluster.
>
> **Scope.** `internal/mail/*_test.go`; the 64% threshold in
> `scripts/check-deep.sh` gets raised to observed − 5pp at
> pass-end. Out: touching the other seven packages' floors —
> each lift earns its own pass when there's enough survivor
> signal to justify the work.
>
> **Settled.** ADR-0234 calibration ratios stand. Pre-beta
> rules apply.
>
> **Open.** None — survivors are mechanical from
> `make check-deep` output.
>
> **Approach.** Run `make check-deep` against `./internal/mail`
> only, list survivors, write the missing assertions, plan doc
> at `docs/superpowers/plans/2026-05-XX-mail-mutation-lift.md`,
> standard pass-end checklist.
