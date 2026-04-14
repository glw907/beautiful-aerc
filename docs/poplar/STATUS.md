# Poplar Status

**Current pass:** Pass 2.5b-3.7 (sidebar filter UI). Threading +
fold landed 2026-04-13 across ADRs 0059–0063.

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
| 2.5b-3.6 | Prototype: threading + fold | done |
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

## Next starter prompt (Pass 2.5b-3.7)

> **Goal.** Sidebar filter UI — incremental filter over the
> folder list for finding a folder by partial name. Single pane,
> no popover takeover.
>
> **Settled.** Group order fixed (ADR 0019); nested indent rule
> unchanged (ADR 0034); single-key bindings only (ADR 0015).
>
> **Still open — brainstorm:** activation key (`/` taken by
> message search); filter scope and match style; visual treatment
> (replace, overlay, dim); cursor ownership; Esc semantics.
>
> **Approach.** Brainstorm, write spec + plan under
> `docs/superpowers/{specs,plans}/`, implement via
> `subagent-driven-development`. Pass-end via `poplar-pass`.

## Queued: Pass 2.5b-train (mailrender training capture)

Spec `docs/superpowers/specs/2026-04-12-mailrender-training-design.md`,
plan `docs/superpowers/plans/2026-04-13-mailrender-training.md`.
