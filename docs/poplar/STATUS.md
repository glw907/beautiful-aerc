# Poplar Status

**Current pass:** Pass 2.5b-3.6 (threading + fold). Pivot to the
single-binary `poplar` repo landed 2026-04-12 (ADR 0058).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 | Scaffold + Fork | done |
| 2 | Backend Adapter + Connect | done |
| 2.5-render | Lipgloss migration: block model + compiled themes | done |
| 2.5-fix | Fix first-level blockquote wrapping | done |
| 2.5a | Text wireframes for all screens | done |
| 2.5b-1 | Prototype: chrome shell | done |
| 2.5b-keys | Keybinding design | done |
| 2.5b-chrome | Chrome redesign | done |
| 2.5b-2 | Prototype: sidebar | done |
| 2.5b-3 | Prototype: message list | done |
| 2.5b-3.5 | Prototype: UI config + sidebar polish | done |
| 2.5b-3.6 | Prototype: threading + fold | pending |
| 2.5b-3.7 | Prototype: sidebar filter UI | pending |
| 2.5b-train | Tooling: mailrender training capture system | pending |
| 2.5b-4 | Prototype: message viewer | pending |
| 2.5b-5 | Prototype: help popover | pending |
| 2.5b-6 | Prototype: status/toast system | pending |
| 2.5b-7 | Prototype: search | pending |
| 3 | Wire prototype to live backend | pending |
| 6 | Triage actions | pending |
| 7 | Search | pending |
| 8 | Gmail IMAP | pending |
| 9 | Compose + send (Catkin editor) | pending |
| 9.5 | Tidytext in compose | pending |
| 10 | Config | pending |
| 11 | Polish for daily use | pending |
| 1.1 | Neovim embedding (nvim --embed RPC) | pending |

## Next starter prompt (Pass 2.5b-3.6)

> **Goal.** Threaded display, per-thread fold state, bulk fold/unfold.
>
> **Approach.** Pure implementation pass — design is settled. Spec at
> `docs/superpowers/specs/2026-04-13-poplar-threading-design.md`,
> 18-task plan at
> `docs/superpowers/plans/2026-04-12-poplar-threading.md`. Execute
> via `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans`. Pass-end ritual: invoke
> `poplar-pass`.
>
> **Settled.** Inherited from ADRs 0045/0052/0053/0054 (threading
> default-on, `Space`/`F`/`U` keys, no runtime toggle) plus the
> 2026-04-13 brainstorm: latest-activity sort key, `MessageInfo`
> gains `ThreadID`/`InReplyTo` only (no wire `Depth`), Camp 2 /
> Thunderbird-style flat displayRow with transient tree, threads
> default expanded, per-session fold state reset on reload, thread
> root is the message with empty `InReplyTo` (earliest-by-date
> fallback for broken chains), 4-message branching mock thread.

## Pass 2.5b-train details

Tooling pass — not a UX prototype. Slotted before 2.5b-4 because the
viewer's `b` capture key reuses `internal/train.Save`. Spec:
`docs/superpowers/specs/2026-04-12-mailrender-training-design.md`.
Plan: `docs/superpowers/plans/2026-04-13-mailrender-training.md`
(26 tasks, 8 phases, subagent-driven).
