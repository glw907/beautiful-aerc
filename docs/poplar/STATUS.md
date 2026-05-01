# Poplar Status

**Current pass:** Pass 6.6 next — Trash retention + manual empty.
Pass 6.5 done — move-to-folder picker (`m`): App-owned modal
overlay, type-to-filter, ↑↓ nav, optimistic move via shared
`buildTriageCmdWithDest`, "Moved N to <dest>" toast + undo;
ADR-0091.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1, 2, 2.5-render, 2.5a | Scaffold, backend, lipgloss, wireframes | done |
| 2.5b-1..3.6, 2.5b-7 | Chrome / sidebar / msglist / threading / search | done |
| 2.5b-4 | Prototype: message viewer | done |
| 2.5b-4.5 | Audit-1+2 mechanical fixes | done |
| 2.5b-5 | Prototype: help popover | done |
| 2.5b-6 | Prototype: error banner + spinner consolidation | done |
| 2.9 | Research: emersion vs aerc fork (BACKLOG #10) | done |
| 3 | JMAP direct-on-rockorager + delete fork + wire live | done |
| 4 | Bubbletea conventions audit + infrastructure | done — [audit](audits/2026-04-26-bubbletea-conventions.md) |
| 4.1 | Render bugfix pass — 7 findings, absorbs #14 | done |
| SPUA-policy | Three-mode iconography (auto/simple/fancy) + runtime probe | done — ADR-0084, [matrix](testing/icon-modes.md) |
| 2.5b-4b | Viewer completion: long-bare-URL footnoting + `n`/`N` nav + `Tab` link picker | done — ADR-0085/0086/0087 |
| 5 | Bubbletea conventions cleanup: `key.Matches` (#17) + delegation (#18) + App.View trust (#19) | done — ADR-0088 |
| 6 | Triage actions (delete/archive/star/read; toast + undo bar) | done — ADR-0089/0090 |
| 6.5 | Move-to-folder picker (`m` modal; toast + undo) | done — ADR-0091 |
| 6.6 | Trash retention + manual empty (config knob, default 30d) | next |
| 7 | Polish I — popover narrow-terminal (#15) + small render drift cleanup | pending |
| 8 | Gmail IMAP (direct-on-emersion rewrite) | pending |
| 9 | Compose framing: `Editor` interface, neovim `--embed` adapter, send via go-smtp | pending |
| 9.5 | Compose enhancements: Catkin native editor, tidytext (#12), content cleanup (#13) | pending |
| 10 | Config polish | pending |
| 11 | Final polish + 1.0 prep | pending |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |
| 1.1 | Neovim companion plugin (post-v1, #6) | post-v1 |

## Next starter prompt (Pass 6.6)

> **Goal.** Trash retention + manual empty. Auto-purge from Trash
> after configurable age (default 30d); add manual "empty trash."
>
> **Scope.** New `[ui] trash_retention_days` (default 30, clamp
> [0, 365]; 0 disables). Backend hook to purge old Trash on folder
> load. Confirmation-modal manual empty key on Trash view. Toast +
> local-only undo (backend commit is irreversible).
>
> **Settled:** Optimistic triage (ADR-0089/0090). Modal overlay
> (ADR-0087/0091). `[ui]` config (ADR-0053).
>
> **Still open — brainstorm:** Auto-purge trigger (on-load vs
> background sweep)? Confirmation copy + key? Apply to Spam too?
>
> **Approach.** Brainstorm, plan at
> `docs/superpowers/plans/YYYY-MM-DD-trash-retention.md`, implement.

## Audits

- 2026-04-26: [bubbletea conventions](audits/2026-04-26-bubbletea-conventions.md)
- 2026-04-25: [invariants](audits/2026-04-25-invariants-findings.md) · [library packages](audits/2026-04-25-library-packages-findings.md) · [plan shape](audits/2026-04-25-plan-shape-findings.md)
