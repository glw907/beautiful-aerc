# Poplar Status

**Current state:** Message list prototype complete. Hand-rolled
`MessageList` component in `internal/ui/msglist.go` with column
layout (cursor / flag / sender(22) / subject(fill) / date(12)), `▐`
cursor on the selected row, viewport scrolling
(`MoveDown`/`MoveUp`/`MoveToTop`/`MoveToBottom` plus `HalfPage` and
`Page` Up/Down — all routed through a single `moveBy` helper).
**Brightness, not hue:** read rows render in `FgDim`, unread in
`FgBright` (sender bold), the flag glyph dims with the row, and
`ColorWarning` is reserved for the single unread+flagged case. The
cursor `▐` is the only other place hue is used. Glyphs `󰈻 󰑚 󰇮`
carry the flag/answered/unread distinction; color carries the "demands
attention" signal. Codified as a general TUI rule in the
`bubbletea-design` skill ("Hue Budget") and as the poplar-specific map
in `docs/poplar/styling.md`. Folder changes via J/K refresh the
message list through `AccountTab.loadSelectedFolder` (mock-backed;
Pass 3 wires real JMAP). Single-pane key dispatch: `j/k` move
messages, `J/K/G` move folders, every key always live. Shared
`applyBg` and `fillRowToWidth` helpers extracted from sidebar and
msglist row renderers. Flag cell width pinned to **1 lipgloss cell**
— visual width vs `lipgloss.Width()` mismatch is documented inline.
Ready for message viewer prototype (Pass 2.5b-4).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 | Scaffold + Fork | done |
| 2 | Backend Adapter + Connect | done |
| 2.5-render | Lipgloss migration: block model + compiled themes | done |
| 2.5-fix | Fix first-level blockquote wrapping (BACKLOG #7) | done |
| 2.5a | Text wireframes for all screens | done |
| 2.5b-1 | Prototype: chrome shell | done |
| 2.5b-keys | Keybinding design: single-key scheme for all screens | done |
| 2.5b-chrome | Chrome redesign: drop tabs, frame, status, footer | done |
| 2.5b-2 | Prototype: sidebar | done |
| 2.5b-3 | Prototype: message list | done |
| 2.5b-3.5 | Prototype: threaded view + UI config | pending |
| 2.5b-4 | Prototype: message viewer | pending |
| 2.5b-5 | Prototype: help popover | pending |
| 2.5b-6 | Prototype: status/toast system | pending |
| 2.5b-7 | Prototype: command mode | pending |
| 3 | Wire prototype to live backend | pending |
| 6 | Triage actions | pending |
| 7 | Command mode + search | pending |
| 8 | Gmail IMAP | pending |
| 9 | Compose + send (Catkin editor, inline compose) | pending |
| 10 | Config | pending |
| 11 | Polish for daily use | pending |
| 1.1 | Neovim embedding (nvim --embed RPC) | pending |

## Plans

- [Design spec](../superpowers/specs/2026-04-09-poplar-design.md)
- [UI design spec](../superpowers/specs/2026-04-10-poplar-ui-wireframing-design.md)
- [Lipgloss migration spec](../superpowers/specs/2026-04-10-mailrender-lipgloss-design.md)
- [Lipgloss migration plan](../superpowers/plans/2026-04-10-mailrender-lipgloss.md)
- [Pass 1 plan](../superpowers/plans/2026-04-09-poplar-pass1-scaffold.md)
- [Pass 2 plan](../superpowers/plans/2026-04-09-poplar-pass2-backend-adapter.md)
- [Pass 2.5a wireframe plan](../superpowers/plans/2026-04-10-poplar-wireframes.md)
- [Pass 2.5b-1 chrome shell plan](../superpowers/plans/2026-04-10-poplar-chrome-shell.md)
- [Chrome shell design spec](../superpowers/specs/2026-04-10-poplar-chrome-shell-design.md)
- [Wireframes](../poplar/wireframes.md)
- [bubbletea-design skill spec](../superpowers/specs/2026-04-10-bubbletea-design-skill-design.md)
- [bubbletea-design skill plan](../superpowers/plans/2026-04-10-bubbletea-design-skill.md)
- [Sidebar plan](../superpowers/plans/2026-04-10-poplar-sidebar.md)
- [Chrome redesign spec](../superpowers/specs/2026-04-11-poplar-chrome-redesign-design.md)
- [Chrome redesign plan](../superpowers/plans/2026-04-11-poplar-chrome-redesign.md)
- [Keybinding map](../poplar/keybindings.md)
- [Styling reference](../poplar/styling.md)
- [Theme selection spec](../superpowers/specs/2026-04-11-poplar-themes-design.md)
- [Theme selection plan](../superpowers/plans/2026-04-11-poplar-themes.md)
- [Compose system spec](../superpowers/specs/2026-04-11-poplar-compose-design.md)

## Continuing Development

### Next steps

1. **Execute Pass 2.5b-3.5** — threaded view + UI config

### Next starter prompt

> Start Pass 2.5b-3.5: threaded message list view and the first
> piece of UI config. **Open by brainstorming** — the design has
> open questions (see below) and the user wants to settle them
> before any code goes in. Read the wireframes at
> `docs/poplar/wireframes.md` (section 3 — message list, plus
> §7 screen state #14 "Threaded View"), the architecture doc at
> `docs/poplar/architecture.md`, the keybinding map at
> `docs/poplar/keybindings.md`, and the styling reference at
> `docs/poplar/styling.md`. Aerc's per-folder
> `[ui:folder=Inbox]` `threading-enabled = true` model is the
> closest prior art and should be referenced explicitly during
> brainstorming.
>
> **Goal.** Add threaded display to the message list (the
> wireframe shows it as the default state) and the first
> `[ui]` config section so users can pick threaded vs flat per
> folder. The viewer pass (2.5b-4) is unblocked either way; this
> sub-pass exists because the wireframe shows threading and
> Pass 2.5b-3 shipped without it.
>
> **What needs to happen (subject to brainstorm refinement):**
>
> 1. **Data model** — extend `mail.MessageInfo` (or add a sibling
>    type) with `ThreadID`, parent reference, and depth so the
>    backend can express grouping. JMAP threads are native;
>    IMAP needs RFC 5256 THREAD or in-memory grouping (deferred
>    to Pass 8).
> 2. **Mock backend** — add at least one threaded conversation
>    to `internal/mail/mock.go` so the renderer can be exercised
>    without a real backend. Use the wireframe's example
>    (Frank Lee → Grace Kim → Frank Lee).
> 3. **Render** — thread prefix glyphs in the subject column
>    in `FgDim`: `├─` has-siblings, `└─` last-sibling, `│`
>    stem. Document the new style slot(s) in
>    `docs/poplar/styling.md` **before** writing renderer code
>    (per the doc-first rule).
> 4. **Fold state** — per-thread expanded/collapsed flags on
>    `MessageList`. `j/k` skip hidden children. Cursor never
>    lands on a collapsed child. Collapsed thread shows
>    `[N]` count badge in `fg_dim` before the subject (per
>    wireframes.md:515).
> 5. **Keys** — single-keypress fold operations: `zo` unfold,
>    `zc` fold, `za` toggle. **All three are two-keypress
>    sequences** — re-read the no-multikey rule
>    (`docs/poplar/architecture.md` "No multi-key sequences")
>    and resolve the contradiction in brainstorming. Either
>    pick single-keypress alternatives (`+`/`-`/`Space`?) or
>    accept that vim's `z` prefix is the one place we allow a
>    two-key chord. The wireframe assumes `z*` — the architecture
>    doc forbids it. The user needs to break the tie.
> 6. **UI config** — first `[ui]` section in
>    `~/.config/poplar/accounts.toml`. Suggested shape:
>    ```toml
>    [ui]
>    threading = true   # default for all folders
>
>    [ui.folders.Inbox]
>    threading = false  # per-folder override
>    ```
>    Add a `UIConfig` struct in `internal/config/` with a
>    `Threading bool` plus `FolderOverrides map[string]FolderUI`.
>    Wire it through `App` → `AccountTab` → `MessageList` as a
>    read-only field at construction. Document the schema in
>    `docs/poplar/architecture.md` since this is the first UI
>    config section and sets the pattern for future ones.
> 7. **Runtime toggle (optional, decide in brainstorm)** —
>    single-key (e.g. `T`) to flip the current folder between
>    threaded and flat for the session. In-memory only; doesn't
>    write back to disk. Useful for "show me this folder flat
>    right now" without editing config.
>
> **Open questions to settle in brainstorming:**
>
> - **Default value** — threaded on or off? Aerc defaults on.
>   Pine philosophy says on (modern expectation). Flat is
>   simpler for chronological folders. The user wants to decide
>   this explicitly.
> - **Granularity** — global / per-account / per-folder? The
>   recommendation above is per-folder with a global fallback,
>   but confirm before coding.
> - **Fold key conflict** — `zo`/`zc`/`za` vs the no-multikey
>   rule. Pick a side.
> - **Runtime toggle** — `T` keybinding yes/no? Is the toggle
>   per-folder, per-session, or both?
> - **Sort interaction** — threads sort by latest reply, children
>   render chronologically inside the parent. Confirm.
>
> **Approach.** Brainstorm first (settle the open questions),
> then write a short plan doc at
> `docs/superpowers/plans/2026-04-12-poplar-threading.md`,
> then implement. Standard pass-end checklist applies.

### Pass-end checklist

1. `/simplify` — code quality review
2. Update `docs/poplar/architecture.md` — design decisions
3. Update this file — mark pass done, next starter prompt
4. Update docs appropriate to the pass stage
5. Commit all changes
6. `git push`
