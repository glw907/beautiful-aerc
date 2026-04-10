# Poplar UI: Interface Inventory, Wireframing, and Keybindings

**Date:** 2026-04-10
**Status:** Design approved
**Parent spec:** `2026-04-09-poplar-design.md`

## Summary

Defines the complete UI surface of poplar: every screen, overlay,
transient element, and screen state. Establishes the wireframing
workflow (text wireframes then bubbletea prototype), vim-first
keybinding map, and design philosophy targeting idiomatic bubbletea
and showcase-quality code.

## 1. Interface Inventory

20 UI elements across 6 categories. Each element is tagged with the
pass where it first appears.

### Primary Screens

| # | Screen | Description | Pass |
|---|--------|-------------|------|
| 1 | Folder + Message List | Default view. Sidebar (folders with icons + unread counts) + message list (flags, sender, subject, date). One per account. Not closeable. | 2.5b |
| 2 | Message Viewer | Opens in a new tab on Enter. Header block + rendered body in a scrollable viewport. Closeable. | 2.5b |
| 3 | Compose (external) | Suspends bubbletea, launches `$EDITOR` via `tea.ExecProcess` (default: `micro`). Not a poplar screen — poplar disappears and reappears. | 9 |

### Overlays / Pickers

| # | Overlay | Trigger | Description | Pass |
|---|---------|---------|-------------|------|
| 4 | Compose Review | Editor exits 0 | Inline prompt: `y` send, `n` abort, `e` re-edit, `p` postpone. Renders over the message list tab. | 9 |
| 5 | Folder Picker | `:move`, `:copy` | Fuzzy-filtered list of folders. Type to narrow, Enter to select, Escape to cancel. Modal overlay centered on screen. | 7 |
| 6 | Confirm Delete | Bulk delete (3+) | Single-line prompt: "Delete N messages? y/n". Single-message delete is instant with undo. | 6 |
| 7 | Keybinding Help | `?` | Context-sensitive modal overlay showing all keybindings for the current context, organized by group in columns. | 2.5b |

### Transient UI

| # | Element | Behavior | Pass |
|---|---------|----------|------|
| 8 | Status Toast | Auto-dismissing (3s) above command footer. "Archived", "Deleted", "Message sent". Success = themed green, info = themed blue. | 6 |
| 9 | Undo Bar | Replaces toast for reversible destructive actions: "Deleted 1 message. Press u to undo" (5s window). Action deferred until window expires. | 6 |
| 10 | Error Banner | Persistent (no auto-dismiss). Red background. Connection failures, send errors, auth failures. Dismissed by keypress or condition clearing. | 6 |
| 11 | Loading Spinner | Inline spinner in message list while fetching headers. Spinner in viewer while fetching body. Uses bubbles spinner. | 2.5b |
| 12 | Connection Status | Subtle indicator in status bar: connected/reconnecting/offline. Persistent state display, not a toast. | 11 |

### Screen States

| # | State | Screen | Description | Pass |
|---|-------|--------|-------------|------|
| 13 | Empty Folder | Folder + Message List | Centered dim text: "No messages". | 2.5b |
| 14 | Threaded View | Folder + Message List | Default on. Box-drawing prefixes. Collapsible via `zo/zc/za`. | 2.5b |
| 15 | Search Results | Folder + Message List | Query shown in status area. Filtered message list. `:clear` restores. | 7 |
| 16 | Multi-Select | Folder + Message List | Visual markers on selected messages. Footer swaps to bulk actions. | 6 |
| 17 | Focused Panel | Folder + Message List | Sidebar vs message list focus. Tab cycles. Focused panel gets styled highlight. | 2.5b |

### Persistent Chrome

| # | Element | Description | Pass |
|---|---------|-------------|------|
| 18 | Tab Bar | Top of screen. Active tab highlighted. Account folder tabs permanent, viewer tabs closeable. Title = folder name or message subject. | 2.5b |
| 19 | Command Footer | Bottom of screen. Context-sensitive key hints with curated display. Changes per active tab type. Uses bubbles help component. | 2.5b |
| 20 | Status Bar | Between content and command footer. Current folder, message count, position. Toast/undo/error render here. | 2.5b |

### Bubbletea Implementation Map

| Category | Native Bubbles | Standard Pattern | Custom Component |
|----------|---------------|-----------------|-----------------|
| Spinner (#11) | `bubbles/spinner` | | |
| Viewport (#2) | `bubbles/viewport` | | |
| Command Footer (#19) | `bubbles/help` | | |
| Command Input | `bubbles/textinput` | | |
| Tab Bar (#18) | | Iota enum + child models | |
| Focus Cycling (#17) | | `focusedPanel` enum | |
| Toast (#8) | | `tea.Tick` + status string | |
| Undo Bar (#9) | | Tick + deferred action | |
| Error Banner (#10) | | Persistent string | |
| Compose (#3) | | `tea.ExecProcess` | |
| Help Popover (#7) | | State-gated overlay | |
| Sidebar (#1) | | | Icons, unread counts, selection |
| Message List (#1) | | | Columns, threading, selection |
| Multi-Select (#16) | | | Selection set + footer swap |
| Folder Picker (#5) | | | Modal + filtered list |
| Status Bar (#20) | | | Composite render line |

### Deliberately Excluded

- **Settings screen** — config files, not a TUI form
- **Address book browser** — khard via contact picker in nvim-mail
- **Attachment browser** — `:open`/`:save` commands, MIME parts via filter rendering
- **Help screen tab** — the command footer + `?` popover ARE the help
- **Folder management** — no create/delete/rename in UI (use fastmail-cli or webmail)
- **Snooze picker** — v2 candidate, needs date/time picker UI
- **Label/tag system** — Gmail-specific, not in v1

### Future: Neovim Companion Plugin

A neovim plugin for poplar is a project goal beyond v1. Potential
scope: email browsing within neovim (folder list, message list,
viewer as neovim buffers), compose integration beyond nvim-mail,
telescope pickers for folders/messages/contacts, and poplar
command passthrough. Details to be designed when the core client
is stable.

## 2. Wireframing Workflow

Two-phase approach integrated into the development process.

### Phase 1: Text Wireframes (Pass 2.5a)

Monospace character layouts in the spec for every screen in the
inventory. These are the reference drawings that define layout,
proportions, and information density. Created during brainstorming,
before any code.

### Phase 2: Bubbletea Prototype (Pass 2.5b)

A working `cmd/poplar/` binary with a mock `mail.Backend`
implementation. All UI elements navigable with sample data and
real theme colors. Validates visual design, interaction flow, and
feel in the actual terminal. Components, styles, and navigation
code graduate directly to production.

**Mock backend:** Implements `mail.Backend` with hardcoded folders,
messages, and sample rendered body. Lives in `internal/mail/mock.go`.
Stays in the codebase permanently — useful for development, testing,
and demos.

### Phase 3: Wire to Live Backend (Pass 3)

Replace mock data sources with real `mail.Backend` calls. The UI
components, styles, focus management, and navigation are already
working from the prototype.

## 3. Design Philosophy

### Better Pine, Not Better Mutt

Poplar targets users who want something highly functional and
visually attractive out of the box. It is opinionated — not
configurable in v1. Users who want maximum configurability should
use aerc or mutt. The goal is the lowest possible learning curve
for a vim-literate user.

### Idiomatic Bubbletea, Showcase Quality

Poplar should be a reference implementation the bubbletea community
points to.

**Architecture:**
- Pure Elm architecture throughout — no shortcuts, no global state,
  no goroutine hacks
- Children signal parents via `Msg` types, never method calls or
  shared pointers
- All I/O in `tea.Cmd`, never blocking in `Update` or `View`
- Clean component boundaries — each component understandable in
  isolation

**Bubbles ecosystem first:**
- Use `bubbles/viewport`, `bubbles/spinner`, `bubbles/help`,
  `bubbles/textinput`, `bubbles/key` wherever they fit
- Don't reimplement what bubbles already provides
- Custom components follow the same `Model`/`Update`/`View`/`Init`
  pattern so they could be extracted as standalone bubbles

**Lipgloss for all styling:**
- No raw ANSI escapes in UI chrome (filter output is the exception
  — it pre-renders ANSI)
- Adaptive terminal support via lipgloss's built-in detection
- Theme-to-lipgloss bridge as a clean, reusable pattern

**Code as documentation:**
- Component structure obvious from the file tree
- A bubbletea developer can open any component file and understand
  it without reading the rest of the codebase
- `internal/ui/` is the showcase — clean, well-factored,
  demonstrative of best practices

**Extractable patterns:**
- Theme-to-lipgloss bridge, tab manager, focus cycling, toast
  system — clean enough that someone could lift them into their own
  bubbletea project
- Not as libraries (overengineering), but as clear, self-contained
  patterns

**Spotlight bubbletea features:**
- Poplar should serve as a showcase for what bubbletea can do.
  When there's a choice between a plain approach and one that
  demonstrates a compelling bubbletea/lipgloss/bubbles capability,
  lean toward the showcase — as long as it serves the user experience
- Examples: lipgloss adaptive colors, half-block borders, styled
  inline ranges, `tea.ExecProcess` for editor handoff, `tea.Batch`
  for concurrent commands, viewport with scroll percentage,
  context-sensitive help via `bubbles/help` KeyMap swapping
- The goal is that someone browsing the source thinks "I want to
  build something with bubbletea" — poplar is the proof it scales
  to a real application

## 4. Enforcement

### Extended Elm Conventions

Add to `~/.claude/docs/elm-conventions.md`:

- **Rule 6: Bubbles First** — Use `bubbles/viewport`, `bubbles/spinner`,
  `bubbles/help`, `bubbles/textinput`, `bubbles/key` before building
  custom components. Document why if skipping a bubbles component.
- **Rule 7: Lipgloss Only** — No raw ANSI escape sequences in
  `internal/ui/`. All styling via lipgloss styles from the theme
  bridge. Filter output is exempt (pre-renders ANSI for viewport).
- **Rule 8: Typed Child Models** — Child `Update` returns
  `(ChildModel, tea.Cmd)`, never `(tea.Model, tea.Cmd)`. Preserves
  type safety in parent delegation.
- **Rule 9: No Cmd Discard** — Every `tea.Cmd` returned by a child
  must be collected into `tea.Batch`. Never `_ = cmd`.

### Extended Lint Hook

Add checks to `.claude/hooks/elm-architecture-lint.sh`:

- Raw ANSI escapes (`\033[`, `\x1b[`) in `internal/ui/` files
- `(tea.Model, tea.Cmd)` return signatures on child component
  `Update` methods (should be concrete types)
- Discarded cmds (`_, _ =` or `_ = cmd` patterns after child
  Update calls)

### CLAUDE.md Addition

Note that poplar code in `internal/ui/` should be written as if it's
a public bubbletea example project — clean component boundaries,
clear naming, discoverable patterns. Anyone familiar with bubbletea
should be able to read a single component file and understand it
without context.

## 5. Revised Pass Structure

| Pass | Goal | Gate |
|------|------|------|
| 1 | Scaffold + Fork | `make build` compiles *(done)* |
| 2 | Backend Adapter + Connect | `poplar` prints folder list *(done)* |
| **2.5a** | **Text wireframes** | **Wireframes in spec, reviewed and approved** |
| **2.5b** | **Bubbletea prototype (7 sub-passes, one screen at a time)** | **All screens navigable, themed, footers with keybind hints** |
| 3 | Wire to live backend | Live folders, real messages, real rendering |
| 6 | Triage actions | Delete, archive, flag with undo |
| 7 | Command mode + search | `:move`, `:search`, folder picker |
| 8 | Gmail IMAP | Gmail working alongside Fastmail |
| 9 | Compose + send | Full compose/reply/send loop |
| 10 | Config | UI settings, theme-dir, account options |
| 11 | Polish for daily use | Multi-account, reconnection, push |

**What changed from the original spec:**

- **Passes 4 and 5 collapse.** The prototype (2.5b) builds message
  list and viewer components with sample data. Pass 3 wires them to
  the real backend — no separate passes needed.
- **Keybindings move into 2.5b.** The command footer is core chrome,
  not a late-stage feature. The prototype needs a keybinding map from
  day one.
- **Pass 10 becomes Config only.** Keybindings are already wired in
  2.5b.
- **Pass 2.5b is broken into 7 sub-passes**, one screen at a time.
  Each sub-pass produces a working increment. Lessons from each
  inform the next.

### Pass 2.5b Sub-Passes

| Sub-Pass | Screen | What's Built | Gate |
|----------|--------|-------------|------|
| 2.5b-1 | Chrome shell | Tab bar, status bar, command footer, focus cycling, theme-to-lipgloss bridge. Empty content area. | Themed shell renders, Tab cycles focus, footer shows hints |
| 2.5b-2 | Sidebar | Folder list with groups, icons, unread counts, selection, `g`-prefix jumps | Navigate folders with j/k, folder jumps work, groups spaced |
| 2.5b-3 | Message list | Columns, threading, cursor, multi-select, mock data | Browse messages, thread fold/unfold, `v` select works |
| 2.5b-4 | Message viewer | Viewer tab, scrollable viewport, header block, sample rendered body | Enter opens viewer, scroll works, q closes tab |
| 2.5b-5 | Help popover | `?` overlay with context-sensitive grouped keybindings | `?` shows correct bindings per context, Escape closes |
| 2.5b-6 | Status/toast | Toast, undo bar, error banner, mock triage actions triggering them | `d`/`a` show undo bar, errors persist, toasts auto-dismiss |
| 2.5b-7 | Command mode | `:` input, tab completion, `:move` triggers folder picker overlay | Commands execute, folder picker renders and selects |

The chrome shell comes first because every other component lives
inside it. Each subsequent sub-pass adds one component to the
working shell.

### Docs Evolution

Each pass-end checklist includes documentation appropriate to the
stage:

- **Passes 1-3:** Developer-facing. Architecture decisions, component
  API, mock backend usage. Target audience: Claude and contributors.
- **Passes 6-9:** Command reference surfaces in the UI itself (command
  footer, help popover). Document keybindings and commands as
  implemented.
- **Passes 10-11:** User-facing. README, getting started, config
  reference, theme customization. Written for someone installing
  poplar for the first time.

### Pass-End Checklist

1. `/simplify`
2. Update `docs/poplar/architecture.md` — design decisions
3. Update `docs/poplar/STATUS.md` — mark pass done, next starter prompt
4. Update docs appropriate to the pass stage
5. Commit
6. `git push`

## 6. Keybinding Map

Vim-first. A vim user should feel at home immediately.

### Keybinding Groups

| Group | Bindings | Footer Display |
|-------|----------|----------------|
| Navigate | `j/k`, `gg/G`, `C-d/C-u/C-f/C-b` | Never (vim literacy assumed) |
| Read | `Enter`, `q`, `Tab` (links) | Contextual |
| Triage | `d/D`, `a/A`, `s`, `.` | Always |
| Reply | `r`, `R`, `f` | Always |
| Compose | `c` | Always |
| Search | `/`, `n/N`, `:clear` | Always (list only) |
| Select | `v`, `Space` | Contextual |
| Threads | `zo/zc/za` | Never (vim literacy assumed) |
| Go To | `gi/gd/gs/ga/gx/gt` | Never (shown in `?` popover) |
| Command | `:` | Always |

### Global Bindings

| Key | Action |
|-----|--------|
| `:` | Command mode |
| `q` | Close tab / quit if last |
| `Ctrl+C` | Force quit |
| `1-9` | Switch to tab N |
| `Tab` | Cycle focus panel |
| `c` | Compose new |
| `?` | Keybinding help popover |

### Navigation (all scrollable contexts)

| Key | Action |
|-----|--------|
| `j/k` | Down/up one |
| `Ctrl+D/Ctrl+U` | Half-page down/up |
| `Ctrl+F/Ctrl+B` | Full page down/up |
| `gg` | Jump to top |
| `G` | Jump to bottom |

### Sidebar

| Key | Action |
|-----|--------|
| `j/k` | Move cursor |
| `gg/G` | Top/bottom |
| `Enter` | Open folder |

### Message List

| Key | Action |
|-----|--------|
| `j/k` | Move cursor |
| `gg/G` | Top/bottom |
| `Enter` | Open in viewer tab |
| `d` | Delete + next |
| `D` | Delete + stay |
| `a` | Archive + next |
| `A` | Archive + stay |
| `s` | Star/flag toggle |
| `.` | Read/unread toggle |
| `r` | Reply |
| `R` | Reply all |
| `f` | Forward |
| `/` | Search |
| `n/N` | Next/prev search result |
| `v` | Toggle multi-select |
| `Space` | Select/deselect current |
| `zo/zc/za` | Unfold/fold/toggle thread |

### Message Viewer

| Key | Action |
|-----|--------|
| `j/k` | Scroll line |
| `Ctrl+D/Ctrl+U` | Half-page |
| `Ctrl+F/Ctrl+B` | Full page |
| `gg/G` | Top/bottom |
| `d` | Delete + open next |
| `a` | Archive + open next |
| `s` | Star/flag toggle |
| `r` | Reply |
| `R` | Reply all |
| `f` | Forward |
| `Tab` | Link picker |
| `q` | Close viewer |

### Command Mode

| Key | Action |
|-----|--------|
| `Enter` | Execute |
| `Escape` | Cancel |
| `Tab` | Autocomplete |

### Commands

| Command | Action |
|---------|--------|
| `:move <folder>` | Move to folder (picker if no arg) |
| `:copy <folder>` | Copy to folder |
| `:delete` | Delete selected |
| `:search <term>` | Search current folder |
| `:clear` | Clear search/filter |
| `:quit` | Quit poplar |

### Design Decisions

- **`gg` not `g` for top** — standard vim, avoids ambiguity
- **`R` for reply-all** — follows lowercase/uppercase variant pattern
  (d/D, a/A), simpler than aerc's `rr` chord
- **`zo/zc/za` for threads** — extends vim's fold idiom, as aerc
  pioneered
- **`v` for multi-select** — vim visual mode convention, more
  recognizable than mutt's `t` tag convention
- **`g` prefix is multi-purpose** — `gg` (top), `gi/gd/gs/ga/gx/gt`
  (folder jumps). Implementation uses a key sequence buffer: after
  `g`, wait for the next keypress to disambiguate. Timeout (300ms)
  is not needed — all `g` chords require a second key

## 7. Command Footer

### Display Rules

- **Always show:** Triage, Reply, Compose, Command — email-specific,
  not discoverable from vim knowledge
- **Never show:** Navigate, Threads — vim users know these
- **Show contextually:** Read (where Enter/q are relevant), Select
  (message list only), Search (message list only)

### Styling

Key in `fg_bright` (bold), separator and hint in `fg_dim`:

```
r:reply  R:all  f:fwd  d:del  a:archive  s:star  c:compose  /:search  ::cmd
```

The `:` separator and hint text are dimmed. The key character is
bright and bold. This makes keys scannable at a glance.

### Footer by Context

```
MsgList:  d:del  a:archive  s:star  r:reply  R:all  f:fwd  c:compose  /:search  ::cmd
Viewer:   d:del  a:archive  s:star  r:reply  R:all  f:fwd  Tab:links  q:close
Sidebar:  Enter:open  c:compose  ::cmd
```

### Multi-Select Footer Swap

When `v` activates multi-select, the footer changes:

```
Select:  Space:toggle  d:del all  a:archive all  v:cancel  Esc:cancel
```

## 8. Keybinding Help Popover

### Behavior

- **Trigger:** `?` in any context
- **Close:** `?` or `Escape`
- **Modal overlay** centered on screen, dimmed content behind
- **Context-sensitive** — title shows current context, content
  changes per context
- **Not scrollable** — must fit on screen

### Layout

```
╭─ Message List ──────────────────────────────╮
│                                             │
│  Navigate          Triage        Reply      │
│  j/k  up/down      d  delete     r  reply   │
│  gg   top          D  delete…    R  all      │
│  G    bottom       a  archive    f  forward  │
│  C-d  half pg dn   A  archive…   c  compose  │
│  C-u  half pg up   s  star                   │
│  C-f  page dn      .  read/unrd             │
│  C-b  page up                               │
│                                             │
│  Search           Select        Threads     │
│  /    search       v  select     zo open     │
│  n    next         ␣  toggle     zc close    │
│  N    prev                       za toggle   │
│                                             │
│  Go To                                      │
│  gi   inbox        gd drafts    gs sent      │
│  ga   archive      gx spam      gt trash     │
│                                             │
│  Enter  open       :  command    ?  close    │
│                                             │
╰─────────────────────────────────────────────╯
```

### Design Rules

- Title shows current context ("Message List", "Viewer", "Sidebar")
- Groups in columns, same grouping as keybinding map
- Key in bright, description in dim — same styling as footer
- Content changes per context
- No scrolling — too many bindings means the map needs pruning

### Implementation

State-gated overlay in root model. `showHelp bool` field. When true,
`View()` renders popover over dimmed background. All keypresses
route to help modal — only `?` and `Escape` handled. Lipgloss for
border, columns, and styling.

## 9. Folder Organization

### Default Groups

Poplar recognizes well-known folders and organizes them into three
semantic groups automatically, without configuration. The groups
exist in the data model for sort order and `g`-prefix jump bindings
but are not rendered as text headers in the sidebar — blank line
spacing between groups is sufficient since the grouping is
self-evident from the folder names and icons.

**Primary** — Inbox, Drafts, Sent, Archive
The daily email lifecycle: receive, compose, send, file.

**Disposal** — Spam, Trash
Automated cleanup and manual deletion.

**Custom** — everything else
User-created folders, mailing lists, provider-specific folders.

### Provider Name Normalization

Folders are assigned to groups by case-insensitive matching against
known aliases. Poplar displays canonical names, not provider names.

| Canonical | Fastmail | Gmail IMAP | Outlook IMAP | iCloud |
|-----------|----------|------------|-------------|--------|
| Inbox | Inbox | INBOX | Inbox | INBOX |
| Drafts | Drafts | [Gmail]/Drafts | Drafts | Drafts |
| Sent | Sent | [Gmail]/Sent Mail | Sent Items | Sent Messages |
| Archive | Archive | [Gmail]/All Mail | Archive | Archive |
| Spam | Spam | [Gmail]/Spam | Junk | Junk |
| Trash | Trash | [Gmail]/Trash | Deleted Items | Deleted Messages |

Unrecognized folders fall into the Custom group.

### Sort Order

- **Primary:** Fixed order: Inbox, Drafts, Sent, Archive
- **Disposal:** Fixed order: Spam, Trash
- **Custom:** Alphabetical, with optional `folders-sort` override
  in `accounts.toml` for users who want a specific order

### Sidebar Rendering

```
 ┃ 󰇰 Inbox                              3
   󰏫 Drafts
   󰑚 Sent
   󰀼 Archive

   󰍷 Spam                               12
   󰩺 Trash

   󰂚 Notifications
   󰑴 Remind
   󰡡 Lists/golang
```

**Styling (lipgloss):**
- **Selected row:** Full-width `bg_selection` background +
  left thick border in `accent_primary`
- **Unread counts:** Right-aligned in `accent_tertiary`, only
  shown when non-zero
- **Folder icons:** Nerd Font in `fg_base`, or `accent_tertiary`
  when folder has unread messages
- **Group separation:** Blank line between groups, no headers
- **No scrollbar** for typical folder counts — simple viewport
  clip with j/k scrolling if the list exceeds the panel height

**Implementation:** Hand-rolled component using
`lipgloss.JoinVertical`, not `bubbles/list` (which lacks native
section support). This is the idiomatic approach — Charm's own
apps use the same technique for grouped sidebars.

### Go To Bindings

`g` prefix for quick folder jumps (vim "go to" convention):

| Key | Action |
|-----|--------|
| `gi` | Go to Inbox |
| `gd` | Go to Drafts |
| `gs` | Go to Sent |
| `ga` | Go to Archive |
| `gx` | Go to Spam |
| `gt` | Go to Trash |

These are silent bindings (not in footer), shown in the `?` help
popover under a "Go To" group.
