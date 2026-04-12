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

1. **Execute Pass 2.5b-4** — message viewer prototype

### Next starter prompt

> Start Pass 2.5b-4: message viewer prototype. Read the wireframes
> at `docs/poplar/wireframes.md` (section 4 — message viewer), the
> architecture doc at `docs/poplar/architecture.md`, the keybinding
> map at `docs/poplar/keybindings.md`, and the styling reference at
> `docs/poplar/styling.md`. The message list (Pass 2.5b-3) is
> complete — `j/k` navigation, viewport scrolling, flag icons,
> read/unread styling, mock-backed via
> `AccountTab.loadSelectedFolder`. Add a `MessageViewer` component
> in `internal/ui/viewer.go` that opens over the right panel when
> the user presses `Enter` on the selected message. Reuse the
> existing `internal/content` package (`ParseHeaders`,
> `ParseBlocks`, `RenderHeaders`, `RenderBody`) to render the
> message body — that's already lipgloss-based. The sidebar stays
> visible; only the right panel swaps from list → viewer. `q`
> closes the viewer back to the list. Single-pane key dispatch:
> while the viewer is open, `j/k` scrolls the viewer body, not
> the list. Before adding any new styles, add them to `styling.md`
> first so the semantic role is documented alongside the palette
> assignment. Mock body content can come from extending
> `internal/mail/mock.go` with a `FetchMessage` implementation.

### Pass-end checklist

1. `/simplify` — code quality review
2. Update `docs/poplar/architecture.md` — design decisions
3. Update this file — mark pass done, next starter prompt
4. Update docs appropriate to the pass stage
5. Commit all changes
6. `git push`
