# Poplar Status

**Current pass:** Pass 41 ran the pre-soak Audit Final via four
parallel agents (test-infra, security/credentials, voice/doc rot,
invariant drift). Zero P0; the P1 tail is queued for Pass 41.1.
Two doc-drift findings on `mailcompose.AssembleMIME` were repaired
in `invariants.md` inline (ADR-0238).

Pass 41.1 (remediation) is the soak gate.
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
| 41 | Audit Final — comprehensive pre-soak (ADR-0238) | done |
| 41.1 | Audit Final remediation | next |
| Beta soak | Enter when 41.1 re-skim returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 41.1)

> **Goal.** Remediate the P1 findings from Audit Final (ADR-0238)
> so a re-skim returns empty and beta soak can open.
>
> **Scope.** Per `docs/superpowers/archive/plans/2026-05-12-audit-final.md`:
> three config-write temp-file races → `OpenFile(..., O_EXCL, 0o600)`;
> five fake-backend `*Err` injection seams + one IMAP-cmd-path
> `ErrAuth` end-to-end drainer test; T15 renames
> `cache.CacheEvent → cache.Event` and `compose.CacheStore → compose.Store`;
> ADR-0233–0237 em-dash density trim; six line-level voice fixes;
> three invariant doc corrections (ADR refs at `invariants.md:31`,
> internal-package list, catkin-invariants muesli/reflow provenance).
>
> **Settled.** Findings + remediation plan in ADR-0238 and the
> archived audit plan.
>
> **Open.** None — mechanical from the findings table.
>
> **Approach.** Group changes into one ADR. Standard pass-end
> checklist applies. End with a quick re-skim against the same
> agents — if empty, the next pass opens beta soak.
