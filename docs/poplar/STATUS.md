# Poplar Status

**Current pass:** Pass 5 next — bubbletea conventions cleanup (#17
+ #18 + #19). Pass 2.5b-4b done — viewer completion: long-bare-URL
footnoting, `n`/`N` nav, `Tab` link picker; ADR-0085/0086/0087;
BACKLOG #22 logged for upstream parser autolink gap.

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
| 5 | Bubbletea conventions cleanup: `key.Matches` (#17) + delegation (#18) + App.View trust (#19) | next |
| 6 | Triage actions (delete/archive/star/read; toast + undo bar) | pending |
| 7 | Polish I — popover narrow-terminal (#15) + small render drift cleanup | pending |
| 8 | Gmail IMAP (direct-on-emersion rewrite) | pending |
| 9 | Compose framing: `Editor` interface, neovim `--embed` adapter, send via go-smtp | pending |
| 9.5 | Compose enhancements: Catkin native editor, tidytext (#12), content cleanup (#13) | pending |
| 10 | Config polish | pending |
| 11 | Final polish + 1.0 prep | pending |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |
| 1.1 | Neovim companion plugin (post-v1, #6) | post-v1 |

## Next starter prompt (Pass 5)

> **Goal.** Pay down bubbletea conventions debt from the Pass 4
> audit: migrate AccountTab + Viewer key dispatch to `key.Matches`
> (#17), replace zero-latency intra-model `tea.Cmd` signals with
> direct delegation (#18), and trust `AccountTab.View` line widths
> in `App.View` (#19, depends on #17).
>
> **Scope.** `internal/ui/{account_tab,viewer,app}.go` plus a new
> `AccountKeys`/`ViewerKeys` struct in `keys.go` parallel to
> `GlobalKeys`. No new components; structural cleanup only.
>
> **Settled:** Pass 4 audit findings A3/A9/A10; ADR-0080; conventions
> doc §3/§8/§10. Splits into ~3-4 commits ordered #17 → #18 → #19.
>
> **Still open — brainstorm:** binding struct slicing; App-side read
> accessors on AccountTab.
>
> **Approach.** Brainstorm, plan at
> `docs/superpowers/plans/YYYY-MM-DD-bubbletea-conventions-cleanup.md`,
> implement. Standard pass-end checklist applies.

## Audits

- 2026-04-26: [bubbletea conventions](audits/2026-04-26-bubbletea-conventions.md)
- 2026-04-25: [invariants](audits/2026-04-25-invariants-findings.md) · [library packages](audits/2026-04-25-library-packages-findings.md) · [plan shape](audits/2026-04-25-plan-shape-findings.md)
