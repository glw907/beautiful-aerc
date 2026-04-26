# Poplar Status

**Current pass:** Pass 2.9 shipped 2026-04-25. Library-stack
research settled BACKLOG #10: direct-on-libraries
(`emersion/go-imap` v1 + `rockorager/go-jmap`); ADR-0075
supersedes 0002/0006/0008/0010/0012. Pass 3 reshapes around it.

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
| 2.9 | Research: emersion vs aerc fork (BACKLOG #10) | done |
| 3 | JMAP direct-on-rockorager + delete fork + wire live | next |
| 6 | Triage actions (bundles toast + undo bar) | pending |
| 8 | Gmail IMAP (direct-on-emersion rewrite) | pending |
| 9, 9.5 | Compose + send (emersion/go-smtp), tidytext in compose | pending |
| 10, 11 | Config, polish | pending |
| 1.1 | Neovim --embed RPC | pending |

## Next starter prompt (Pass 3)

> **Goal.** Rewrite `internal/mailjmap/` directly against
> `rockorager/go-jmap` (synchronous), delete `internal/mailworker/`,
> wire the prototype to live Fastmail. Per ADR-0075. Gmail IMAP
> deferred to Pass 8.
>
> **Scope.** New sync `internal/mailjmap/` on `rockorager/go-jmap`.
> Vendor fork's `auth/xoauth2.go` + `keepalive/` into
> `internal/mailauth/` with provenance comments. Delete
> `internal/mailworker/` entirely (including IMAP). Wire Fastmail
> through `App`. Address BACKLOG #11 (MIME-aware `FetchBody`).
>
> **Settled:** Library choice + sync shape (ADR-0075).
> Classification stays in `internal/mail/`. Wire types stay
> `mail.MessageInfo` / `mail.Folder` — aerc `models/` is dropped.
>
> **Open — brainstorm:** push/EventSource shape on a sync
> interface (event channel vs callback); blob/state cache for v1
> (skip? in-memory?); connection state → `●/◐/○`; large-mailbox
> Email/get pagination.
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-jmap-direct-backend.md`,
> implement. Standard pass-end checklist applies.

## Audits

Done 2026-04-25: [invariants](audits/2026-04-25-invariants-findings.md) · [library packages](audits/2026-04-25-library-packages-findings.md) · [plan shape](audits/2026-04-25-plan-shape-findings.md).
