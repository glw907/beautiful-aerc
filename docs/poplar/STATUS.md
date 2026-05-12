# Poplar Status

**Current pass:** Pass 41.1 landed the Audit Final remediation
batch (ADR-0239): `config.AtomicWrite` collapses three temp-file
write sites with chmod-on-handle, five fake-backend `*Err` seams,
`TestCmdClient_AuthDialFailure` completes the IMAP cmd-path
ErrAuth → drainer coverage, T15 renames
(`cache.CacheEvent → Event`, `compose.CacheStore → Store`), ADR
em-dash trim, six line-level voice fixes, three invariant
doc-drift repairs.

Pass 42 opens beta soak.
Pass 35.1 still pending Gmail/Outlook creds.

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
| 41.1 | Audit Final remediation (ADR-0239) | done |
| 42 | Beta soak entry | next |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 42)

> **Goal.** Open beta soak. Audit Final returned empty after Pass
> 41.1; the soak gate is clear.
>
> **Scope.** Per `docs/poplar/release-stance.md` beta-soak rules
> (load on entry — not yet in scope here). Tag `v0.9.1` if the
> stance asks for a soak-entry version bump, otherwise leave the
> tag policy alone. Update `STATUS.md` to mark soak active and
> set the post-1.0 rules pointer.
>
> **Settled.** Audit gate cleared by ADR-0239 re-skim.
>
> **Open.** Read `docs/poplar/release-stance.md` first — it owns
> the soak-entry checklist; this pass executes that checklist.
>
> **Approach.** Load release-stance, work the checklist, write
> the soak-entry ADR. Standard pass-end checklist applies.
