# Poplar Status

**Current pass:** Pass 2.5b-5 shipped 2026-04-25. Help popover
landed: `?` opens a centered, rounded-box modal over the account or
viewer context with the full planned keybinding vocabulary; unwired
rows render dim per the future-binding policy (option C1, ADR-0072);
modal infrastructure (root-owned bool + sentinel struct + key
stealing + view takeover) is the template for future modals
(ADR-0071). Next is Pass 2.5b-6 (error banner + spinner
consolidation).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1, 2, 2.5-render, 2.5a | Scaffold, backend, lipgloss, wireframes | done |
| 2.5b-1..3.6, 2.5b-7 | Chrome / sidebar / msglist / threading / search | done |
| 2.5b-4 | Prototype: message viewer | done |
| 2.5b-4.5 | Audit-1+2 mechanical fixes | done |
| 2.5b-5 | Prototype: help popover | done |
| 2.5b-6 | Prototype: error banner + spinner consolidation | next |
| 2.5b-train | Tooling: mailrender training capture | pending (after Pass 3) |
| 2.9 | Research: emersion vs aerc fork (BACKLOG #10) | pending |
| 3 | Wire prototype to live backend | pending |
| 6 | Triage actions (bundles toast + undo bar) | pending |
| 8 | Gmail IMAP | pending |
| 9, 9.5 | Compose + send, tidytext in compose | pending |
| 10, 11 | Config, polish | pending |
| 1.1 | Neovim --embed RPC | pending |

## Next starter prompt (Pass 2.5b-6)

> **Goal.** Capture backend errors currently dropped silently
> (mark-read in Pass 2.5b-4, body fetch, `xdg-open` in Pass 2.5b-4)
> into a coherent banner + standardize spinner placeholders. Lays
> the ground for Pass 6 toast/undo bar and Pass 9 send-progress.
>
> **Scope.** Top-anchored error banner; reusable spinner surface;
> route `ErrMsg`-style events from `mail.Backend` Cmds via App.
> Out of scope: toast + undo bar (Pass 6).
>
> **Settled:** consolidation goal (Audit-3 plan-shape); banner is
> chrome (not a modal — does not steal keys), distinct from the
> Pass 2.5b-5 modal infrastructure (ADR-0071).
>
> **Still open — brainstorm:** banner anchoring + dismissal
> (top vs above footer, auto vs Esc); styling (`ColorError` fill vs
> accent border, single vs wrapped); spinner reuse (shared model
> vs shared styles); error-stream wiring (per-Cmd `ErrMsg` vs
> dedicated channel).
>
> **Approach.** Brainstorm, plan at
> `docs/superpowers/plans/YYYY-MM-DD-error-banner-spinner.md`,
> implement. Standard pass-end checklist applies.

## Audits

Done 2026-04-25:
[invariants](audits/2026-04-25-invariants-findings.md) ·
[library packages](audits/2026-04-25-library-packages-findings.md) ·
[plan shape](audits/2026-04-25-plan-shape-findings.md).
