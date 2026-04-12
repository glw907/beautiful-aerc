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
| 2.5b-7 | Prototype: search | pending |
| 3 | Wire prototype to live backend | pending |
| 6 | Triage actions | pending |
| 7 | Search | pending |
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

> Resume Pass 2.5b-3.5: threaded message list view + intelligent
> folder config + keybindings-doc cleanup. A previous brainstorm
> session settled several questions and recorded them below and
> in the keybindings / architecture / wireframe docs. **Open this
> session by brainstorming only the remaining open questions** —
> do not re-litigate the settled ones. Read the wireframes at
> `docs/poplar/wireframes.md` (section 3 — message list, plus
> §7 screen state #14 "Threaded View"), the architecture doc at
> `docs/poplar/architecture.md`, the keybinding map at
> `docs/poplar/keybindings.md`, and the styling reference at
> `docs/poplar/styling.md`. Aerc's per-folder
> `[ui:folder=Inbox]` `threading-enabled = true` model is the
> closest prior art for the per-folder override.
>
> **Goal.** Three things land in this pass, because they all
> revolve around the first `[ui]` config section and the
> sidebar/message-list surfaces that consume it:
>
> 1. **Threaded display in the message list** — the wireframe
>    shows this as the default state.
> 2. **Intelligent folder config** — auto-discover personal
>    folders, display alphabetically by default, allow the user
>    to override order, threading, and sort.
> 3. **Keybindings doc cleanup** — `docs/poplar/keybindings.md`
>    has already been partially cleaned up (drop `:` command
>    mode, mark multi-select as deferred to Pass 6). Finish the
>    cleanup as part of this pass by adding the thread-fold key
>    once it's settled.
>
> ---
>
> **Settled (do not re-brainstorm):**
>
> - **Threading default is ON globally.** Per-folder override
>   via `[ui.folders.<name>]`. Matches the wireframe, matches
>   modern client expectations (Fastmail/Apple Mail/Gmail), and
>   "Better Pine" is about UX polish, not pine defaults.
> - **Sidebar folder groups are load-bearing.** The
>   Primary / Disposal / Custom three-group structure from the
>   architecture doc stays. Ranking happens *within* a group.
>   Canonical folders keep their canonical order unless
>   explicitly reordered. Custom folders alphabetize by default;
>   user can override with explicit ranks.
> - **Nested folders render flat with a one-space indent.**
>   Folder names containing `/` (e.g. `Lists/golang`) get an
>   extra leading space so the alphabetical adjacency of
>   siblings reads as a visual group. No tree view, no
>   expand/collapse — pure render polish on top of the flat
>   data model. Tree view was explicitly rejected (see
>   architecture.md — aerc tried it and it didn't work out).
> - **`v` stays as the designed multi-select entry.** Whole
>   feature is deferred to Pass 6; `v` and `Space` are reserved
>   in the keybindings doc but marked as future. This is the
>   reason `Space` is NOT free for thread-fold toggle.
> - **`:` command mode is dropped entirely.** Every use case in
>   the wireframes has a more direct path (key or modal
>   picker). Pass 7 is now just "Search," not "Command mode +
>   search." The `: cmd` hint is out of the footer. Added to
>   the architecture doc as a design decision.
> - **`n`/`N` stay in the footer as aspirational hints** even
>   though Pass 7 search isn't wired yet. Footer philosophy is
>   "show what it will look like when done" — future hints are
>   deliberate.
> - **Keybinding doc cleanup is in scope for this pass**, not a
>   separate item. Multi-key artifacts in the wireframes
>   (`zo`/`zc`/`za`, `gg`, `gi`/`gd`/etc.) have already been
>   corrected where the replacement is settled (`gg` → `G`,
>   `gi`…`gt` → `I`/`D`/`S`/`A`/`X`/`T`). Fold keys in the
>   wireframe annotations still read "TBD" until this pass
>   picks one.
>
> ---
>
> **Still open — settle these first, then implement:**
>
> - **Thread fold key.** Candidate set narrowed to `Tab`
>   (unbound, expand/collapse convention from lazygit/k9s) or
>   `Space` (best ergonomics, file-manager convention). The
>   wrinkle: `Space` is reserved for multi-select's
>   "toggle selection on current row" action per
>   keybindings.md. Leaning toward `Tab` on collision-avoidance
>   grounds, but the user raised the "isn't Space what users
>   expect?" point and it wasn't resolved before the brainstorm
>   was paused. **This is the first question to settle.**
>   Also decide whether fold-all / unfold-all ship in this pass
>   or defer.
> - **Runtime threading toggle.** Should there be a single-key
>   runtime flip (e.g. "flat view just for this session")? The
>   prior recommendation was to drop it — config-only — on
>   YAGNI + Better Pine grounds. Not confirmed.
> - **Sort interaction.** Threads sort by latest reply, children
>   render chronologically inside the parent. Confirm this
>   matches what the user wants.
> - **Exact config schema.** Once fold keys and ordering are
>   settled, finalize the `[ui]` + `[ui.folders.<name>]` shape:
>   what fields, what key names, what types, what defaults.
>   Draft below is illustrative, not final.
> - **Data model details.** Thread ID / parent reference /
>   depth on `mail.MessageInfo` vs a sibling type. JMAP supplies
>   thread info natively; IMAP grouping is a Pass 8 concern.
>
> ---
>
> **Implementation outline (subject to brainstorm refinement):**
>
> 1. **Data model** — extend `mail.MessageInfo` (or add a
>    sibling type) with thread ID, parent reference, and depth.
> 2. **Mock backend** — add at least one threaded conversation
>    to `internal/mail/mock.go`. Use the wireframe example
>    (Frank Lee → Grace Kim → Frank Lee).
> 3. **Render** — thread prefix glyphs in the subject column
>    in `FgDim`: `├─` has-siblings, `└─` last-sibling, `│` stem.
>    Document new style slot(s) in `docs/poplar/styling.md`
>    **before** writing renderer code (doc-first rule).
> 4. **Fold state** — per-thread expanded/collapsed on
>    `MessageList`. `j/k` skip hidden children. Cursor never
>    lands on a collapsed child. Collapsed thread shows `[N]`
>    count badge in `fg_dim` before the subject
>    (wireframes.md:515).
> 5. **Config** — first `[ui]` section in
>    `~/.config/poplar/accounts.toml`. Illustrative draft:
>    ```toml
>    [ui]
>    threading = true          # global default
>    folder-order = "grouped"  # Primary / Disposal / Custom,
>                              #   alpha within Custom (default)
>
>    [ui.folders."Inbox"]
>    threading = false
>    sort = "oldest-first"
>
>    [ui.folders."Notifications"]
>    rank = 10
>    threading = false
>
>    [ui.folders."Lists/golang"]
>    rank = 1                  # pin to top of Custom group
>    ```
>    Finalize key names during brainstorm. Add a `UIConfig`
>    struct in `internal/poplar/` (or `internal/config/`),
>    wire it through `App` → `AccountTab` → `Sidebar` +
>    `MessageList` as a read-only field at construction.
>    Document the schema in `docs/poplar/architecture.md`
>    since this is the first `[ui]` section and sets the
>    pattern for future ones.
> 6. **Sidebar** — apply the folder-order policy in
>    `Sidebar.SetFolders` (group classification, within-group
>    ranking, alpha fallback). Add the one-space indent for
>    folders whose names contain `/`.
> 7. **Keybindings-doc cleanup** — add the chosen thread-fold
>    key to `keybindings.md`, update the footer examples and
>    drop-rank table, resolve the remaining `TBD` placeholders
>    in `wireframes.md` annotations.
>
> **Approach.** Brainstorm the remaining open questions first
> (start with the fold key — that's the unfinished thread),
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
