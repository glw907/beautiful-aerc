---
description: UI/UX invariants for poplar's bubbletea layer
paths:
  - "internal/ui/**/*.go"
  - "docs/superpowers/plans/**/*.md"
  - "docs/superpowers/specs/**/*.md"
  - "docs/poplar/wireframes.md"
  - "docs/poplar/keybindings.md"
---

# Poplar UI Invariants

Component and UX binding facts for poplar's bubbletea layer. Loaded
when editing `internal/ui/`, planning a UI pass (plan or spec
docs), or reading the wireframe / keybinding references.

The authoritative key map is `docs/poplar/keybindings.md` — this
file describes behavior, not the key tables.

## Components

### Sidebar

- Account view is one pane. No focus cycling. `j/k` always navigates
  messages, `J/K` always navigates folders, every triage and reply
  key is always live.
- Sidebar renders three folder groups in fixed order: Primary,
  Disposal, Custom. Separated by blank lines. No group headers.
  Groups are permanent — user config only ranks folders within
  their group.
- Nested folder names (containing `/`) render flat. The `/` in the
  display name is the only affordance. No tree, no expand/collapse.
- Sidebar width is responsive: `sidebarWidthFor(termWidth) =
  clamp(termWidth - 56, 24, 30)`. Linear from 24 at termWidth=80 up
  to 30 at termWidth≥86; clamped below 80. Folder labels truncate
  with `…` (via `displayTruncateEllipsis`) when their natural width
  exceeds the per-row label budget. Every rendered folder row
  preserves a 1-cell right margin before the chrome divider at
  every width in `[24, 30]`. ADR-0096.

### Message list

- `MessageList` owns thread grouping + fold state. Holds `source
  []MessageInfo` plus derived `rows []displayRow` rebuilt by a
  group→sort→flatten pipeline. A transient `*threadNode` tree is
  built per bucket in `appendThreadRows` to compute box-drawing
  prefixes, then discarded — the renderer never sees the tree.
- Date column: `formatRelativeDate(t, now)` in
  `internal/ui/date_format.go`. Same calendar day → 12-hour time
  (`10:23 AM`); other day → `Mon 2006-01-02`; zero → empty. All in
  `now`'s location. `MessageList` snapshots `now` at construction
  and on `SetMessages`; `rebuild` precomputes
  `displayRow.dateText` so the render path is I/O-free.
- `MessageList.ActionTargets()` is the source of truth for triage
  scope: if anything is marked, return marks in source order
  (mode-agnostic); otherwise cursor row, with WYSIWYG expansion to
  all thread UIDs on a folded thread root. `visualMode` controls
  input routing only (`Space` marks iff on); marks survive
  `ExitVisual` and are consumed by the next dispatch. Visual mode
  auto-exits on dispatch. Bulk star/read direction follows the
  cursor row.

### Viewer

- `Viewer` is an `AccountTab` child with no backend reference. Body
  fetch + mark-read Cmds are built at `AccountTab`; `bodyLoadedMsg`
  carries parsed blocks back. Stale events are dropped by comparing
  `viewer.CurrentUID()`. Phases: closed → loading (spinner) → ready
  (headers + body in `bubbles/viewport`) → closed. While open every
  key routes there first; search keys + folder jumps are inert.
- Mark-read on viewer open is optimistic: `MessageList.MarkSeen`
  flips the local seen flag immediately and the backend `MarkRead`
  Cmd runs in parallel. Failures surface via `ErrorMsg` into the
  App-owned banner.
- Body content rendering caps at `maxBodyWidth = 72` cells; headers
  wrap at the panel width (uncapped). Outbound links are harvested
  by `content.RenderBodyWithFootnotes` into `[N]: <url>` rows below
  a rule; inline link text gets ` [^N]` glued to its last word with
  U+00A0. Short bare URLs (`Text == URL`, ≤30 cells) render inline
  without a marker.

### Triage, undo, error banner

- Triage actions (delete/archive/star/read/move) are optimistic with
  a shared undo bar. `MessageList.Apply{Delete,Insert,Flag,Seen}`
  flip local state without firing Cmds; `AccountTab.dispatchTriage`
  (or `dispatchMoveFromPicker` for move) snapshots inverse data,
  applies the flip, exits visual mode, and emits `triageStartedMsg`
  + the forward Cmd via `buildTriageCmd` (or
  `buildTriageCmdWithDest` for move's dest). `App` owns
  `pendingAction` and schedules a `tea.Tick` for `[ui] undo_seconds`
  (default 6, clamped `[2, 30]`). `u` fires `onUndo` + the saved
  inverse Cmd. A folder change commits (no inverse). An `ErrorMsg`
  runs `onUndo` before setting `lastErr` so a backend failure
  visibly reverts the flip. The chrome row above the status bar is
  shared with the error banner; error wins, then toast, else the
  row collapses (`App.chromeBannerRow`). `pendingAction.IsZero()`
  checks `op == ""`.
- Permanent-delete consumers — both bypass the undo bar (the
  primitive is irreversible). **Retention sweep:** opt-in via `[ui]
  trash_retention_days` / `spam_retention_days` (default 0, clamp
  `[0, 365]`). Fires once per session per Disposal folder, on first
  `headersAppliedMsg` for that folder. Iterates loaded messages,
  collects UIDs whose `SentAt` is before
  `now - retention_days * 24h` (zero `SentAt` skipped — partial
  sweep by design), dispatches `destroyCmd`. `swept[name]` flag is
  set on first attempt regardless of outcome — failures land in the
  error banner; no retry-loop. **Manual empty:** `E` on Disposal
  folders → `OpenConfirmEmptyMsg` → App opens `ConfirmModal` →
  `EmptyFolderConfirmedMsg` → `emptyFolderCmd` pages `QueryFolder`
  in 1000-unit batches → `Destroy`. Toast renders `Emptied <Folder>
  (<N>)` and suppresses `[u undo]` (toast keys off `op == "empty"`).
- `ErrorMsg{Op, Err}` is the canonical Cmd error type. Every
  fallible `tea.Cmd` returns it with a short verb-phrase `Op`
  ("mark read", "fetch body", "purge expired"). `App` owns
  `lastErr` (last-write-wins). Banner is one foreground-only row
  above the status bar (`⚠ <Op>: <Err>`), truncated with `…`;
  account region shrinks one cell when shown so view height is
  unchanged. No key steal, dismiss, severity, queue. Part of the
  dimmed underlay while overlays are open.
- Spinner placeholders go through `NewSpinner(t)` (Dot, `FgDim`) in
  `internal/ui/styles.go`; shared across viewer/folder/send.

### Compose (planned)

- Compose is pluggable behind an `Editor` interface. v1 ships
  Catkin (native bubbletea editor); v1.1 adds neovim via `--embed`
  RPC. Compose renders inline — sidebar and chrome stay visible.
  No `tea.ExecProcess` terminal takeover.

## UX

### Keybinding philosophy

- Poplar is opinionated and not configurable in v1. Users who want
  maximum configurability should use aerc or mutt.
- Vim-first keybindings: single-key motions, visual mode for
  multi-select. No multi-key sequences (one tea.KeyMsg per
  keypress).
- No `:` command mode. Every action is a single-key binding or a
  modal picker launched by a key.
- Modifier-free: user-facing actions never bind a Ctrl/Alt/Meta
  chord. Viewer scroll uses single keys (see keybindings.md).
  `Ctrl-c` survives only as a terminal-kill alias on the Quit
  binding; never advertised. `pgup/pgdown` are not bound.
- Folder jumps use uppercase single keys; the lowercase/uppercase
  pairing is namespaced so triage (`d`) and folder jump (`D`)
  coexist without conflict.
- `q` exits the viewer when the viewer is open, quits poplar when
  on the account view. While the sidebar search shelf is non-idle,
  `q` is stolen and clears the search instead of quitting. While
  the help popover is open, `q` is swallowed (help is a view, not
  a state to escape). `?` opens the help popover; `?` or `Esc`
  closes it.

### Overlays

- App owns modal overlays via the same compose pattern: render
  underlying frame, dim via `DimANSI`, composite via `PlaceOverlay`
  (vendored from superfile, MIT) at the centered top-left from
  `centerOverlay`. While an overlay is open, `App.Update`
  short-circuits keys into it. Four overlays exist: help popover
  (`App` owns `helpOpen` + `help HelpPopover`; `viewerOpen` selects
  `HelpAccount` vs `HelpViewer` context), link picker
  (viewer-context-only), move picker (`m` from account view), and
  confirm modal (`ConfirmModal` — generic destructive-action
  prompt, used by manual empty). Confirm is topmost — its
  key-route and overlay-render branches run before the others.
- Help popover advertises the full planned keybinding vocabulary,
  not just currently-wired keys. Each row in the binding tables
  carries a `wired bool` flag. Wired rows: bright-bold key + dim
  desc. Unwired rows: dim throughout. Group headings stay bright.
  Later passes flip wired flags as bindings come online.
- Viewer link launch: `1`–`9` opens the Nth harvested URL via
  `xdg-open` (fire-and-forget). `Tab` opens `LinkPicker` when ≥1
  URL is harvested (inert otherwise). Picker is App-owned,
  viewer-context-only: `j/k` cursor, `Enter`/`1`–`9` launch+close,
  `Esc`/`Tab` close, `q` swallowed.
- Bare URL footnoting: `Link{Text: url, URL: url}` with
  `lipgloss.Width > 30` cells harvests into the footnote list with
  `trimURL(url) + nbsp + [^N]` inline. Short bare URLs pass
  through. `trimURL` strips scheme, keeps host (+port), appends
  `/<first-segment>` when present, `…` when anything was removed.

### Reading & navigation

- `Enter` on the message list opens the selected message in the
  viewer. Unread → marked seen optimistically. `q`/`Esc` closes
  the viewer and the cursor stays on the same row. While the
  viewer is ready, `n`/`N` advances/retreats to the next visible
  message (skipping folded rows), reusing the same fetch +
  mark-read flow as `Enter`. Boundaries are inert; `n`/`N` are
  inert during `viewerLoading`.
- Threaded display is default-on. Per-folder `[ui.folders.<name>]
  threading = false` overrides to flat. No runtime toggle.
- Threads sort by latest activity (max date across the thread) in
  the folder's configured direction. Children inside a thread
  always sort chronologically ascending regardless of folder
  direction. Folder sort comes from `[ui.folders.<name>] sort`
  (`date-desc` default, `date-asc` opt-in).
- Thread root is the message with empty `InReplyTo`. Fallback for
  broken chains: earliest by date in the bucket; remaining orphans
  attach to the root as depth-1 children.
- Fold state is per-session, reset on every `SetMessages` (folder
  reload). Threads default expanded. The `[N] ` prefix badge
  replaces the box-drawing prefix on a collapsed root.
- `Space` toggles fold on the thread under the cursor (snaps to
  nearest visible row after fold; in visual-select mode toggles
  row selection instead). `F` is the bulk counterpart: folds every
  multi-message thread if any is unfolded, else unfolds everything.
- Search: `/` activates a 3-row shelf pinned to the bottom of the
  sidebar. Filter-and-hide: non-matching threads disappear;
  matching threads render fully expanded regardless of saved fold
  state (preserved, restored on `Esc`). `Esc` clears query +
  restores pre-search cursor. `Tab` cycles `[name]` (subject +
  sender) ↔ `[all]` (+date text). Case-insensitive substring;
  current folder only — folder jumps clear search. Fold keys inert
  while filter is committed.

### Visual language

- Message list encodes read state by brightness — unread sender is
  `FgBright` bold, unread subject is `FgBright`; read rows are
  `FgDim`. Hue is reserved for the cursor (`AccentPrimary`) and
  for the unread+flagged case (`ColorWarning`). Read-flagged rows
  dim their flag glyph along with the rest of the row.
- Chrome is a three-sided frame: top `──┬──╮`, right `│`, bottom
  status bar `──┴──╯`. No left border.
- Connection state renders as shape + color + text for colorblind
  accessibility: `●` green connected, `◐` orange reconnecting,
  `○` red hollow offline.
- Command footer is the primary discoverability surface. Each hint
  carries a drop rank 0–10. When the terminal is too narrow, hints
  drop in descending rank order. Rank 0 (`? help`, `q quit`) never
  drops. Groups with no remaining hints collapse their preceding
  `┊` separator.
