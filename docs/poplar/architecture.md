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
