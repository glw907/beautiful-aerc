# Poplar Status

**Current pass:** Pass 39.1 landed the eight P1 items from Audit
F (ADR-0228) — loopback HTTP timeouts, `smtpDial` ctx threading,
device-code `ExpiresIn` floor, cache write-back + drainer
terminal-state error surfacing, `queryFolderCmd` sync-error
propagation, `OpenStore` preferred-backend honoring, and a
`chmod 0600` mirror in `config_discover_folders`. ADR-0229
records the fixes. Audit G (Pass 40) is unblocked.

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
| 40 | **Audit G — test assertion meaningfulness** | next |
| 41 | Audit Final — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 40)

> **Goal.** Run Audit G — test assertion meaningfulness — per
> `docs/poplar/audit-plan.md` §"Phase G".
>
> **Scope.** Walk every `*_test.go` in `internal/` + `cmd/poplar/`
> looking for assertions that pass trivially: tautological cases
> (asserting a value equals itself or a constant the code under
> test just set), missing error-branch coverage (happy path only),
> mock-only assertions that don't exercise real logic, and
> assertion shapes that would survive a deliberate bug. Dispatch
> in parallel batches by package cluster as in Audit F.
>
> **Settled (do not re-brainstorm):**
> - Audit cadence and batch shape from Audit F (4 parallel
>   batches by cluster, per-batch finding tables, single
>   aggregating ADR).
> - Triage rubric P0 / P1 / P2 as defined in audit-plan.md.
> - Pre-beta rules: schema work + breaking-change findings are
>   welcomed; "would require refactor" is not a valid skip
>   rationale.
>
> **Open — brainstorm.** None; structured audit.
>
> **Approach.** Plan doc at
> `docs/superpowers/plans/2026-05-14-audit-g.md`, dispatch
> parallel batches, write ADR-0230 with the finding tally and
> remediation queue, standard pass-end checklist applies.
