# Poplar Status

**Current pass:** Pass 40.5b killed 22 of 23 mailauth survivors
(efficacy 78.50% → 99.07%, floor 73 → 94; one equivalent mutant
documented) and recalibrated the seven other curated package
floors under the calibrated flag set. `cache` jumped from 72 to
95 once the bad-flag artifact lifted (ADR-0237).

Pass 41 (Audit Final) is the next gate.
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
| 40.5b | mailauth survivor kill + floor recalibration (ADR-0237) | done |
| 41 | Audit Final — comprehensive pre-soak | next |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 41)

> **Goal.** Run the comprehensive pre-soak audit per
> `docs/poplar/audit-plan.md`; surface any remaining
> production-risk findings before beta-soak entry.
>
> **Scope.** Whole codebase. Output is a fresh audit doc under
> `docs/superpowers/plans/` listing P0/P1 findings with file:line
> citations and ADR pointers; no code yet.
>
> **Settled.** Audit cadence + scope live in
> `docs/poplar/audit-plan.md`; release-stance gate is
> "Audit Final returns empty" (CLAUDE.md).
>
> **Open.** None — the audit plan is the spec.
>
> **Approach.** Walk the audit plan section by section, log
> findings as you go, classify P0/P1/P2. The remediation pass
> (41.1) will land afterwards. Standard pass-end checklist.
