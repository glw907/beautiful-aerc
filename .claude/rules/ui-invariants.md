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
- Sidebar width, sender column, date column, flag column, and
  sidebar icons are all derived from `ComputeLayout(termWidth)`
  in `internal/ui/layout.go`. Three tiers: Spartan (W=80–89,
  sidebar=14, no flags, no date, no icons), Intermediate
  (W=90–107, flags + 3- or 5-cell date, no icons), Full
  (W=108+, all chrome on). Sidebar floor 14 fits "Archive";
  ceiling 30. Sender slope 0.125 hits coverage cliffs at
  22/28/32 cells. The 14-cell ISO date is removed. ADR-0109.
- The left-hand column composite (account header rows / `Sidebar` /
  spacer padding / `SidebarSearch` shelf) lives in
  `SidebarColumn` (`internal/ui/sidebar_column.go`). `AccountTab`
  holds one `SidebarColumn` field; reads go through
  `Sidebar()` / `SidebarSearch()` accessors and writes through
  `WithSidebar` / `WithSidebarSearch`. `SidebarColumn.View()`
  emits the column content *without* the right-edge `│` divider
  — `AccountTab` still owns the row-by-row join with the right
  pane to preserve the SPUA-A-safe assembly invariant
  (ADR-0084). `SidebarColumn` does not propagate `SetSize` to
  its children; `AccountTab.WindowSizeMsg` calls
  `SetLayout`/`SetSize` on the children directly. ADR-0129.

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
- Mark-read on viewer open is optimistic via the cache:
  `markReadCmd` queues `FlagArgs{FlagSeen, true}` through
  `cache.Account.QueueOp`, which transactionally flips `ui_flags`
  and inserts the outbox row. The follow-up `folderLoadedMsg`
  refresh re-reads the now-flipped state into `MessageList` via
  `RefreshSource` (cursor preserved). Failures surface via
  `ErrorMsg` into the App-owned banner.
- Body content rendering caps at `maxBodyWidth = 72` cells; headers
  wrap at the panel width (uncapped). Outbound links are harvested
  by `content.RenderBodyWithFootnotes` into `[N]: <url>` rows below
  a rule; inline link text gets ` [^N]` glued to its last word with
  U+00A0. Short bare URLs (`Text == URL`, ≤30 cells) render inline
  without a marker.
- Chip row sits between header panel and body. Hidden when
  `len(attachments) == 0`. Layout owned by `Viewer.layout`; body
  height = `v.height - panel - chipHeight`. Chip row populates from
  `attachmentsLoadedMsg` batched in the same Cmd as `bodyLoadedMsg`
  on viewer open. `@` opens the App-owned `AttachPicker` overlay
  (`o`/Enter/digit open via `xdg-open` on a tempfile; `s` saves to
  `[ui] download_dir`; `Esc`/`q`/`@` close).

### Triage, undo, error banner

- Triage actions (delete/archive/star/read/move) are optimistic
  through the cache. `AccountTab.dispatchTriage` (or
  `dispatchMoveFromPicker`) calls `queueOpsCmd` which queues the
  op via `cache.QueueOp` (transactional optimistic flip on
  `ui_flags` / `ui_hide`) and immediately re-reads the folder via
  `cache.QueryFolder`. The result `folderLoadedMsg` updates the
  msglist via `RefreshSource` (cursor preserved). `triageStartedMsg`
  carries the inverse Cmd — a compensating `cache.QueueOp` that
  reverses the action. `App` owns `pendingAction` and schedules a
  `tea.Tick` for `[ui] undo_seconds` (default 6, clamped `[2, 30]`).
  `u` fires the saved inverse Cmd; there is no separate local
  roll-back since cache state is the only state. A folder change
  commits the toast (cache state stands). An `ErrorMsg` clears the
  toast — the user's responsibility to fire the inverse manually if
  a forward op already flipped state. The chrome row above the
  status bar is shared with the error banner; error wins, then
  toast, else the row collapses (`App.chromeBannerRow`).
  `pendingAction.IsZero()` checks `op == ""`. "Delete" is a Move
  to the canonical Trash folder (no `mail.Backend.Delete` exists);
  the inverse moves it back to the source folder.
- Permanent-delete consumers — both bypass the undo bar (the
  primitive is irreversible). **Retention sweep:** opt-in via `[ui]
  trash_retention_days` / `spam_retention_days` (default 0, clamp
  `[0, 365]`). Fires once per session per Disposal folder, on first
  `folderLoadedMsg` for that folder. Iterates loaded messages,
  collects UIDs whose `SentAt` is before
  `now - retention_days * 24h` (zero `SentAt` skipped — partial
  sweep by design), dispatches `destroyCmd` which queues
  `DestroyArgs{}` per UID via `cache.QueueOp`. `swept[name]` flag
  is set on first attempt regardless of outcome — failures land in
  the error banner; no retry-loop. **Manual empty:** `E` on
  Disposal folders → `OpenConfirmEmptyMsg` → App opens
  `ConfirmModal` and stores `pendingEmptyConfirm{folder, source}`
  → user accepts → `ConfirmModalYesMsg` → App emits
  `EmptyFolderConfirmedMsg` → `emptyFolderCmd` pages
  `cache.QueryFolder` in 1000-unit batches → `enqueueDestroys`
  fan-out. Toast renders `Emptied <Folder> (<N>)` and suppresses
  `[u undo]` (toast keys off `op == opEmpty`).
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

### Compose

- `ComposeTab` (`internal/ui/compose_tab.go`) is the App-owned
  inline compose surface. While `App.compose != nil`, App routes
  keys into ComposeTab and renders its `View()` in place of
  AccountTab's right pane via `AccountTab.RenderWithRightPane` —
  sidebar and chrome stay drawn, no overlay, no
  `tea.ExecProcess`. Five focusable fields: To/Cc/Bcc/Subject as
  `bubbles/textinput`, body as `compose.Editor` (CatkinEditor in
  v1; v1.1 will add a neovim adapter behind the same interface).
- Focus model. `Tab` / `Shift+Tab` cycles To→Cc→Bcc→Subject→Body
  and wraps. `Esc` is a focus toggle only (Body→Subject; any
  header→Body) and never closes compose. `Ctrl+X` sends — emits
  `ComposeSendMsg` with the assembled draft. `Ctrl+C` cancels —
  emits `ComposeCancelMsg{Dirty}`; App opens `ConfirmModal` when
  Dirty, closes immediately otherwise. Per ADR-0076 text-entry
  surfaces are exempt from the modifier-free rule, so these
  chords coexist with Catkin's `Ctrl+B/I/K/L/Q/Space`.
- App owns the lifecycle: `compose *ComposeTab` (nil when closed)
  + `(tidyEnabled, tidyAPIKey, tidyCfg)` resolved once at
  construction from `[ui.tidy]` and `tidy.ResolveAPIKey`, threaded
  into every new `compose.Model` via `SetTidy`. `c` opens fresh;
  `r`/`R`/`f` open via `composeSeedCmd` after fetching the parent
  body. The send path calls `compose.AssembleMIME`, then
  `cache.Account.QueueOutbound` (one op JMAP, two ops IMAP per
  ADR-0160). Sent folder resolves via the cached classified-
  folder list (`cf.Canonical == "Sent"`) with case-fold "Sent"
  name fallback; missing surfaces inline as `c.err`.
- `Ctrl+T` in the body runs `tidy.Tidy` in a `tea.Cmd`; on
  return compose replaces the catkin buffer and feeds
  character-range diffs to a `tidyAnnotator` painted in
  `Styles.TidyChange` (`AccentPrimary` underline). Highlights
  clear on first body mutation. Tidy never runs on send. The
  footer shows a gated `^T tidy` hint at rank 6 when
  `[ui.tidy] enabled` and the body is focused. ADR-0178.
- Single-instance for Pass 9h. Drafts persistence is 9h.5;
  address autocomplete is 9.1; signatures + identities is 9.4.
  `ComposeTab`/`AccountTab` names are placeholders pending the
  Pass 9h.1 organizational sweep.

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
  short-circuits keys into it. Seven overlays exist: help popover
  (`App` owns `helpOpen` + `help HelpPopover`; `viewerOpen` selects
  `HelpAccount` vs `HelpViewer` context), link picker
  (viewer-context-only), attachment picker (`@` from viewer;
  `o`/`s`/`Enter`/digit/`Esc`), move picker (`m` from account
  view), confirm modal (`ConfirmModal` — generic destructive-action
  prompt, used by manual empty), outbox overlay (`Q` — read-only
  grouped summary), and conflict overlay (`!` — per-row retry /
  discard via `cache.RetryOp` / `DiscardOp`). Confirm is topmost —
  its key-route and overlay-render branches run before the others.
  Cascade order: confirm > conflict > outbox > help > link picker
  > attach picker > move picker. The Box-rendering overlays
  (`ConfirmModal`, `LinkPicker`, `AttachPicker`, `MovePicker`,
  `OutboxOverlay`, `ConflictOverlay`)
  share frame chrome via a named-field embedded `shell ModalShell`
  (`internal/ui/modal_shell.go`); per-overlay `View()` builds
  `bodyRows` + `footerRows` pre-padded to `contentW` cells and
  calls `m.shell.Box(title, bodyRows, footerRows, contentW)`.
  `HelpPopover` uses `lipgloss.Style` with a rounded border and is
  *not* a ModalShell consumer. `MovePicker`, `HelpPopover`, and
  `ConflictOverlay` cache their per-frame render via a heap-
  allocated `*<T>Cache` pointer + dirty flag (the only Elm-
  immutable-model escape hatch in the tree, scoped to view-stable
  overlays — see ADR-0130). `OutboxOverlay` does not — its content
  churns every cache event while the queue drains.
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

### Cache & Outbox

- `Q` opens the outbox overlay (read-only grouped summary).
- `!` opens the conflict overlay; the user resolves rows with
  `r` (retry — `cache.RetryOp` resets attempts and signals the
  drainer) or `d` (discard — `cache.DiscardOp` reverts the
  optimistic flip and deletes the outbox row).
- Status bar carries an outbox depth segment: `⇅N` (FgDim) when
  only in-flight ops; `⚠N` (ColorWarning, N = inflight + conflict)
  when any conflict; segment hidden when outbox is empty.
- Connection state Offline + non-empty outbox emits a one-shot
  ErrorMsg banner ("offline — queued ops will sync on
  reconnect"). Empty outbox stays silent. The drainer's behavior
  is unchanged by ConnState — it keeps trying with exponential
  backoff (cap 60s).
