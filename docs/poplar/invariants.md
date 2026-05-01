# Poplar Invariants

Binding facts for the poplar codebase. Edited in place — new facts
replace or narrow old facts, they do not append. When a pass changes
a binding fact, update this file before committing.

Every fact here is codified in an ADR under `docs/poplar/decisions/`.
The decision index at the bottom maps each section's claims back to
the ADR(s) that justify them.

## Architecture

- Poplar is a single-binary bubbletea terminal email client built
  from one Go module: `cmd/poplar`.
- Repository organization: `cmd/poplar/` (CLI wiring only),
  `internal/ui/` (tea.Model tree), `internal/mail/` (`Backend`
  interface + classifier), `internal/mailjmap/` (Fastmail via
  `git.sr.ht/~rockorager/go-jmap`), `internal/mailimap/` (Gmail via
  `emersion/go-imap` v1), `internal/mailauth/` (vendored XOAUTH2 +
  keepalive snippets), `internal/config/` (`AccountConfig`,
  `UIConfig`, `LoadUI`), `internal/theme/` (compiled lipgloss
  themes), `internal/term/` (capability detection: `HasNerdFont`,
  `MeasureSPUACells`). `internal/filter/`, `internal/content/`,
  `internal/tidy/` await their consumers.
- Mail backends call upstream libraries directly. No aerc fork.
  The library family is emersion (`go-imap` v1, `go-message`,
  `go-smtp`, `go-sasl`, `go-webdav`, `go-vcard`) plus
  `rockorager/go-jmap`. Vendored snippets are MIT-licensed helpers
  (XOAUTH2 against `go-sasl`, Gmail X-GM-EXT against `go-imap`);
  each carries a top-of-file provenance comment.
- Backends in v1: Fastmail JMAP + Gmail IMAP. No maildir/mbox/notmuch.
- `mail.Backend` is synchronous blocking; both packages call their
  libraries synchronously — no pump goroutine, no async bridge.
- `internal/ui/` follows the Elm architecture — invoke the
  `elm-conventions` skill before touching any file there. State
  in tea.Model structs; mutations only in Update; I/O only in
  tea.Cmd; children expose accessors, parents read after
  delegation (`App.deriveChromeFromAcct`). `tea.Msg` is reserved
  for cross-tree signals, never child→parent state mirrors.
- Idiomatic bubbletea is the default. UI uses `bubbles` components
  as primary analogues; deviations are ADR'd. `View()` self-enforces
  size via `clipPane`; renderers honor `width` via wordwrap + hardwrap;
  width math uses `displayCells(s, spuaCellWidth)` for icon-bearing
  strings and `lipgloss.Width` for icon-free strings (never `len()`);
  truncation of icon-bearing strings goes through `displayTruncate`.
  `lipgloss.JoinHorizontal`/`JoinVertical` are forbidden when
  `spuaCellWidth != 1`; use row-by-row `strings.Join` with pre-padded
  children (kept under both modes — see ADR-0084). Keys declared as
  `key.Binding`, dispatched via `key.Matches`; `WindowSizeMsg` handlers
  both `SetSize` children and forward the msg. Full contract in
  `docs/poplar/bubbletea-conventions.md`.
- Icon mode is resolved once at startup. `cmd/poplar/root.go` calls
  `term.HasNerdFont`, `term.MeasureSPUACells`, and `term.Resolve` to
  produce `(IconMode, spuaCellWidth)`. `ui.SetSPUACellWidth` is called
  before `tea.NewProgram`. The resolved `IconSet` is threaded into
  `ui.NewApp`. No runtime mode toggling.
- `internal/ui/icons.go` is the only place icon literals live.
  `SimpleIcons` runes are East Asian Width Na/N (`lipgloss.Width == 1`).
  `FancyIcons` runes are in `[U+F0000, U+FFFFD]`. Both class
  invariants are unit-tested.
- `App` constructs the model tree and threads `mail.Backend` and
  `*theme.CompiledTheme` into the components that need them.
  `AccountTab` holds the backend reference for tea.Cmd closures;
  `Viewer` holds the theme reference for markdown rendering.
  No component caches backend results as owned state.
- Account view is one pane. No focus cycling. `j/k` always
  navigates messages, `J/K` always navigates folders, every triage
  and reply key is always live.
- Config lives in `~/.config/poplar/accounts.toml`. Both
  `[[account]]` blocks and the `[ui]` table live in the same file;
  `config.ParseAccounts` and `config.LoadUI` decode them
  independently.
- Themes are compiled Go values in `internal/theme/` (15 themes,
  One Dark default). No runtime TOML, no glamour. Components style
  through the `Styles` struct from `theme.CompiledTheme`.
  `lipgloss.NewStyle()` is permitted only in `internal/ui/styles.go`
  and `internal/theme/palette.go`. Hex literals only in `themes.go`.
- The semantic map from palette slots to UI surfaces lives in
  `docs/poplar/styling.md`; update it before changing any color.
- Folder classification is a pure function:
  `mail.Classify([]Folder) []ClassifiedFolder`. Priority:
  `Folder.Role` → alias table → `Custom`. Provider folder names
  are normalized to canonical display names (Inbox, Sent, Trash,
  ...) regardless of JMAP/IMAP naming.
- Sidebar renders three folder groups in fixed order: Primary,
  Disposal, Custom. Separated by blank lines. No group headers.
  Groups are permanent — user config only ranks folders within
  their group.
- Nested folder names (containing `/`) render flat. The `/` in the
  display name is the only affordance. No tree, no expand/collapse.
- Compose (planned): pluggable behind an `Editor` interface. v1
  ships Catkin (native bubbletea editor); v1.1 adds neovim via
  `--embed` RPC. Compose renders inline — sidebar and chrome stay
  visible. No `tea.ExecProcess` terminal takeover.
- `mail.MessageInfo` carries `ThreadID` and `InReplyTo` on the
  wire. Depth is not a wire field — the UI derives it during the
  prefix walk. A non-threaded message is a thread of size 1 with
  `ThreadID == UID` and `InReplyTo == ""`.
- `mail.MessageInfo` carries `Date string` + `SentAt time.Time`.
  `SentAt` is authoritative for sorts + date-column rendering;
  `Date` is a legacy display fallback for fixtures predating
  `SentAt`. `lessMessage` falls back to `Date` lex only when
  `SentAt` is zero on both operands.
- Message list date column: `formatRelativeDate(t, now)` in
  `internal/ui/date_format.go`. Same calendar day → 12-hour time
  (`10:23 AM`); other day → `Mon 2006-01-02`; zero → empty. All in
  `now`'s location. `MessageList` snapshots `now` at construction +
  on `SetMessages`; `rebuild` precomputes `displayRow.dateText` so
  the render path is I/O-free.
- `MessageList` owns thread grouping + fold state. Holds `source
  []MessageInfo` plus derived `rows []displayRow` rebuilt by a
  group→sort→flatten pipeline. A transient `*threadNode` tree is
  built per bucket in `appendThreadRows` to compute box-drawing
  prefixes, then discarded — the renderer never sees the tree.
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
- Triage actions (delete/archive/star/read/move) are optimistic with
  a shared undo bar. `MessageList.Apply{Delete,Insert,Flag,Seen}` flip
  local state without firing Cmds; `AccountTab.dispatchTriage` (and
  `dispatchMoveFromPicker` for move) snapshots inverse data, applies
  the flip, exits visual mode, emits `triageStartedMsg` + forward Cmd
  via `buildTriageCmd` (or `buildTriageCmdWithDest` for move's dest).
  `App` owns `pendingAction` and schedules a
  `tea.Tick` for `[ui] undo_seconds` (default 6, clamped [2,30]).
  `u` fires `onUndo` + the saved inverse Cmd. A folder change
  commits (no inverse). An `ErrorMsg` runs `onUndo` before setting
  `lastErr` so a backend failure visibly reverts the flip. The
  chrome row above the status bar is shared with the error banner;
  error wins, then toast, else the row collapses
  (`App.chromeBannerRow`). `pendingAction.IsZero()` checks
  `op == ""`.
- `MessageList.ActionTargets()` is the source of truth for triage
  scope: if anything is marked, return marks in source order
  (mode-agnostic); otherwise cursor row, with WYSIWYG expansion to
  all thread UIDs on a folded thread root. `visualMode` controls
  input routing only (`Space` marks iff on); marks survive
  `ExitVisual` and are consumed by the next dispatch. Visual mode
  auto-exits on dispatch. Bulk star/read direction follows the
  cursor row.
- `ErrorMsg{Op, Err}` is the canonical Cmd error type. Every fallible
  `tea.Cmd` returns it with a short verb-phrase `Op` ("mark read",
  "fetch body"). `App` owns `lastErr` (last-write-wins). Banner is
  one foreground-only row above the status bar (`⚠ <Op>: <Err>`),
  truncated with `…`; account region shrinks one cell when shown so
  view height is unchanged. No key steal, dismiss, severity, queue.
  Part of the dimmed underlay while overlays are open.
- Spinner placeholders go through `NewSpinner(t)` (Dot, `FgDim`)
  in `internal/ui/styles.go`; shared across viewer/folder/send.
- Body content rendering caps at `maxBodyWidth = 72` cells; headers
  wrap at the panel width (uncapped). Outbound links are harvested
  by `content.RenderBodyWithFootnotes` into `[N]: <url>` rows below
  a rule; inline link text gets ` [^N]` glued to its last word with
  U+00A0. Short bare URLs (`Text == URL`, ≤30 cells) render inline
  without a marker.

## UX

- Poplar is opinionated and not configurable in v1. Users who want
  maximum configurability should use aerc or mutt.
- Vim-first keybindings: single-key motions, visual mode for multi-
  select. No multi-key sequences (one tea.KeyMsg per keypress).
- No `:` command mode. Every action is a single-key binding or a
  modal picker launched by a key.
- `q` exits the viewer when the viewer is open, quits poplar when
  on the account view. While the sidebar search shelf is non-idle,
  `q` is stolen and clears the search instead of quitting. While
  the help popover is open, `q` is swallowed (help is a view, not
  a state to escape). `?` opens the help popover; `?` or `Esc`
  closes it.
- App owns modal overlays via the same compose pattern: render
  underlying frame, dim via `DimANSI`, composite via `PlaceOverlay`
  (vendored from superfile, MIT) at the centered top-left from
  `centerOverlay`. While an overlay is open, `App.Update` short-
  circuits keys into it. Two overlays exist: help popover (`App`
  owns `helpOpen bool` + `help HelpPopover`; `viewerOpen` selects
  `HelpAccount` vs `HelpViewer` context) and link picker (`App`
  owns `linkPicker LinkPicker`; viewer-context-only).
- Help popover advertises the full planned keybinding vocabulary,
  not just currently-wired keys. Each row in the binding tables
  carries a `wired bool` flag. Wired rows: bright-bold key + dim
  desc. Unwired rows: dim throughout. Group headings stay bright.
  Later passes flip wired flags as bindings come online.
- Folder jumps use uppercase single keys:
  `I` Inbox, `D` Drafts, `S` Sent, `A` Archive, `X` Spam, `T`
  Trash. Shared with lowercase triage keys (`d` delete vs
  `D` drafts) without conflict.
- Threaded display is default-on. Per-folder `[ui.folders.<name>]
  threading = false` overrides to flat. No runtime toggle.
- Threads sort by latest activity (max date across the thread)
  in the folder's configured direction. Children inside a thread
  always sort chronologically ascending regardless of folder
  direction. Folder sort comes from `[ui.folders.<name>] sort`
  (`date-desc` default, `date-asc` opt-in).
- Thread root is the message with empty `InReplyTo`. Fallback
  for broken chains: earliest by date in the bucket; remaining
  orphans attach to the root as depth-1 children.
- Fold state is per-session, reset on every `SetMessages`
  (folder reload). Threads default expanded. The `[N] ` prefix
  badge replaces the box-drawing prefix on a collapsed root.
- `Space` toggles fold on the thread under the cursor (snaps to
  nearest visible row after fold; in visual-select mode toggles
  row selection instead). `F` is the bulk counterpart: folds every
  multi-message thread if any is unfolded, else unfolds everything.
- Message list encodes read state by brightness — unread sender
  is `FgBright` bold, unread subject is `FgBright`; read rows are
  `FgDim`. Hue is reserved for the cursor (`AccentPrimary`) and
  for the unread+flagged case (`ColorWarning`). Read-flagged rows
  dim their flag glyph along with the rest of the row.
- Command footer is the primary discoverability surface. Each hint
  carries a drop rank 0–10. When the terminal is too narrow, hints
  drop in descending rank order. Rank 0 (`? help`, `q quit`) never
  drops. Groups with no remaining hints collapse their preceding
  `┊` separator.
- Chrome is a three-sided frame: top `──┬──╮`, right `│`, bottom
  status bar `──┴──╯`. No left border.
- Connection state renders as shape + color + text for colorblind
  accessibility: `●` green connected, `◐` orange reconnecting,
  `○` red hollow offline.
- Search: `/` activates a 3-row shelf pinned to the bottom of the
  sidebar. Filter-and-hide: non-matching threads disappear; matching
  threads render fully expanded regardless of saved fold state
  (preserved, restored on `Esc`). `Esc` clears query + restores
  pre-search cursor. `Tab` cycles `[name]` (subject+sender) ↔ `[all]`
  (+date text). Case-insensitive substring; current folder only —
  folder jumps clear search. Fold keys inert while filter is
  committed.
- Modifier-free keybindings: user-facing actions never bind a
  Ctrl/Alt/Meta chord. Viewer scroll uses `j/k/Space/b/g/G`.
  `Ctrl-c` survives only as a terminal-kill alias on the Quit
  binding; never advertised. `pgup/pgdown` are not bound.
- `Enter` on the message list opens the selected message in the
  viewer. Unread → marked seen optimistically. `q`/`Esc` closes
  the viewer and the cursor stays on the same row. While the viewer
  is ready, `n`/`N` advances/retreats to the next visible message
  (skipping folded rows), reusing the same fetch + mark-read flow
  as `Enter`. Boundaries are inert; `n`/`N` are inert during
  `viewerLoading`.
- Viewer link launch: `1`–`9` opens the Nth harvested URL via
  `xdg-open` (fire-and-forget). `Tab` opens the `LinkPicker` modal
  overlay when ≥1 URL is harvested (inert otherwise). Picker is
  App-owned, viewer-context-only, mirrors the help-popover overlay
  pattern: `j/k` cursor, `Enter`/`1`–`9` launch+close, `Esc`/`Tab`
  close, `q` swallowed.
- Bare URL footnoting: `Link{Text: url, URL: url}` with
  `lipgloss.Width > 30` cells harvests into the footnote list with
  `trimURL(url) + nbsp + [^N]` inline. Short bare URLs pass through.
  `trimURL` strips scheme, keeps host (+port), appends
  `/<first-segment>` when present, appends `…` when anything was
  removed.

## Build & verification

- Makefile targets: `build`, `test`, `vet`, `lint`, `install`,
  `check`, `clean`. `make check` (vet+test) is the commit gate;
  `make install` writes to `~/.local/bin/`.
- Go module: `github.com/glw907/poplar`. `go.mod` floor is 1.26.0;
  workstation toolchain is 1.26.1.
- Skills: invoke `go-conventions` before any Go file,
  `elm-conventions` before any `internal/ui/` file, update
  `docs/poplar/styling.md` before any color/style change.
- Pass-end ritual lives in the `poplar-pass` skill (trigger:
  "continue development", "next pass", "finish pass", "ship pass").
- Live UI verification uses the tmux workflow in
  `.claude/docs/tmux-testing.md`.

## Decision index

Load the relevant ADR when you need the rationale behind an
invariant. ADR numbering is chronological.

| Invariant theme | ADRs |
|---|---|
| Monorepo, single binary | 0001, 0058 |
| Direct-on-libraries mail stack (no aerc fork) | 0002 (superseded by 0075), 0006 (superseded by 0075), 0008 (superseded by 0075), 0010 (superseded by 0075), 0012 (superseded by 0075), 0075 |
| Lipgloss + compiled themes, styling discipline | 0004, 0043, 0046 |
| JMAP + IMAP only, minimal account config | 0009, 0075 |
| Mail backend interface synchronous | 0010 (superseded by 0075), 0075 |
| Config layout, folder classifier, UI config | 0013, 0052, 0053 |
| Elm architecture in internal/ui/ | 0023, 0035, 0036, 0037, 0042, 0044, 0054, 0088 |
| Frame, chrome, status, footer | 0025, 0026, 0027, 0028, 0029, 0030, 0038 |
| Sidebar groups, nested indent, classification | 0018, 0019, 0034, 0049, 0050 |
| Message list, threading, fold | 0041, 0045, 0047, 0048, 0055, 0059, 0060, 0061, 0062, 0063 |
| Vim-first keybindings, no command mode, no multi-key, no modifiers (reading/nav surfaces; text-entry exempt per 0076) | 0015, 0024, 0051, 0068, 0076 |
| Compose, Catkin, editor interface, library foundation | 0031, 0032, 0033, 0076 |
| Per-screen prototype passes | 0022 (superseded by 0070), 0070 |
| Sidebar search shelf, filter-and-hide, thread-level | 0064 |
| Viewer prototype, footnote harvesting, optimistic mark-read, n/N nav, long-bare-URL footnoting | 0065, 0066, 0067, 0069, 0085, 0086 |
| Help popover modal, future-binding policy, overlay+dim, link picker | 0071 (superseded by 0082), 0072, 0082, 0087 |
| Error banner, ErrorMsg, shared spinner | 0073, 0074 |
| Optimistic triage with toast/undo, ActionTargets, visual mode, move picker | 0089, 0090, 0091 |
| Bubbletea conventions: research-grounded, lint hook, displayCells, key dispatch, WindowSizeMsg, displayCells-everywhere | 0077, 0078, 0079 (superseded by 0084), 0080, 0081, 0083 (narrowed by 0084) |
| Icon-mode policy: NF autodetect + CPR probe + simple/fancy tables | 0084 |
