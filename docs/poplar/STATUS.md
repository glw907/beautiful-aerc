# Poplar Status

**Current pass:** Pass 37 — Audit D (database). Pass 36.1 closed
ADR-0222: SMTP `smtpAuth` now mirrors IMAP `dial`'s ForceRefresh-
on-`ErrAuth` retry, `classifyErr` covers `*gosmtp.SMTPError`
530/535/538, IMAP `dial` wraps `authenticate` through
`classifyErr` so the existing retry actually fires, and both
compose-open sites close the viewer first so mouse routing
matches the rendered surface. Live Gmail + Outlook verification
(Pass 35.1) still queued — no OAuth client IDs on hand.

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
| 37 | **Audit D** — database | next |
| 38 | **Audit Final** — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |
| post-1.0 | Neovim companion (#6), raw RFC822 (#21), beyond | future |

### Next starter prompt (Pass 37)

> **Goal.** Walk Audit D's database focuses (`docs/poplar/audit-plan.md`
> §"Phase D") read-only and classify findings P0/P1/P2. Apply P0
> inline; queue P1 for a Pass 37.1 remediation if any land.
>
> **Scope.** Cache + outbox + drainer + FTS schema and operational
> contract. Read `internal/cache/` (account.go, drainer.go, schema
> migrations, fts.go, attachments.go, outbox.go) and the `cache-`
> and `search-` invariants under `.claude/rules/`. Cross-check
> ADRs 0110–0124 + 0132–0134 + 0183 + 0184 + 0188 against current
> code.
>
> **Settled:** Pre-beta — schema work is welcomed (v1.0 freeze is
> the trigger to land schema improvements). Audit follows the
> `audit-plan.md` mechanics: P0 inline this pass, P1 to a
> remediation pass before the next audit, P2 noted only.
>
> **Still open — brainstorm:** None at start; the audit will
> surface questions per finding.
>
> **Approach.** Plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-audit-d.md` listing the
> focuses + walk surface + findings table. ADR records the audit
> outcome (P0 inline + P1 queued summary). Standard pass-end
> checklist applies.
