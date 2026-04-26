# Poplar Status

**Current pass:** Pass 2.5b-6 shipped 2026-04-25. Error banner +
spinner consolidation landed: `ErrorMsg{Op, Err}` is the canonical
Cmd error type; `App` owns `lastErr` and renders a one-row
foreground-only banner above the status bar (ADR-0073). Banner is
chrome (does not steal keys), hidden when help short-circuits View.
Shared `NewSpinner(t)` constructor centralizes the placeholder
spinner for viewer (and future folder/send) loads (ADR-0074). Next
is Pass 3 (wire prototype to live backend).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1, 2, 2.5-render, 2.5a | Scaffold, backend, lipgloss, wireframes | done |
| 2.5b-1..3.6, 2.5b-7 | Chrome / sidebar / msglist / threading / search | done |
| 2.5b-4 | Prototype: message viewer | done |
| 2.5b-4.5 | Audit-1+2 mechanical fixes | done |
| 2.5b-5 | Prototype: help popover | done |
| 2.5b-6 | Prototype: error banner + spinner consolidation | done |
| 2.5b-train | Tooling: mailrender training capture | pending (after Pass 3) |
| 2.9 | Research: emersion vs aerc fork (BACKLOG #10) | pending |
| 3 | Wire prototype to live backend | next |
| 6 | Triage actions (bundles toast + undo bar) | pending |
| 8 | Gmail IMAP | pending |
| 9, 9.5 | Compose + send, tidytext in compose | pending |
| 10, 11 | Config, polish | pending |
| 1.1 | Neovim --embed RPC | pending |

## Next starter prompt (Pass 3)

> **Goal.** Replace the in-process mock backend with the real
> Fastmail JMAP worker so the prototype renders live mail.
>
> **Scope.** Wire `internal/mailjmap` adapter into `cmd/poplar` for
> the configured Fastmail account; load folder list + headers + body
> from the live worker; handle reconnect state and surface it via
> the existing status indicator + error banner. Out of scope: Gmail
> IMAP (Pass 8), triage actions (Pass 6), compose (Pass 9).
>
> **Settled:** the Backend interface is synchronous (ADR-0011); the
> JMAP adapter bridges the worker's async channels via a pump
> goroutine; auth uses `$FASTMAIL_API_TOKEN`; ErrorMsg + banner are
> the surface for failures (ADR-0073); spinner placeholder via
> `NewSpinner(t)` (ADR-0074).
>
> **Still open — brainstorm:** initial-load orchestration (parallel
> folder + first-folder fetch vs serial); reconnect/backoff visible
> behavior; how header pagination interacts with sidebar search;
> live mark-read failure UX (banner alone vs flag-flicker rollback).
>
> **Approach.** Brainstorm the open questions, plan at
> `docs/superpowers/plans/YYYY-MM-DD-pass-3-live-jmap.md`, then
> implement. Standard pass-end checklist applies.

## Audits

Done 2026-04-25: [invariants](audits/2026-04-25-invariants-findings.md) · [library packages](audits/2026-04-25-library-packages-findings.md) · [plan shape](audits/2026-04-25-plan-shape-findings.md).
