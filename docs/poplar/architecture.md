# Poplar Architecture

Living architecture record for the poplar email client. Updated after
each implementation pass with decisions made and rationale.

## Overview

Poplar is a bubbletea-based terminal email client that lives in the
beautiful-aerc monorepo. It reuses the existing filter, theme, and
compose infrastructure and adds forked aerc worker code for mail
protocol handling (IMAP + JMAP).

| Layer | Package | Purpose |
|-------|---------|---------|
| UI | `internal/ui/` | Bubbletea components (topline, sidebar, msglist, viewer, statusbar, footer) |
| Mail adapter | `internal/mail/` | `Backend` interface, poplar-native types, account lifecycle |
| Poplar config | `internal/poplar/` | AccountConfig struct, poplar-specific types |
| Forked workers | `internal/aercfork/worker/` | Forked aerc IMAP + JMAP workers |
| Forked models | `internal/aercfork/models/` | Forked aerc data types |
| Forked support | `internal/aercfork/{log,parse,xdg,auth,keepalive}/` | Forked aerc support libraries |
| Content pipeline | `internal/filter/` | CleanHTML/CleanPlain: raw email → normalized markdown |
| Block model | `internal/content/` | ParseBlocks, RenderBody, ParseHeaders, RenderHeaders |
| Themes | `internal/theme/` | Compiled lipgloss themes (15 themes, One Dark default) |
| Compose | `internal/compose/` | Existing compose buffer normalization (shared with mailrender) |
| Config | `internal/config/` | Poplar config parsing (accounts, UI, keybindings) |

## Key Decisions

### Monorepo over separate repo
**Decision:** Poplar lives in beautiful-aerc, not its own repo.
**Rationale:** Shares filter, theme, and compose code as library
calls. Avoids cross-repo version coordination during development.
Mailrender can be gradually retired as poplar subsumes its
functionality.
**Date:** 2026-04-09

### Clean fork over direct import
**Decision:** Fork aerc's worker code rather than importing aerc as
a Go dependency.
**Rationale:** Aerc doesn't maintain a stable library API — internal
packages change without warning. A fork with upstream tracking
(cherry-pick protocol fixes) is more stable than chasing breaking
`go get -u` updates.
**Date:** 2026-04-09

### External editor only
**Decision:** No built-in compose editor. Always launch `$EDITOR`
(nvim-mail) via `tea.ExecProcess`.
**Rationale:** Building an inline editor in bubbletea is a massive
effort for marginal benefit. nvim-mail already provides the exact
compose UX we want. Simplifies the UI significantly.
**Date:** 2026-04-09

### Compiled lipgloss themes over TOML
**Decision:** Themes are compiled Go values (`Palette` → `NewCompiledTheme`
→ `*CompiledTheme` with lipgloss.Style fields), not runtime TOML files.
Glamour dependency removed entirely.
**Rationale:** Follows Charm conventions (lipgloss styles as Go values).
Eliminates runtime file discovery, TOML parsing errors, and the
glamour→lipgloss impedance mismatch. Three-layer pipeline: filter
(CleanHTML/CleanPlain) → content (ParseBlocks) → renderer (RenderBody
with lipgloss). Poplar and mailrender share the same compiled themes.
**Date:** 2026-04-10 (Pass 2.5-render)

### Command footer in all tabs
**Decision:** Every tab displays a persistent footer showing available
commands grouped by function. Format: `c:compose  j/k:move  d:delete`
etc. Poplar is opinionated about keybindings — the footer is the
primary discoverability mechanism, not a help page.
**Rationale:** Terminal email clients have steep learning curves.
A visible command reference eliminates guesswork without requiring
a manual. Grouping by function (navigation, triage, compose, search)
makes the footer scannable. The footer content changes per tab context
(message list commands differ from viewer commands).
**Date:** 2026-04-09

### Status indicator for transient feedback
**Decision:** A status area displays transient messages for user
actions: "Message sent", "Draft saved", "Deleted 3 messages", etc.
Positioned above the command footer (or integrated into the tab bar
area) so it doesn't displace the persistent command hints.
**Rationale:** Without feedback, destructive or async actions feel
uncertain — did the send actually go through? A brief, auto-dismissing
status message confirms the action completed. Covers sends, drafts,
deletes, archive, moves, errors, and connection state changes.
**Implementation:** Open question — could be an inline status bar
region, a toast-style overlay, or a brief modal. Decide during
implementation based on what looks best in bubbletea. Modern TUI
convention leans toward toast overlays (bottom-right, auto-dismiss
after 2-3s) but the right choice depends on how the layout feels.
**Date:** 2026-04-09

### Fork namespace: internal/aercfork/
**Decision:** All forked aerc code lives under `internal/aercfork/`
rather than directly in `internal/`.
**Rationale:** Makes the fork boundary visible (ours vs aerc's),
simplifies cherry-picks (clear mapping to aerc source tree), and
avoids name collisions with existing `internal/` packages like
`internal/jmap/` (fastmail-cli's JMAP client).
**Date:** 2026-04-09 (Pass 1)

### Minimal AccountConfig
**Decision:** Replace aerc's 50+ field `config.AccountConfig` with
a minimal `poplar.AccountConfig` (15 fields) in `internal/poplar/`.
**Rationale:** Workers only access Name, Source, Params, Folders,
Headers, HeadersExclude, CheckMail, and identity fields. The rest
(PGP, AuthRes, CheckMailCmd, etc.) is unused by IMAP/JMAP workers
or handled differently by poplar. Smaller surface = easier to
maintain and understand.
**Date:** 2026-04-09 (Pass 1)

### Split aerc lib/ into focused packages
**Decision:** aerc's monolithic `lib/` package was split into focused
packages: `auth/` (OAuth), `keepalive/` (TCP), `xdg/` (paths),
`log/` (logging), `parse/` (headers).
**Rationale:** aerc's `lib/` is a grab-bag of unrelated utilities.
Splitting by concern makes dependencies explicit and avoids pulling
in UI-specific code (messageview, dirstore, etc.) that lives in the
same package.
**Date:** 2026-04-09 (Pass 1)

### JMAP + IMAP only (v1)
**Decision:** Target Gmail (IMAP) and Fastmail (JMAP) only. No
maildir, mbox, or notmuch backends.
**Rationale:** Covers the vast majority of real-world users. Smaller
extraction surface from aerc. Backend interface is pluggable — other
backends can be added later if there's demand.
**Date:** 2026-04-09

### Synchronous adapter over async
**Decision:** The `mail.Backend` interface uses synchronous blocking
methods. The JMAP adapter bridges the forked worker's async
message-passing (channels + callbacks) to blocking calls via a pump
goroutine.
**Rationale:** Bubbletea's `tea.Cmd` model handles async naturally —
blocking calls run in commands that return messages on completion.
Synchronous methods are simpler to reason about and test than
channel-based APIs. The pump goroutine reads from the worker's
response channel and dispatches registered callbacks; `doAction`
blocks on a per-call channel until Done/Error arrives.
**Date:** 2026-04-09 (Pass 2)

### Config in ~/.config/poplar/
**Decision:** Poplar config lives in `~/.config/poplar/accounts.toml`,
separate from aerc's config.
**Rationale:** Allows both clients to coexist during development.
The TOML format matches the theme files for consistency. Credential
resolution uses a `credential-cmd` field that executes a shell
command and injects the output into the source URL's userinfo.
**Date:** 2026-04-09 (Pass 2)

### Inherited global WorkerMessages channel
**Decision:** Keep aerc's package-level `types.WorkerMessages` channel
for now. Known limitation: multiple adapters would race on this
channel.
**Rationale:** Refactoring to per-worker channels requires changes
across the entire forked worker codebase. Single-account use (Passes
2-10) is unaffected. Will need to be addressed for multi-account
support in Pass 11.
**Date:** 2026-04-09 (Pass 2)

### Better Pine, not Better Mutt
**Decision:** Poplar is opinionated and visually attractive out of
the box. Not configurable in v1. Users who want maximum
configurability should use aerc or mutt.
**Rationale:** Lowest possible learning curve for a vim-literate
user. Pine's "it just works" philosophy with modern aesthetics.
**Date:** 2026-04-10

### Idiomatic bubbletea showcase
**Decision:** Poplar code should be reference-quality bubbletea.
When there's a choice between a plain approach and one that
demonstrates a compelling framework capability, lean toward the
showcase — as long as it serves the UX.
**Rationale:** Attract bubbletea contributors. Prove the framework
scales to a real, complex application. Extractable patterns (theme
bridge, tab manager, focus cycling, toast system) benefit the
community.
**Date:** 2026-04-10

### Vim-first keybindings
**Decision:** Single-key vim motions (j/k, G, C-d/C-u, C-f/C-b),
vim visual mode for multi-select (v). Folder jumps via command
mode. Not configurable in v1.
**Rationale:** Vim motions are the universal TUI convention (mutt,
aerc, lazygit, k9s all agree). Assuming vim literacy lets the
command footer focus on email-specific actions instead of wasting
space on navigation hints.
**Date:** 2026-04-10

### Curated command footer over full help
**Decision:** Footer shows only email-specific keybindings (triage,
reply, compose, search, command). Vim navigation and thread folding
are silent. Full reference via `?` popover.
**Rationale:** Vim users don't need to be told about j/k. Footer
real estate is precious — use it for email workflow discovery. The
`?` popover provides the complete reference when needed.
**Date:** 2026-04-10

### Folder groups without headers
**Decision:** Three folder groups (Primary, Disposal, Custom)
separated by blank lines in the sidebar. No rendered group headers.
**Rationale:** The grouping is self-evident from folder names and
icons. Headers would label the obvious and add visual noise. Groups
exist in the data model for sort order and folder jump targeting
(via command mode).
**Date:** 2026-04-10

### Provider folder name normalization
**Decision:** Poplar displays canonical folder names (Inbox, Sent,
Trash) regardless of provider naming ([Gmail]/Sent Mail, Sent Items,
etc.). Recognition via case-insensitive matching against known
aliases.
**Rationale:** Users shouldn't see provider implementation details.
Canonical names work across providers and match command mode
jump targets.
**Date:** 2026-04-10

### Hand-rolled sidebar over bubbles/list
**Decision:** Sidebar uses `lipgloss.JoinVertical` with custom
row rendering, not `bubbles/list`.
**Rationale:** `bubbles/list` lacks native section/group support.
Hand-rolled is the idiomatic approach — Charm's own apps use the
same technique for grouped sidebars. Allows full control over
selection styling (left thick border + background fill), unread
count badge alignment, and group spacing.
**Date:** 2026-04-10

### Mock backend for prototype
**Decision:** `internal/mail/mock.go` implements `mail.Backend`
with hardcoded data. Stays in the codebase permanently.
**Rationale:** Enables the prototype (Pass 2.5b) without backend
dependencies. Useful long-term for development, testing, and demos.
Pass 3 swaps mock for real JMAP adapter — no throwaway code.
**Date:** 2026-04-10

### Per-screen prototype sub-passes
**Decision:** Pass 2.5b broken into 7 sub-passes, one screen at a
time: chrome shell, sidebar, message list, viewer, help popover,
status/toast, command mode.
**Rationale:** Each screen is a learning opportunity about bubbletea
idioms. Lessons from building the sidebar inform the message list.
Incremental validation — each sub-pass produces a testable result.
**Date:** 2026-04-10

### No multi-key sequences
**Decision:** Avoid multi-key chords (e.g., `g i`, `g g`) in
bubbletea. Use single-key bindings for all actions.
**Rationale:** Bubbletea sends one `tea.KeyMsg` per keypress.
Multi-key sequences require a custom state machine (pending key
buffer, timeout logic, disambiguation). This is unnecessary
complexity — folder jumps and other multi-key actions belong in
command mode (`:go inbox`) which naturally handles multi-word
input. `G` (shift-g) for jump-to-bottom is fine since it's a
single keypress.
**Date:** 2026-04-10 (Pass 2.5b-2)

### appModel wrapper for tea.Model compliance
**Decision:** `ui.App.Update` returns `(App, tea.Cmd)` (typed, per
Elm convention). The `appModel` wrapper in `cmd/poplar/root.go`
satisfies `tea.Model`'s `(tea.Model, tea.Cmd)` return type. Similarly,
`AccountTab` has a public `Update` returning `(tea.Model, tea.Cmd)`
for the `Tab` interface, plus an internal `updateTab` returning
`(AccountTab, tea.Cmd)` for typed access in tests.
**Rationale:** Keeps the UI layer free of interface-return overhead
while satisfying bubbletea's `tea.Model` contract at the boundary.
This is the standard pattern in the bubbletea ecosystem for apps
with typed model hierarchies.
**Date:** 2026-04-10 (Pass 2.5b-1)

### Drop tabs in favor of sidebar
**Decision:** Remove the tab bar entirely. The sidebar (always
visible) shows folder context. Opening a message renders in the
right panel, not a new tab. `q` returns to the message list.
**Rationale:** With the sidebar always visible, the tab bar
provided no new information while consuming 3 rows. Simplifies
navigation — no tab lifecycle, no `1-9` switching. Aligns with
"Better Pine" philosophy (one thing at a time).
**Date:** 2026-04-11

### Three-sided frame with open left edge
**Decision:** Top `──┬──╮`, right `│`, bottom status bar
`──┴──╯`. No left border.
**Rationale:** Distinctive asymmetric frame that avoids the
junction problem at bottom-left where the status bar meets a
left border. The open left edge matches the bottom where the
grey status bar starts at column 0.
**Date:** 2026-04-11

### Account name in sidebar, switchable
**Decision:** Account name at top of sidebar, one account at a
time, key to cycle between accounts.
**Rationale:** Pine-style simplicity over stacked account trees.
When the sidebar collapses, the top frame line shows
`account · folder` for context.
**Date:** 2026-04-11

### Colorblind-accessible connection states
**Decision:** Connection states use shape + color + text: `●`
green filled (connected), `○` red hollow (offline), `◐` orange
half (reconnecting).
**Rationale:** Triple redundancy ensures accessibility across
colorblind conditions, monochrome terminals, and screen readers.
**Date:** 2026-04-11

### Footer group separators
**Decision:** `┊` (U+250A, light quadruple dash vertical) in
`fg_dim` between key groups. Custom rendering, not bubbles/help.
**Rationale:** Subtle enough to recede behind key hints, clear
enough to read groups. Spacing alone was insufficient.
**Date:** 2026-04-11

### Single-key folder jumps
**Decision:** Uppercase single keys (I/D/S/A/X/T) jump to
canonical folders from any context. Not multi-key sequences.
**Rationale:** Bubbletea sends one KeyMsg per keypress. Multi-key
sequences require a state machine. Uppercase avoids conflict with
lowercase triage keys (d/a/s).
**Date:** 2026-04-11

### Catkin: reusable built-in editor
**Decision:** Poplar's built-in compose editor is named Catkin. It lives
in `catkin/` as a standalone, importable bubbletea component with no
poplar dependencies. Email-specific features (reflow, quote handling,
tidytext, spellcheck) are layered on top by poplar's compose panel.
**Rationale:** A premier bubbletea text editor component that others
can import. Keeps the editor focused on editing, the compose panel
focused on email workflow. Extractable to its own repo if it gains
traction.
**Date:** 2026-04-11

### Catkin: Ctrl+key commands, no multi-key sequences
**Decision:** Catkin is non-modal. All commands use modifier keys
(Ctrl+key) or special keys (arrows, Home/End, PgUp/PgDn). No bare
letter commands, no multi-key sequences. One `tea.KeyMsg` = one action.
**Rationale:** Idiomatic bubbletea. Catkin is always in insert mode —
bare keys are text input. The spirit is vim-flavored (efficient,
keyboard-driven, no mouse required) but the grammar is Ctrl+key like
pico/micro. Consistent with poplar's global "no multi-key sequences"
rule.
**Date:** 2026-04-11

### Two-editor architecture with Editor interface
**Decision:** Compose supports two editor backends behind an `Editor`
interface: Catkin (v1 default) and neovim via `--embed` RPC (v1.1).
Config selects the editor. The compose panel, header region, lifecycle,
and send pipeline are shared.
**Rationale:** Catkin provides an out-of-the-box experience for
everyone. Neovim embedding is the 1.1 killer feature — inline nvim in
the right panel while sidebar and chrome stay visible. No terminal
email client does this today. The `Editor` interface is designed in v1
so the neovim implementation slots in without refactoring.
**Date:** 2026-04-11

### Inline compose over terminal takeover
**Decision:** Compose renders in the right panel. Sidebar, top line,
status bar, and footer remain visible and active with compose-appropriate
content. No `tea.ExecProcess` terminal takeover.
**Rationale:** Every terminal email client today shells out and loses
context. Keeping the chrome visible during compose is the differentiating
UX feature. The header region is native bubbletea (not part of the
editor), the editor fills the remaining space.
**Date:** 2026-04-11

### Sidebar as child model, not inline rendering
**Decision:** Sidebar is a standalone `Sidebar` struct in
`internal/ui/sidebar.go` with its own `View()`, navigation methods,
and style-aware rendering. `AccountTab` owns it as a child model
and delegates key events via `handleSidebarKey`.
**Rationale:** The chrome shell pass (2.5b-1) had inline sidebar
rendering in `AccountTab.renderSidebar`. Extracting to a child model
follows Elm architecture (each component owns its state and view),
makes the sidebar independently testable, and prepares for the
message list component to follow the same pattern.
**Date:** 2026-04-11 (Pass 2.5b-2)

### Selection background via style composition
**Decision:** Selected rows apply `bg_selection` by passing a
`bgStyle lipgloss.Style` into `renderRow`. Each text segment
uses `withBg(baseStyle)` to layer the background on top of its
foreground color. Two `bgStyle` variants (plain and selected)
are computed once in `View()` and passed per-row.
**Rationale:** Lipgloss doesn't support layering backgrounds on
already-rendered ANSI text. The alternative — re-rendering each
segment with `style.Background()` — requires every render call
to know about selection state. Passing `bgStyle` as a parameter
keeps selection logic in `View()` and rendering logic in
`renderRow()`.
**Date:** 2026-04-11 (Pass 2.5b-2)

### 15 compiled themes with One Dark default
**Decision:** Ship 15 compiled themes (10 dark, 5 light). Default is
One Dark. Selection criteria: terminal ecosystem presence (kitty/alacritty
ports minimum) with popularity as tiebreaker.
**Rationale:** One Dark is neutral and familiar to millions of VS Code
users. The 15-theme lineup covers every major terminal color scheme
family. All themes are compiled Go values — no runtime config files.
`poplar themes` subcommand lists available themes.
**Date:** 2026-04-11
