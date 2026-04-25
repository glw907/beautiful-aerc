# Poplar Status

**Current pass:** Pass 2.5b-4 (message viewer prototype) shipped
2026-04-25. ADRs 0065–0069 cover the viewer model, body width
correction (78→72), footnote harvesting, modifier-free
keybindings, and optimistic mark-read.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 | Scaffold + Fork | done |
| 2 | Backend Adapter + Connect | done |
| 2.5-render | Lipgloss migration | done |
| 2.5a | Text wireframes for all screens | done |
| 2.5b-1..3.6, 2.5b-7 | Chrome / sidebar / msglist / threading / search | done |
| 2.5b-4 | Prototype: message viewer | done |
| 2.5b-5 | Prototype: help popover | next |
| 2.5b-6 | Prototype: status/toast system | pending |
| 2.5b-train | Tooling: mailrender training capture system | pending (after Pass 3) |
| 2.9 | Research: JMAP/IMAP/SMTP/parser library survey | pending |
| 3 | Wire prototype to live backend | pending |
| 6 | Triage actions | pending |
| 8 | Gmail IMAP | pending |
| 9 | Compose + send (Catkin editor) | pending |
| 9.5 | Tidytext in compose | pending |
| 10 | Config | pending |
| 11 | Polish for daily use | pending |
| 1.1 | Neovim embedding (nvim --embed RPC) | pending |

## Next starter prompt (Pass 2.5b-5)

> **Goal.** Help popover prototype — `?` opens a centered modal
> overlay listing keybindings for the current context (account
> view or viewer). `?` or `Esc` closes. Dimmed content behind.
>
> **Settled.** Two contexts only (account, viewer) — see ADR 0024
> + invariants. Modifier-free bindings throughout (ADR 0068).
> Wireframes §5 has the layout (now updated to drop the obsolete
> sidebar context). Modal overlay pattern follows ADR 0065 (viewer
> model): a child of App owned at root level so it can dim every
> other surface, key routing popover-first when open.
>
> **Still open — brainstorm:** popover ownership (App vs
> AccountTab vs new HelpOverlay model); how to dim "everything
> behind" given the chrome is composed of multiple sub-models;
> whether the popover content is data-driven (single source of
> truth: the same drop-rank table the footer uses) or hand-curated
> per context; interaction with active search shelf or open
> viewer (`?` from inside either should still work, and Escape
> should pop the popover only — not also clear the search or
> close the viewer).
>
> **Approach.** Brainstorm, write spec + plan under
> `docs/superpowers/{specs,plans}/`, implement via
> `subagent-driven-development`. Pass-end via `poplar-pass`.
