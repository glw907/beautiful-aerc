# Poplar Status

**Current state:** Pass 2.5b-3.5 complete. `internal/config/`
package now holds both `AccountConfig` and the new `UIConfig` +
`LoadUI` — `internal/poplar/` is gone. Folder classification lives
in `internal/mail/classify.go` as a pure
`Classify([]Folder) []ClassifiedFolder` with a role→alias→Custom
priority ladder, verified against Gmail/Fastmail/Outlook/iCloud/
Yahoo/Proton. The sidebar consumes `[]ClassifiedFolder` +
`config.UIConfig` with rank, label, hide, and a one-space nested
indent capped at depth 3; canonical display names normalize
provider oddities like `[Gmail]/Sent Mail` → `Sent`. The JMAP
adapter stub relocated to `internal/mailjmap/` to let
`internal/config` import `internal/mail` for the classifier
without a cycle. Backend I/O is now Cmd-based: `AccountTab.Init`
returns `loadFoldersCmd`, J/K dispatches `loadFolderCmd`, and
`AccountTab` emits `FolderChangedMsg` that `App` consumes to
update the status bar without reaching through child state.
New `poplar config init` subcommand discovers folders and merges
commented `[ui.folders.<name>]` subsections into `accounts.toml`
(dry-run by default, `--write` replaces atomically, idempotent).
Dead `:` command-mode stub and its rank-0 footer hint are gone.
Ready for Pass 2.5b-3.6 (threading + fold).

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
| 2.5b-3.5 | Prototype: UI config + sidebar polish | done |
| 2.5b-3.6 | Prototype: threading + fold (index view completion) | pending |
| 2.5b-3.7 | Prototype: sidebar filter UI (UX only, no backing logic) | pending |
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

1. **Execute Pass 2.5b-3.6** — threading + fold (index view completion)
2. **Execute Pass 2.5b-3.7** — sidebar filter UI prototype (UX only)
3. **Execute Pass 2.5b-4** — message viewer prototype

### Next starter prompt (Pass 2.5b-3.6)

> After Pass 2.5b-3.5 lands, resume with Pass 2.5b-3.6: threading
> display + fold state + index-view completion. Read the
> wireframes at `docs/poplar/wireframes.md` (§3 message list,
> §7 screen state #14 threaded view), the architecture doc at
> `docs/poplar/architecture.md` (especially the threading/sidebar
> decisions, the Pass split decision, the Space-fold-key decision,
> and the F/U reservation), the keybindings doc at
> `docs/poplar/keybindings.md`, and the existing `MessageList`
> code at `internal/ui/msglist.go`. Pass 2.5b-3.5 already parses
> the threading config fields — this pass wires the consumer.
>
> **Goal.** This pass completes the index view:
>
> 1. Thread fields on `mail.MessageInfo` (thread id, parent ref,
>    depth) and the mock backend gains at least one threaded
>    conversation.
> 2. Render `├─ └─ │` prefixes in the subject column in `FgDim`.
>    Document new style slot(s) in `docs/poplar/styling.md`
>    **before** writing renderer code (doc-first rule).
> 3. Per-thread fold state on `MessageList`. Fold-toggle with
>    `Space` (outside visual mode), fold-all with `F`, unfold-all
>    with `U`. `j/k` skips hidden children. Cursor never lands on
>    a collapsed child. Collapsed thread shows `[N]` count badge
>    in `fg_dim` before the subject.
> 4. Consume the `[ui.folders.<name>] threading = ...` field that
>    Pass 2.5b-3.5 parsed. Flip threading on/off per folder.
> 5. Sort interaction: thread roots sort by the folder's existing
>    sort order (one knob), children always render chronological
>    ascending. No separate thread-activity sort.
> 6. Footer gains the fold hint; `keybindings.md` promotes
>    `Space`/`F`/`U` from reserved to live.
>
> ---
>
> **Settled (do not re-brainstorm):**
>
> - `Space` is the fold key outside visual mode; inside visual
>   mode (Pass 6) `Space` toggles row selection — disambiguated
>   by mode. See architecture.md "Thread fold key: Space, dual
>   meaning in visual-select mode".
> - `F` (fold-all) and `U` (unfold-all) are the reserved keys for
>   bulk fold, shipping in this pass. Shift-Space was rejected
>   because terminals don't send it reliably.
> - No runtime threading toggle — config only. See architecture.md
>   "Runtime threading toggle: dropped".
> - Threading default ON globally, per-folder override via the
>   `[ui.folders.<name>]` subsection 2.5b-3.5 establishes.
> - Thread prefixes use box-drawing `├─ └─ │` in `FgDim` per the
>   wireframe.
>
> ---
>
> **Still open — brainstorm these:**
>
> - **Sort interaction confirmation.** Folder's existing sort
>   setting orders thread roots (by root date), children always
>   chronological ascending. The alternative — threading overrides
>   folder sort, always uses latest-activity ordering — was
>   discussed and the one-knob model is the lean. Confirm before
>   implementing.
> - **Data model shape.** Thread id / parent ref / depth as fields
>   on `mail.MessageInfo`, or a sibling `ThreadInfo` type? JMAP
>   supplies thread info natively; IMAP needs Message-ID /
>   References header parsing (Pass 8 concern).
> - **Fold state model.** Per-session in-memory only, or
>   persisted? Default fold state when opening a folder (all
>   unfolded? previous state? all folded if threading is on?).
> - **Thread root identification.** The earliest message in the
>   thread, or the topmost in current sort order? Determines which
>   message stays visible when the thread is collapsed.
> - **Mock backend threaded conversation content.** The wireframe
>   shows Frank Lee → Grace Kim → Frank Lee; reuse that or pick
>   a different example.
>
> **Approach.** Brainstorm the open questions, then write a plan
> doc at `docs/superpowers/plans/2026-04-12-poplar-threading.md`,
> then implement. Standard pass-end checklist applies.

### Follow-up starter prompt after (Pass 2.5b-3.7)

> After Pass 2.5b-3.6 lands, resume with Pass 2.5b-3.7: sidebar
> filter UI prototype. **This pass is UI/UX only — no backing
> filter logic, no backend calls, no persistence beyond the
> filter-string field on `Sidebar`.** The match set is hardcoded
> or applied as a trivial substring match over the already-loaded
> folder slice. The point of the pass is to prototype and validate
> the *interaction* — entry key, inline input rendering, matched
> vs. dimmed row treatment, empty state, clear/exit — on a
> bounded surface before the real search work in Pass 2.5b-7 /
> Pass 7. Read the wireframes at `docs/poplar/wireframes.md`
> (§2 sidebar), the architecture doc at
> `docs/poplar/architecture.md`, the keybindings doc at
> `docs/poplar/keybindings.md`, the styling reference at
> `docs/poplar/styling.md`, and the existing sidebar code at
> `internal/ui/sidebar.go`.
>
> **Goal.** The sidebar gains a filter affordance. When active:
>
> 1. The sidebar panel shows an inline input row (probably near
>    the top, under the account name) where the user types a
>    filter string.
> 2. Folder rows whose names match the filter are rendered
>    normally; non-matching rows are either hidden or dimmed
>    (brainstorm which).
> 3. `Esc` clears the filter and exits the mode; `Enter`
>    commits the selection (opens the folder).
> 4. Group headers, indent, unread counts, and the `┃` selection
>    indicator continue to render correctly under filter.
>
> Real filter functionality (beyond a trivial substring match)
> is explicitly out of scope. Backend-backed filtering, fuzzy
> ranking, and highlight-span rendering are all later work,
> tracked separately. This pass proves the interaction.
>
> ---
>
> **Still open — brainstorm these:**
>
> - **Entry key.** `/` is earmarked for global search (footer
>   shows `/ find`). Options: reuse `/` and disambiguate by
>   context ("if sidebar has focus"), pick a different key
>   (`f`? `Ctrl-f`?), or rethink the global-search key to free
>   `/` for sidebar-local use. The one-pane, no-focus-cycling
>   architecture makes "if sidebar has focus" tricky — there's
>   no sidebar focus state to disambiguate against.
> - **Modal or inline-always?** Is the filter a mode (enter,
>   type, exit) or always visible at the top of the sidebar
>   accepting keystrokes when non-empty? Mode is simpler and
>   matches the no-clutter sidebar; always-visible is a lighter
>   touch but adds a row to the sidebar chrome.
> - **Non-match treatment.** Hide non-matching rows entirely
>   (compact list), or keep them visible but dimmed (stable
>   position, easier to re-find)?
> - **Group-header behavior under filter.** If the Primary group
>   has zero matches, does its blank-line separator collapse?
>   Does the group disappear? Or stay with an empty slot?
> - **Interaction with `J/K` and folder-jump keys.** Does `J/K`
>   move through the filtered set or the full list? Do `I/D/S/A`
>   still jump to canonical folders while the filter is active?
> - **What does "matched" rendering look like?** Bold? Accent
>   color? Character-level highlight spans? (Spans are the
>   hardest and can defer.)
> - **Scope test for 2.5b-7.** Is there a specific search-UX
>   question the sidebar prototype is meant to answer that we
>   should name now so the brainstorm stays targeted?
>
> **Approach.** Brainstorm the open questions, then write a short
> plan doc at
> `docs/superpowers/plans/2026-04-12-poplar-sidebar-filter-ui.md`,
> then implement. Standard pass-end checklist applies.

### Pass-end checklist

1. `/simplify` — code quality review
2. Update `docs/poplar/architecture.md` — design decisions
3. Update this file — mark pass done, next starter prompt
4. Update docs appropriate to the pass stage
5. Commit all changes
6. `git push`
