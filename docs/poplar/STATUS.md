# Poplar Status

**Current pass:** Pass 36.1 — Audit C remediation. Pass 36
closed ADR-0221: Audit C walked the four Phase C focuses, applied
two inline fixes (`BeginSync`/`EndSync` deferred, audit-plan
ReportFocus focus dropped), and queued two P1 remediations
(SMTP OAuth retry, mouse-routes-to-viewer-under-compose). Live
Gmail + Outlook verification (Pass 35.1) still queued — no
OAuth client IDs on hand.

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
| 36.1 | **Audit C remediation** — SMTP OAuth retry + mouse-under-compose | next |
| 37 | **Audit D** — database | gate |
| 38 | **Audit Final** — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |
| post-1.0 | Neovim companion (#6), raw RFC822 (#21), beyond | future |

### Next starter prompt (Pass 36.1)

> **Goal.** Land Audit C's two P1 remediations so Audit D can
> proceed.
>
> **Scope.** Two findings from
> `docs/superpowers/archive/plans/2026-05-13-audit-c.md`:
> - **A (SMTP OAuth retry gap).** Mirror IMAP `dial`'s
>   `ForceRefresh`-on-`ErrAuth` retry in `mailimap.smtpAuth`;
>   drop cached `b.smtp` when `Send` returns `mail.ErrAuth` so
>   the next dial picks up the refreshed token. Test via the
>   existing fake `smtpClient` returning the AUTHENTICATIONFAILED
>   shape `classifyErr` already routes to `mail.ErrAuth`.
> - **B (mouse-routes-to-viewer-under-compose).** Close the
>   viewer on compose open (preferred) or branch
>   `App.updateMouse` on `m.compose != nil` ahead of the viewer
>   cases. Cover the chosen path with a test in `app_test.go`.
>
> **Settled:** Both findings are documented with proposed fixes
> in ADR-0221 + the archived Audit C plan. Pre-beta — no compat
> shims.
>
> **Still open — brainstorm:** A vs B in fix B (close-viewer
> vs route-on-compose). Recommendation: close-viewer-on-compose-
> open. Confirm before coding.
>
> **Approach.** Plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-audit-c-remediation.md`,
> land A and B with tests, standard pass-end checklist.
