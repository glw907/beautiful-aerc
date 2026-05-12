# Poplar Status

**Current pass:** Pass 38 — Audit E (specified vs. assumed
across the ADR archive). Pass 37.1 closed ADR-0224: schema v13
rebuilt `message_recipients` with FK CASCADE (orphan
`messages_fts` rows scrubbed free), `migrateV11` now backfills
FTS body text from cached bodies, IMAP `Changes` reads the
captured UIDVALIDITY and returns `ErrCannotCalculateChanges` on
mismatch. Live Gmail + Outlook verification (Pass 35.1) still
queued — no OAuth client IDs on hand.

**Beta soak deferred.** Pre-beta rules apply.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 32 | Scaffold through v2 declarative chrome | done |
| 33 | Mouse — reader (ADR-0218) | done |
| 34 | Mouse — sidebar + cross-pane (ADR-0219) | done |
| 35 | Native OAuth final wiring (ADR-0220) | done |
| 35.1 | Live Gmail + Outlook OAuth verification | pending creds |
| 36 | Audit C — feature surface (ADR-0221) | done |
| 36.1 | Audit C remediation (ADR-0222) | done |
| 37 | Audit D — database (ADR-0223) | done |
| 37.1 | Audit D remediation (ADR-0224) | done |
| 38 | **Audit E — specified vs. assumed (ADR walk)** | next |
| 39 | Audit F — sharp edges + insecure defaults | gate |
| 40 | Audit G — test assertion meaningfulness | gate |
| 41 | Audit Final — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 38)

> **Goal.** Run Audit E per `docs/poplar/audit-plan.md` §"Phase
> E": walk the ADR archive (`docs/poplar/decisions/`) against
> the running code and flag every binding fact that is asserted
> but unimplemented, drifted, or contradicted.
>
> **Scope.** Pure read pass — no code edits. Output is a single
> ADR (next number after 0224) cataloging findings (P0/P1/P2 per
> the audit-plan rubric) and the plan's audit walk surface
> archived under `docs/superpowers/archive/plans/`. Each finding
> cites the asserting ADR + the contradicting file:line.
>
> **Settled:** Audit walks emit one ADR + one queued remediation
> pass when P0/P1 land. Pre-beta endorses inline remediation
> only for P0; P1 lands the following pass.
>
> **Still open — brainstorm:** None. Direct audit.
>
> **Approach.** Plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-audit-e.md` listing the
> ADR groups to walk and the checking strategy per group, then
> execute. Standard pass-end checklist.
