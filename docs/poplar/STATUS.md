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

> **Goal.** Complete the index view with threaded display, per-thread
> fold state, and bulk fold/unfold.
>
> **Scope.** Thread fields on `mail.MessageInfo` (thread id, parent
> ref, depth). Mock backend grows one threaded conversation. Render
> `├─ └─ │` prefixes in the subject column in `FgDim` — document the
> new style slot(s) in `docs/poplar/styling.md` before writing
> renderer code. Per-thread fold state on `MessageList` with `Space`
> (fold-toggle), `F` (fold-all), `U` (unfold-all). `j/k` skips hidden
> children. Collapsed thread shows `[N]` count badge in `fg_dim`
> before the subject. Consume the `[ui.folders.<name>] threading`
> field Pass 2.5b-3.5 parsed. Thread roots sort by the folder's
> existing sort order; children always chronological ascending.
> Footer gains the fold hint; `keybindings.md` promotes
> `Space`/`F`/`U` from reserved to live.
>
> **Settled (do not re-brainstorm):** `Space` fold-toggle outside
> visual mode, row-toggle inside it. `F`/`U` for bulk fold — Shift-
> Space rejected because terminals drop the modifier. No runtime
> threading toggle — config only. Threading default on globally,
> per-folder override. Thread prefixes `├─ └─ │` in `FgDim`.
>
> **Still open — brainstorm these:** sort confirmation; data model
> shape (fields on `MessageInfo` vs sibling `ThreadInfo` type);
> fold state model (per-session vs persisted, default on open);
> thread root identification (earliest vs topmost in sort order);
> mock backend threaded conversation content.
>
> **Approach.** Brainstorm the open questions, write a plan doc at
> `docs/superpowers/plans/2026-04-12-poplar-threading.md`, then
> implement. Pass-end ritual: invoke the `poplar-pass` skill.

## Tooling queue (out-of-band)

Cross-cutting subsystems built outside the pass sequence. These do
not block any pass; build them when the queue position makes sense.

- **Mailrender training capture system** — queued, not started.
  - **Spec:** `docs/superpowers/specs/2026-04-12-mailrender-training-design.md`
  - **Plan:** `docs/superpowers/plans/2026-04-13-mailrender-training.md`
  - **Builds:** `internal/train/`, `internal/content/markdown.go`,
    `cmd/poplar/train.go`, `.claude/skills/fix-corpus/` (directory
    skill with normative reference), `docs/poplar/training.md`,
    ADR 0059, plus updates to invariants/system-map/CLAUDE.md.
  - **Logical slot:** between Pass 2.5b-3.7 (sidebar filter UI)
    and Pass 2.5b-4 (message viewer). Building before the viewer
    means the viewer's `b` key reuses `internal/train.Save` from
    day one and the fix-corpus loop is already triaging the 45
    pre-migrated audit-output captures by the time viewer
    development starts surfacing new rendering bugs.
  - **Dependencies:** `internal/filter`, `internal/content`,
    `internal/mailworker/xdg` — all already shipped.
  - **Build approach:** subagent-driven-development on the plan
    above, ~26 tasks across 8 phases, fresh subagent per task.
