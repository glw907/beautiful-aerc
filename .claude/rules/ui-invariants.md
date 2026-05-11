---
description: UI/UX invariants for poplar's bubbletea layer
paths:
  - "internal/ui/**/*.go"
  - "docs/poplar/wireframes.md"
  - "docs/poplar/keybindings.md"
---

# Poplar UI Invariants

Component and UX binding facts for poplar's bubbletea layer. Loaded
when editing `internal/ui/` or reading the wireframe / keybinding
references.

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
- `Sidebar.SetOutboxCount(n)` injects a synthetic
  `mail.CanonicalOutbox` entry at the top of the Disposal group
  when `n > 0` (hidden when zero), reusing `Folder.Unseen` for
  the count badge. Render-time injection in `effectiveEntries`;
  no mutation to `s.entries`. Selection routes the right pane
  to `internal/ui/outbox` instead of the message list. App
  refreshes the count from `Account.OutboxDepth` on every
  `account.CacheEventMsg`, guarded against unchanged totals.
  ADR-0184.
- Custom folders with `/`-paths render as a tree. `→` expands the
  parent under the cursor; `←` collapses (or, on a child, jumps
  to the ancestor and collapses it). Expand state is per-session,
  keyed by full provider path on `sidebar.Model.expanded`,
  pruned to live paths on every `SetFolders`. Collapsed parents
  show sum-of-descendants unread synthesized in the walk
  (`rowMeta.aggUnread` is descendants-only); expanded parents
  show their own `Unseen` only. Synthesized intermediate nodes
  (path segment with no real folder, e.g. "Lists" when only
  "Lists/golang" exists) render with the same prefix machinery
  and aggregate unread. Spartan tier (`LayoutMode.Spartan`, W=80–
  89) caps depth at 1: depth-2+ entries fold into their depth-1
  ancestor. Primary and Disposal groups are always flat.
  `sidebar.Model` exports a `KeyMap` + `Update`; imperative
  movement methods have been removed. ADR-0198.
- Sidebar width, sender column, date column, flag column, and
  sidebar icons are all derived from `uicore.ComputeLayout(
  termWidth)`. Three tiers: Spartan (W=80–89, sidebar=14, no
  flags, no date, no icons), Intermediate (W=90–107, flags + 3-
  or 5-cell date, no icons), Full (W=108+, all chrome on).
  Sidebar floor 14 fits "Archive"; ceiling 30. Sender slope
  0.125 hits coverage cliffs at 22/28/32 cells. The 14-cell ISO
  date is removed. ADR-0109.
- The left-hand column composite (account header rows /
  `sidebar.Model` / spacer padding / `sidebar.Search` shelf)
  lives in `sidebar.Column` (`internal/ui/sidebar/column.go`).
  `account.Model` holds one `sidebar.Column` field; reads go
  through `Sidebar()` / `SidebarSearch()` accessors and writes
  through `WithSidebar` / `WithSidebarSearch`.
  `sidebar.Column.View()` emits the column content *without*
  the right-edge `│` divider — `account.Model` still owns the
  row-by-row join with the right pane to preserve the SPUA-A-
  safe assembly invariant (ADR-0084). `sidebar.Column` does not
  propagate `SetSize` to its children; `account.Model`'s
  `WindowSizeMsg` handler calls `SetLayout` / `SetSize` on the
  children directly. ADR-0129.

### Message list

- `messagelist.Model` owns thread grouping + fold state. Embeds
  `bubbles/v2/list.Model` with a custom `*rowDelegate` for per-row
  rendering (ADR-0199). Holds `source []MessageInfo` plus derived
  `rows []displayRow` rebuilt by a group→sort→flatten pipeline; the
  visible subset of `rows` is materialized into `list.SetItems` via
  `syncList`. Hidden rows (folded thread children) stay in `m.rows`
  for `Rows()`, `ActionTargets` thread expansion, and
  `threadRootIndex`. A transient `*threadNode` tree is built per
  bucket and walked via `walkThread` (an `iter.Seq2` pull
  iterator) to compute box-drawing prefixes, then discarded — the
  renderer never sees the tree.
- `messagelist.KeyMap` (Down/Up/Top/Bottom) is the dispatch surface
  consumed by `Model.Update`. `account.Model.handleKey` falls
  through into `Update` for nav keys; fold (`space`/`F`),
  visual-mode (`v`), triage (`d`/`a`/`s`/`r`), open (`Enter`),
  search (`/`), and folder-jump keys keep account-level guards and
  call mutator methods on `messagelist.Model` directly.
  `MoveCursor(delta) (UID, bool)` survives as the viewer's
  programmatic `n`/`N` entry point.
- Date column: `displayDate(msg, now, width)` in
  `internal/ui/messagelist/model.go`. `width=3` selects
  `formatRelativeDateCompact` (`now`/`5m`/`1h`/`1d`/`1w`/`Jan`/
  `'24`); `width=5` selects `formatRelativeDateShort` (same day →
  `3:41p`, other day → `MM-DD`); `width=0` hides; zero `SentAt`
  falls back to the wire `Date` string. All in `now`'s location.
  `messagelist.Model` snapshots `now` at construction and on
  `SetMessages`; `rebuild` precomputes `displayRow.dateText` so
  the render path is I/O-free.
- `messagelist.Model.ActionTargets()` is the source of truth for triage
  scope: if anything is marked, return marks in source order
  (mode-agnostic); otherwise cursor row, with WYSIWYG expansion to
  all thread UIDs on a folded thread root. `visualMode` controls
  input routing only (`Space` marks iff on); marks survive
  `ExitVisual` and are consumed by the next dispatch. Visual mode
  auto-exits on dispatch. Bulk star/read direction follows the
  cursor row.

### Viewer

- `reader.Model` is an `account.Model` child with no backend
  reference. Body fetch + mark-read Cmds are built in
  `account.Model`; `reader.BodyLoadedMsg` carries parsed blocks
  back. Stale events are dropped by comparing
  `reader.Model.CurrentUID()`. Phases: closed → loading
  (spinner) → ready (headers + body in `bubbles/viewport`) →
  closed. While open every key routes there first; search keys +
  folder jumps are inert.
- Mark-read on viewer open is optimistic via the cache:
  `markReadCmd` queues `FlagArgs{FlagSeen, true}` through
  `cache.Account.QueueOp`, which transactionally flips `ui_flags`
  and inserts the outbox row. The follow-up `folderLoadedMsg`
  refresh re-reads the now-flipped state into `messagelist.Model`
  via `RefreshSource` (cursor preserved). Failures surface via
  `uicore.ErrorMsg` into the App-owned banner.
- Body content rendering caps at `maxBodyWidth = 72` cells; headers
  wrap at the panel width (uncapped). Outbound links are harvested
  by `content.RenderBodyWithFootnotes` into `[N]: <url>` rows below
  a rule; inline link text gets ` [^N]` glued to its last word with
  U+00A0. Short bare URLs (`Text == URL`, ≤30 cells) render inline
  without a marker.
- Invite block + chip row sit between header panel and body, in
  that order, both optional. Layout owned by `reader.Model.layout()`;
  body height = `v.height - panel - inviteHeight - chipHeight`.
  Chip row hidden when `len(attachments) == 0`; populated from
  `reader.AttachmentsLoadedMsg` batched in the same Cmd as
  `reader.BodyLoadedMsg` on viewer open. `@` opens the App-owned
  `reader.AttachPicker` overlay
  (`o`/Enter/digit open via `xdg-open` on a tempfile; `s` saves to
  `[ui] download_dir`; `Esc`/`q`/`@` close). Invite block hidden
  when no `text/calendar`/`application/ics` part is present;
  `reader.BodyLoadedMsg.Invite` carries the parsed first VEVENT.
  Display only — no RSVP, no CalDAV, no popover. Method=CANCEL
  prepends a `[CANCELLED]` row in `ColorWarning`. Recurrence
  humanizes only `FREQ`+`INTERVAL`; anything fancier drops the
  Repeats row. Library: `arran4/golang-ical`. ADR-0186.

### Triage, undo, error banner

- Triage actions (delete/archive/star/read/move) are optimistic
  through the cache. `account.Model.dispatchTriage` (or
  `dispatchMoveFromPicker`) calls `queueOpsCmd` which queues the
  op via `cache.QueueOp` (transactional optimistic flip on
  `ui_flags` / `ui_hide`) and immediately re-reads the folder via
  `cache.QueryFolder`. The result `folderLoadedMsg` updates the
  msglist via `RefreshSource` (cursor preserved).
  `account.TriageStartedMsg` carries the inverse Cmd — a
  compensating `cache.QueueOp` that reverses the action. `App`
  owns `pendingAction` and schedules a `tea.Tick` for `[ui]
  undo_seconds` (default 6, clamped `[2, 30]`). `u` fires the
  saved inverse Cmd; there is no separate local roll-back since
  cache state is the only state. A folder change commits the
  toast (cache state stands). A `uicore.ErrorMsg` clears the
  toast — the user's responsibility to fire the inverse manually
  if a forward op already flipped state. The chrome row above
  the status bar is shared with the error banner; error wins,
  then toast, else the row collapses (`App.chromeBannerRow`).
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
- `uicore.ErrorMsg{Op, Err}` is the canonical Cmd error type.
  Every fallible `tea.Cmd` returns it with a short verb-phrase
  `Op` ("mark read", "fetch body", "purge expired"). `App` owns
  `lastErr` (last-write-wins). Banner is one foreground-only row
  above the status bar (`⚠ <Op>: <Err>`), truncated with `…`;
  account region shrinks one cell when shown so view height is
  unchanged. No key steal, dismiss, severity, queue. Part of the
  dimmed underlay while overlays are open.
- Spinner placeholders go through `uicore.NewSpinner(t)` (Dot,
  `FgDim`); shared across viewer/folder/send.

### Compose

- `compose.Model` (`internal/ui/compose/model.go`, imported as
  `uicompose` from App-side) is the App-owned inline compose
  surface. While `App.compose != nil`, App routes keys into it
  and renders its `View()` in place of `account.Model`'s right
  pane via `account.Model.RenderWithRightPane` — sidebar and
  chrome stay drawn, no overlay, no `tea.ExecProcess`. Five
  focusable fields: To/Cc/Bcc/Subject as `bubbles/textinput`,
  body as `mailcompose.Editor` (CatkinEditor in v1; v1.1 will
  add a neovim adapter behind the same interface).
- Focus model. `Tab` / `Shift+Tab` cycles To→Cc→Bcc→Subject→Body
  and wraps. `Esc` is a focus toggle only (Body→Subject; any
  header→Body) and never closes compose. `Ctrl+X` sends — emits
  `uicompose.SendMsg` with the assembled draft. `Ctrl+C` cancels
  — emits `uicompose.CancelMsg{Dirty}`; App opens `ConfirmModal`
  when Dirty, closes immediately otherwise. Per ADR-0076 text-
  entry surfaces are exempt from the modifier-free rule, so
  these chords coexist with Catkin's `Ctrl+B/I/K/L/Q/Space`.
- App owns the lifecycle: `compose *uicompose.Model` (nil when
  closed) + `(tidyEnabled, tidyAPIKey, tidyCfg)` resolved once
  at construction from `[ui.tidy]` and `tidy.ResolveAPIKey`,
  threaded into every new compose model via `SetTidy`. `c`
  opens fresh; `r`/`R`/`f` open via `composeSeedCmd` after
  fetching the parent body. The send path calls
  `mailcompose.AssembleMIME`, then `cache.Account.QueueOutbound`
  (one op JMAP, two ops IMAP per ADR-0160). Sent folder
  resolves via the cached classified-folder list
  (`cf.Canonical == "Sent"`) with case-fold "Sent" name
  fallback; missing surfaces inline as `c.err`.
- `Ctrl+T` in the body runs `tidy.Tidy` in a `tea.Cmd`; on
  return compose replaces the catkin buffer and feeds
  character-range diffs to a `tidyAnnotator` painted in
  `Styles.TidyChange` (`AccentPrimary` underline). Highlights
  clear on first body mutation. Tidy never runs on send. The
  footer shows a gated `^T tidy` hint at rank 6 when
  `[ui.tidy] enabled` and the body is focused. ADR-0178.
- `Ctrl+O` opens `uicompose.AttachPicker` — multi-select TUI
  file browser overlay (`internal/ui/compose/attachpicker.go`).
  This file is a deliberate deviation from the `bubbles/v2/filepicker`
  adoption pattern (ADR-0194): multi-select model, icon UX, and
  ModalShell wrap don't compose with filepicker's single-select +
  permissions columns + freestanding View; see ADR-0195 for the
  full rationale. Three patterns are lifted from upstream without
  importing it: symlink resolution via `e.Type()&os.ModeSymlink`
  + `filepath.EvalSymlinks`/`os.Stat`; atomic package-level id
  counter (`nextAttachID atomic.Int64`); right-aligned 7-cell size
  column via `lipgloss.Right`. Vim h/l/g/G nav, async readDir with
  id-guard, view-state stack on ascend. `Space` toggles, `a`
  accepts, `Enter` on a file with empty selection is a
  single-attach shortcut, `.` toggles hidden, `Esc` cancels.
  Picker emits `uicompose.AttachAcceptedMsg{Paths}` /
  `uicompose.AttachCancelledMsg{}`; compose appends deduped to
  `c.attachments`, bumps `localDirty`, kicks
  autosave, remembers `attachLastDir`. Compose grows a
  `focusAttach` enum slot between `focusSubject` and `focusBody`,
  skipped in the Tab cycle when no attachments. Inside
  `focusAttach`: `←/→` moves the chip cursor; `d`/`Backspace`/
  `Delete` removes the selected chip; emptying collapses focus to
  Subject. Chip row renders below Subject with humanized sizes,
  hidden when empty; overflow shows `+N`. Footer hint `^O attach`
  at rank 6. App overlays the picker on top of compose via
  `uicore.PlaceOverlay`; compose's `SetSize` forwards into the
  picker. Picker rides inside compose's input window — outside
  the global modal cascade. ADRs 0179, 0195.
- `Ctrl+L` in compose opens `uicompose.SchedulePicker` — three
  preset rows (Tomorrow morning / afternoon, Monday morning)
  plus a "Custom…" row that expands a `bubbles/textinput`
  parsed by pure `uicompose.ParseSchedule(s, now)`. Accept
  emits `uicompose.ScheduleAcceptedMsg{When}`; `m.scheduledFor`
  threads through `composeSendCmd` into `QueueOutbound` and
  bypasses the ADR-0183 undo-send window. Picker overlays on
  top of compose via `uicore.PlaceOverlay`, sized through
  compose's input window like AttachPicker. Outbox-side
  reschedule reuses the same picker but under App ownership
  (`pendingReschedule{picker, opID}`) so accepts route to
  `cache.RescheduleOp` instead of dispatchSend. Footer hint
  `^L later` at rank 6. ADRs 0076 (modifier exemption), 0184.
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
  underlying frame, dim via `uicore.DimANSI`, composite via
  `uicore.PlaceOverlay` (vendored from superfile, MIT) at the
  centered top-left from `uicore.CenterOverlay`. While an
  overlay is open, `App.Update` short-circuits keys into it.
  Seven overlays exist: help popover (`App` owns `helpOpen` +
  `help helppopover.Model`; `viewerOpen` selects `HelpAccount`
  vs `HelpViewer` context), link picker (viewer-context-only,
  `reader.LinkPicker`), attachment picker (`reader.AttachPicker`,
  `@` from viewer; `o`/`s`/`Enter`/digit/`Esc`), move picker
  (`movepicker.Model`, `m` from account view), confirm modal
  (`ConfirmModal` — generic destructive-action prompt, used by
  manual empty), outbox overlay (`OutboxOverlay`, `Q` — read-only
  grouped summary), and conflict overlay (`ConflictOverlay`,
  `!` — per-row retry / discard via `cache.RetryOp` /
  `DiscardOp`). Confirm is topmost — its key-route and overlay-
  render branches run before the others. Cascade order: confirm
  > conflict > outbox > help > link picker > attach picker >
  move picker. The Box-rendering overlays (`ConfirmModal`,
  `reader.LinkPicker`, `reader.AttachPicker`, `movepicker.Model`,
  `OutboxOverlay`, `ConflictOverlay`) share frame chrome via a
  named-field embedded `shell uicore.ModalShell`
  (`internal/ui/uicore/modal_shell.go`); per-overlay `View()`
  builds `bodyRows` + `footerRows` pre-padded to `contentW`
  cells and calls `m.shell.Box(title, bodyRows, footerRows,
  contentW)`. `helppopover.Model` uses `lipgloss.Style` with a
  rounded border + embedded-title top edge and is *not* a
  ModalShell consumer (ADR-0201); the popover does not adopt
  `bubbles/v2/help` (ADR-0200 — six concrete deviations covering
  ADR-0072 wired/unwired dimming and the ADR-0084 JoinHorizontal
  ban among them). `movepicker.Model`,
  `helppopover.Model`, and `ConflictOverlay` cache their per-
  frame render via a heap-allocated `*<T>Cache` pointer + dirty
  flag (the only Elm-immutable-model escape hatch in the tree,
  scoped to view-stable overlays — see ADR-0130).
  `OutboxOverlay` does not — its content churns every cache
  event while the queue drains.
- Help popover advertises the full planned keybinding vocabulary,
  not just currently-wired keys. Each row in the binding tables
  carries a `wired bool` flag. Wired rows: bright-bold key + dim
  desc. Unwired rows: dim throughout. Group headings stay bright.
  Later passes flip wired flags as bindings come online.
- Viewer link launch: `1`–`9` opens the Nth harvested URL via
  `xdg-open` (fire-and-forget). `Tab` opens `reader.LinkPicker`
  when ≥1 URL is harvested (inert otherwise). Picker is App-
  owned, viewer-context-only: `j/k` cursor, `Enter`/`1`–`9`
  launch+close, `Esc`/`Tab` close, `q` swallowed.
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
- Status bar carries a backfill-progress segment between the
  outbox depth chunk and the connection indicator: `↓ N/M` in
  Full / Intermediate tiers (width >= 90), bare `↓` glyph in
  Spartan (width < 90). Hidden when `total == 0` or `done >=
  total`. Substates: `↓ paused` / `↓⏸` while offline or mid-
  activity (driven by `m.statusBar.ConnectionState() !=
  Connected`); `↓ ⚠` / `↓⚠` for persistent throttle, sourced
  from `Cache().BackfillProgress`'s `warn` return.
  `App.refreshBackfillSegment()` queries `Cache().BackfillProgress`
  on every `account.CacheEventMsg` and `backendUpdateMsg`,
  short-circuiting when `(done, total, paused)` is unchanged so
  the COUNT queries don't fire on every drainer event once the
  segment is steady. ADR-0187.
- Connection state Offline + non-empty outbox emits a one-shot
  ErrorMsg banner ("offline — queued ops will sync on
  reconnect"). Empty outbox stays silent. The drainer's behavior
  is unchanged by ConnState — it keeps trying with exponential
  backoff (cap 60s).
