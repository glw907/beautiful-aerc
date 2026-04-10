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
| UI | `internal/ui/` | Bubbletea components (tabs, sidebar, msglist, viewer, statusbar) |
| Mail adapter | `internal/mail/` | `Backend` interface, poplar-native types, account lifecycle |
| Poplar config | `internal/poplar/` | AccountConfig struct, poplar-specific types |
| Forked workers | `internal/aercfork/worker/` | Forked aerc IMAP + JMAP workers |
| Forked models | `internal/aercfork/models/` | Forked aerc data types |
| Forked support | `internal/aercfork/{log,parse,xdg,auth,keepalive}/` | Forked aerc support libraries |
| Rendering | `internal/filter/` | Existing HTML/plain/header filters (shared with mailrender) |
| Themes | `internal/theme/` | Existing TOML theme loader (shared with all binaries) |
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

### Lipgloss styles from theme TOML
**Decision:** UI chrome uses lipgloss styles derived from the same
theme TOML files that drive mailrender's ANSI tokens.
**Rationale:** One theme file controls the entire visual experience.
Switching themes changes both message rendering and UI chrome.
**Date:** 2026-04-09

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
**Decision:** Full vim motion set (j/k, gg/G, C-d/C-u, C-f/C-b,
zo/zc/za), vim visual mode for multi-select (v), `g`-prefix for
folder jumps (gi/gd/gs/ga/gx/gt). Not configurable in v1.
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
exist in the data model for sort order and `g`-prefix jumps.
**Date:** 2026-04-10

### Provider folder name normalization
**Decision:** Poplar displays canonical folder names (Inbox, Sent,
Trash) regardless of provider naming ([Gmail]/Sent Mail, Sent Items,
etc.). Recognition via case-insensitive matching against known
aliases.
**Rationale:** Users shouldn't see provider implementation details.
Canonical names work across providers and match the `g`-prefix
jump mnemonics.
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
