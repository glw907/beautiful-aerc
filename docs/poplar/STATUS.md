# Poplar Status

**Current pass:** Pass 40 ran Audit G (test assertion
meaningfulness, ADR-0230) — 22 packages, ~185 test files,
returning 0 P0, 21 P1, 17 P2. Dominant cluster: silent-success
fakes. Also landed an inline FK fix (ADR-0231) — `upsertMessages`
now uses `RETURNING id` rather than `LastInsertId()` whose
connection-scoped semantics returned stale rowids on the UPDATE
branch of an UPSERT, intermittently FK-failing the
`message_mailboxes` link.

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
| 40.1 | **Audit G remediation — 21 P1 items** | next |
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
> walk the 21 P1 items fake-injection-first, write ADR-0232,
> standard pass-end checklist applies.
