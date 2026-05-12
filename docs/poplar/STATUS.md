# Poplar Status

**Current pass:** Pass 38.1 landed Audit E's three P1 findings
plus F1's free hygiene fix (ADRs 0226 outbox cap + 0227 device-
code fallback; 0193 partially superseded). Movepicker now has
Tab-toggle filter/nav mirroring ADR-0064; `[cache]
max-outbox-bytes` caps `insertFolderOp` payloads with
`ErrOutboxRowTooLarge`; OAuth wizard offers loopback or
device-code via `huh` radio and `[d]` retry affordance after a
loopback failure; new `mailauth.RequestDeviceAuth` /
`PollDeviceCode` plus an `AuthorizeDeviceCode` wrapper.

Pass 39 (Audit F — sharp edges + insecure defaults) is next.
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
| 38 / 38.1 | Audit E + remediation (ADR-0225/0226/0227) | done |
| 39 | **Audit F — sharp edges + insecure defaults** | next |
| 40 | Audit G — test assertion meaningfulness | gate |
| 41 | Audit Final — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 39)

> **Goal.** Run Audit F per `docs/poplar/audit-plan.md` §"Phase F"
> (sharp edges + insecure defaults) across the codebase, gating
> Audit G.
>
> **Scope.** Walk every package in `internal/` and the cobra
> wiring in `cmd/poplar/`. For each, look for: (a) defaults that
> are convenient but insecure (TLS skip, plaintext fallback,
> credential exposure); (b) sharp edges where the obvious API
> use has a non-obvious failure mode (off-by-one, silent
> truncation, race-on-shared-state); (c) error paths that
> swallow context. Tag findings P0/P1/P2 + already-addressed,
> following the audit-plan §"Phase F" rubric.
>
> **Settled.** Audit-plan §"Phase F" walk strategy
> (per-package parallel dispatch). Pre-beta endorses schema
> changes if a finding warrants one. Re-run Phase E if any
> Phase F remediation reshapes the ADR archive enough to
> invalidate the prior walk.
>
> **Open — brainstorm.** None; this is a pure audit pass.
>
> **Approach.** Plan at
> `docs/superpowers/plans/YYYY-MM-DD-audit-f.md`, then dispatch
> parallel audit agents per package range, aggregate findings
> into an ADR. Standard pass-end checklist.
