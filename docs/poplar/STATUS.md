# Poplar Status

**Current pass:** Pass 40.2 landed the structural answer to
Audit G — `make check-deep` runs gremlins mutation testing
across eight curated logic-heavy packages (ADR-0232), and the
`go-conventions` skill grows an "Assertion Discipline"
subsection codifying the five anti-patterns Audit G surfaced.
Initial thresholds are 0% (informational); calibration waits
for Pass 40.3 against the post-40.1 suite. Mutation-testing
smoke on `internal/mailcompose` returned 82.28% efficacy / 14
surviving mutants — real signal at expected granularity.

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
| 40.1 | Audit G remediation — 21 P1 items | next |
| 40.2 | Mutation-testing scaffolding (ADR-0232) | done |
| 40.3 | check-deep calibration + AST skipcheck (BACKLOG #59) | queued |
| 41 | Audit Final — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 40.1)

> **Goal.** Land the 21 P1 remediations from Audit G (ADR-0230).
>
> **Scope.** Finding tables in
> `docs/superpowers/archive/plans/2026-05-14-audit-g.md` and
> ADR-0230. Dominant fix shapes: per-method error-injection on
> six fakes (`MockBackend`, `mailimap.fakeClient`,
> `mailjmap.fakeClient`, `blockingBackend`, `fakeCache`,
> `fakeContactsWriter`); tighten lipgloss assertions to
> `GetForeground()` style attribute checks; type-switch on
> returned `tea.Msg` instead of `cmd != nil`; unskip the four
> `contacts.Sync` tests against a `webdavtest` fake server.
>
> **Settled.** Remediation cadence + ADR shape from Pass 39.1.
> Triage rubric per audit-plan.md. Pre-beta rules apply.
>
> **Open.** None; mechanical remediation.
>
> **Approach.** Plan doc at
> `docs/superpowers/plans/2026-05-15-audit-g-remediation.md`,
> walk the 21 P1 items fake-injection-first, write ADR-0233,
> standard pass-end checklist applies.
