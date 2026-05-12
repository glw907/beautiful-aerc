# Poplar Status

**Current pass:** Pass 36 — Audit C (feature surface). Pass 35
closed ADR-0220: `mailimap.Probe`/`ProbeSMTP` thread an optional
`*mailauth.Client`; the wizard probe builds it via
`ui/wizard.buildOAuthClient`; the config template documents the
native consent flow and `oauth2l` is gone. Live Gmail + Outlook
verification (Pass 35.1) is queued — no OAuth client IDs on hand
to run it now.

**Beta soak deferred.** Pre-beta rules apply; soak entry gated
on a full audit cycle returning no findings.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 32 | Scaffold through v2 declarative chrome (ADRs 0001–0217) | done |
| 33 | Mouse support — reader + attachments + scroll (ADR-0218) | done |
| 34 | Mouse support — sidebar + cross-pane (ADR-0219) | done |
| 35 | Native OAuth final wiring (ADR-0220) | done |
| 35.1 | Live Gmail + Outlook OAuth verification — captures + refresh rotation (Tasks 6-7 of archived plan `2026-05-11-native-oauth.md`) | pending creds |
| 36 | **Audit C** — feature surface | next |
| 37 | **Audit D** — database (schema ladder, tx boundaries, FTS5, UIDVALIDITY, on-disk shape) | gate |
| 38 | **Audit Final** — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |
| post-1.0 | Neovim companion (#6), raw RFC822 (#21), beyond | future |

### Next starter prompt (Pass 36)

> **Goal.** Audit C — feature surface. Walk every user-facing
> feature against the wireframes / keybindings / invariants and
> file findings as ADR-grade deviations, in the pattern of
> Pass 8.5 / Pass 31 / Audits A and B.
>
> **Scope.** Read-only audit. Findings land in a plan doc with
> per-finding fix recommendations. Remediation is a follow-on
> pass.
>
> **Settled (do not re-brainstorm):** Audit format follows
> ADR-0211 (Audit A remediation) and Audit B's structure —
> findings cite the grounding ADR / invariant, the drift, and
> a proposed fix.
>
> **Still open — brainstorm these:** None — pure walk-the-
> surface pass.
>
> **Approach.** Write a plan doc at
> `docs/superpowers/plans/YYYY-MM-DD-audit-c.md` listing the
> feature surfaces and file checklist. Walk in order.
> Standard pass-end checklist applies.
