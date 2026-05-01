# Poplar Status

**Current pass:** Pass 6.5 next — move-to-folder picker (`m`).
Pass 6 done — triage actions (delete/archive/star/read) with
optimistic Apply* helpers, App-owned toast + undo (`u`), visual-
mode multi-select (`v`+Space), WYSIWYG folded-thread expansion;
ADR-0089/0090.

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
| 6.5 | Move-to-folder picker (`m` modal; toast + undo) | next |
| 6.6 | Trash retention + manual empty (config knob, default 30d) | pending |
| 7 | Polish I — popover narrow-terminal (#15) + small render drift cleanup | pending |
| 8 | Gmail IMAP (direct-on-emersion rewrite) | pending |
| 9 | Compose framing: `Editor` interface, neovim `--embed` adapter, send via go-smtp | pending |
| 9.5 | Compose enhancements: Catkin native editor, tidytext (#12), content cleanup (#13) | pending |
| 10 | Config polish | pending |
| 11 | Final polish + 1.0 prep | pending |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |
| 1.1 | Neovim companion plugin (post-v1, #6) | post-v1 |

## Next starter prompt (Pass 6.5)

> **Goal.** Move-to-folder picker. Press `m`, get a modal list of
> folders, pick one, the message moves with the same toast + undo
> bar shape as Pass 6 triage.
>
> **Scope.** New `internal/ui/movepicker.go` (modal overlay,
> mirror `LinkPicker`/`HelpPopover` shape: centerOverlay +
> DimANSI + PlaceOverlay). Reuse Pass 6's `buildTriageCmd` for
> the dispatch + inverse. Hook `m` in AccountKeys.
>
> **Settled:** Optimistic mutation (ADR-0086, ADR-0089). Toast
> precedence + commit-on-folder-change (ADR-0089). ActionTargets
> mode-agnostic (ADR-0090). Overlay pattern (ADR-0082, ADR-0087).
>
> **Still open — brainstorm:** Folder list ordering (Recent? all
> folders alphabetic? sidebar order?). Filter input or just
> j/k+letter prefix? How to handle a folder name typed but not
> matching anything (no-op vs ErrorMsg vs feedback)?
>
> **Approach.** Brainstorm, plan at
> `docs/superpowers/plans/YYYY-MM-DD-move-picker.md`, implement.
> Standard pass-end checklist applies.

## Audits

- 2026-04-26: [bubbletea conventions](audits/2026-04-26-bubbletea-conventions.md)
- 2026-04-25: [invariants](audits/2026-04-25-invariants-findings.md) · [library packages](audits/2026-04-25-library-packages-findings.md) · [plan shape](audits/2026-04-25-plan-shape-findings.md)
