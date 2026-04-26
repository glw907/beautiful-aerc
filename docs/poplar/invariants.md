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
- Repository organization: `cmd/poplar/` holds CLI wiring only.
  `internal/ui/` holds the tea.Model tree. `internal/mail/` holds
  the `Backend` interface and the folder classifier.
  `internal/mailworker/` holds forked aerc IMAP + JMAP worker code,
  annotated with provenance comments. `internal/mailjmap/` holds
  the async→sync adapter that bridges the worker to the Backend
  interface. `internal/config/` holds `AccountConfig`, `UIConfig`,
  and `LoadUI`. `internal/theme/` holds compiled lipgloss themes.
  `internal/filter/`, `internal/content/`, `internal/tidy/` are
  library packages awaiting their poplar consumers.
- Workers are forked from aerc
  (`git.sr.ht/~rjarry/aerc`) on 2026-04-09. The fork lives under
  `internal/mailworker/`. Upstream fixes are cherry-picked, never
  `go get -u`'d.
- Backends supported in v1: Fastmail JMAP and Gmail IMAP. No
  maildir, mbox, or notmuch.
- The `mail.Backend` interface is synchronous blocking. The JMAP
  adapter bridges the forked worker's async channels to blocking
  calls via a pump goroutine.
- `internal/ui/` follows the Elm architecture — invoke the
  `elm-conventions` skill before touching any file there. All
  state lives in tea.Model structs; mutations happen only in
  Update; I/O only in tea.Cmd; children signal parents via Msg
  types; shared state is hoisted to the root.
- `App` constructs the model tree and threads `mail.Backend` and
  `*theme.CompiledTheme` into the components that need them.
  `AccountTab` holds the backend reference for building tea.Cmd
  closures; `Viewer` holds the theme reference for rendering
  markdown blocks. No component caches backend results as owned
  state.
- Account view is one pane. No focus cycling. `j/k` always
  navigates messages, `J/K` always navigates folders, every triage
  and reply key is always live.
- Config lives in `~/.config/poplar/accounts.toml`. Both
  `[[account]]` blocks and the `[ui]` table live in the same file;
  `config.ParseAccounts` and `config.LoadUI` decode them
  independently.
- Themes are compiled Go values in `internal/theme/` (15 themes,
  One Dark default). No runtime TOML, no glamour. Components style
  through the `Styles` struct populated from `theme.CompiledTheme`.
  `lipgloss.NewStyle()` is permitted only in `internal/ui/styles.go`
  (the `Styles` factory) and `internal/theme/palette.go` (the
  `Palette → CompiledTheme` step). Hex literals appear only in
  `internal/theme/themes.go` palette definitions.
- The semantic map from palette slots to UI surfaces lives in
  `docs/poplar/styling.md`. Before changing a color, the doc is
  updated first.
- Folder classification is a pure function:
  `mail.Classify([]Folder) []ClassifiedFolder`. Priority:
  `Folder.Role` → alias table → `Custom`. Provider folder names
  are normalized to canonical display names (Inbox, Sent, Trash,
  ...) regardless of JMAP/IMAP naming.
- Sidebar renders three folder groups in fixed order: Primary,
  Disposal, Custom. Separated by blank lines. No group headers.
  Groups are permanent — user config only ranks folders within
  their group.
- Nested folder names (containing `/`) render flat — no extra
  indent vs. top-level folders. The `/` in the display name is
  the only affordance. No tree, no expand/collapse.
- Compose (planned, Pass 9): pluggable behind an `Editor`
  interface. v1 will ship Catkin (native bubbletea editor,
  `catkin/` package, no poplar dependencies); v1.1 will add
  neovim via `--embed` RPC. Compose renders inline in the right
  panel — sidebar and chrome stay visible. No `tea.ExecProcess`
  terminal takeover.
- `mail.MessageInfo` carries `ThreadID` and `InReplyTo` on the
  wire. Depth is not a wire field — the UI derives it during the
  prefix walk. A non-threaded message is a thread of size 1 with
  `ThreadID == UID` and `InReplyTo == ""`.
- `mail.MessageInfo` carries both `Date string` and
  `SentAt time.Time`. `SentAt` is the authoritative instant — used
  for every sort comparison and for rendering the date column.
  `Date` is a legacy wire field kept only as a display fallback for
  test fixtures that predate `SentAt`; real workers must populate
  `SentAt`. The UI sort helper `lessMessage` falls back to `Date`
  lex comparison only when `SentAt` is zero on both operands.
- Message list date column formatting lives in
  `internal/ui/date_format.go` as `formatRelativeDate(t, now)`.
  Same calendar day as `now` → 12-hour time (e.g. `10:23 AM`).
  Any other day → `Mon 2006-01-02` (3-letter weekday + ISO date).
  Zero time → empty string. Both the same-day check and the
  returned string are in `now`'s location. `MessageList` captures
  a clock snapshot into its `now` field at construction and on
  every `SetMessages`, and `rebuild` precomputes the display
  string into each `displayRow.dateText` so the render path does
  no I/O and no per-frame formatting.
- `MessageList` owns thread grouping and fold state. It holds
  `source []MessageInfo` (the raw backend payload) alongside a
  derived `rows []displayRow` rebuilt by a group→sort→flatten
  pipeline. A transient `*threadNode` tree is built per bucket
  inside `appendThreadRows` only to compute box-drawing prefixes,
  then discarded — the renderer never sees the tree.
- The `Viewer` is an `AccountTab` child that owns no backend
  reference. Body fetch and mark-read Cmds are constructed at
  `AccountTab` and a `bodyLoadedMsg` carries parsed blocks back.
  `AccountTab` drops stale `bodyLoadedMsg` events by comparing
  against `viewer.CurrentUID()`. Phases: closed → loading (spinner
  placeholder) → ready (headers pinned + body in `bubbles/viewport`)
  → closed. While the viewer is open, every key routes there
  first; search keys and folder jumps are inert.
- Mark-read on viewer open is optimistic: `MessageList.MarkSeen`
  flips the local seen flag immediately and the backend `MarkRead`
  Cmd runs in parallel. Failures surface via `ErrorMsg` into the
  App-owned banner.
- `ErrorMsg{Op string; Err error}` is the canonical Cmd error type.
  Every poplar `tea.Cmd` that can fail returns `ErrorMsg` with a
  short verb-phrase `Op` ("mark read", "fetch body", "open folder").
  `App` owns `lastErr ErrorMsg`; `App.Update` intercepts every
  `ErrorMsg` and stores it (last-write-wins). The banner is one
  foreground-only row above the status bar (`⚠ <Op>: <Err>`),
  truncated to width with `…`. When `lastErr.Err != nil` the account
  region shrinks by one cell so total view height is unchanged. The
  banner is chrome — it does not steal keys. Help short-circuits
  `View`, so the banner is hidden along with everything else while a
  modal is open. No dismiss key, no severity, no queue in v1.
- The bubbles/spinner placeholder is constructed via the shared
  `NewSpinner(t *theme.CompiledTheme)` helper in
  `internal/ui/styles.go`: Dot variant, `FgDim` foreground. Every
  placeholder spinner (viewer load now; folder load and send progress
  later) goes through this constructor.
- Body content rendering caps at `maxBodyWidth = 72` cells.
  Headers wrap at the panel content width (uncapped). Outbound
  links are harvested by `content.RenderBodyWithFootnotes` into
  `[N]: <url>` rows below a horizontal rule; inline link text gets
  ` [^N]` glued to its last word with U+00A0 so wrap can never
  orphan the marker. Auto-linked bare URLs (`Text == URL`) render
  inline in link style without a marker.

## UX

- Poplar is opinionated and not configurable in v1. Users who want
  maximum configurability should use aerc or mutt.
- Vim-first keybindings: single-key motions, visual mode for
  multi-select. No multi-key sequences. Bubbletea sends one
  tea.KeyMsg per keypress.
- No `:` command mode. Every action is a single-key binding or a
  modal picker launched by a key.
- `q` exits the viewer when the viewer is open, quits poplar when
  on the account view. While the sidebar search shelf is non-idle,
  `q` is stolen and clears the search instead of quitting. While
  the help popover is open, `q` is swallowed (help is a view, not
  a state to escape). `?` opens the help popover; `?` or `Esc`
  closes it.
- The help popover is the first modal overlay. `App` owns
  `helpOpen bool` and `help HelpPopover`; `viewerOpen` selects the
  context (`HelpAccount` vs `HelpViewer`) at open time. While
  `helpOpen` is true, `App.Update` short-circuits all keys other
  than `?`/`Esc` (no delegation to children) and `App.View` returns
  `m.help.View(width, height)` directly — the underlying account
  layout is skipped. `HelpPopover` is render-only (no `Init`/
  `Update`); centering is `lipgloss.Place`. No background dim in
  v1.
- Help popover advertises the full planned keybinding vocabulary,
  not just currently-wired keys. Each row in the static binding
  tables (`accountGroups`, `viewerGroups`,
  `accountBottomHints`, `viewerBottomHints`) carries a `wired bool`
  flag. Wired rows: bright-bold key + dim description. Unwired
  rows: entire row dim, no bold, no glyph. Group headings stay
  bright regardless. The dim/bright contrast is the future-binding
  signal; later passes flip the flag as bindings come online.
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
- `Space` toggles fold on the thread under the cursor (operates
  on the thread root if the cursor is on a child; cursor snaps
  to the nearest visible row after fold). Inside visual-select
  mode (Pass 6) `Space` toggles row selection instead. `F` is
  the bulk counterpart: it folds every multi-message thread if
  any is currently unfolded, otherwise unfolds everything. Mixed
  state collapses on first press — reach fully-unfolded with a
  second press.
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
- Search is activated by `/` from the account view. The search
  shelf is a 3-row region pinned to the bottom of the sidebar
  column. Filter-and-hide: non-matching threads disappear; matching
  threads render fully expanded (root + all children) regardless of
  saved fold state, which is preserved unmutated and restored on
  `Esc`. `Esc` clears the query and restores the pre-search cursor
  row.
- Search modes cycle between `[name]` (subject + sender) and
  `[all]` (subject + sender + date text) via `Tab` while the prompt
  is focused. Case-insensitive substring match. Scope is the
  current folder only — folder jumps (`I/D/S/A/X/T`, `J/K`) clear
  the active search. Fold keys (`Space/F/U`) are no-ops while a
  filter is committed.
- Modifier-free keybindings: user-facing actions never bind a
  Ctrl/Alt/Meta chord. Viewer scroll uses `j/k/Space/b/g/G`.
  `Ctrl-c` survives only as a terminal-kill alias on the Quit
  binding; never advertised. `pgup/pgdown` are not bound.
- `Enter` on the message list opens the selected message in the
  viewer. Unread → marked seen optimistically. `q`/`Esc` closes
  the viewer and the cursor stays on the same row.
- Viewer link launch: `1`–`9` open the Nth harvested URL via
  `xdg-open` (fire-and-forget; `xdg-open` itself detaches and exit
  status is unreliable). `Tab` is reserved for the link picker
  (Pass 2.5b-4b) — a no-op in Pass 2.5b-4.

## Build & verification

- Single Makefile target set: `build`, `test`, `vet`, `lint`,
  `install`, `check`, `clean`.
- `make check` (vet + test) is the gate before any commit.
- `make install` places the `poplar` binary in `~/.local/bin/`.
- Go module: `github.com/glw907/poplar`. Go version in `go.mod` is
  the minimum supported floor (1.26.0); the workstation toolchain is
  1.26.1.
- Before writing any Go code, invoke the `go-conventions` skill.
- Before touching `internal/ui/`, invoke the `elm-conventions`
  skill.
- Before changing any color or style, update
  `docs/poplar/styling.md` first.
- Pass-end ritual lives in the `poplar-pass` skill. Trigger
  phrases: "continue development", "next pass", "finish pass",
  "ship pass".
- Live verification of UI renders uses the tmux testing workflow
  in `.claude/docs/tmux-testing.md`.

## Decision index

Load the relevant ADR when you need the rationale behind an
invariant. ADR numbering is chronological.

| Invariant theme | ADRs |
|---|---|
| Monorepo, single binary, fork policy | 0001, 0002, 0058 |
| Lipgloss + compiled themes, styling discipline | 0004, 0043, 0046 |
| JMAP + IMAP only, minimal account config | 0008, 0012, 0009 |
| Mail backend adapter synchronous | 0011 |
| Config layout, folder classifier, UI config | 0013, 0052, 0053 |
| Elm architecture in internal/ui/ | 0023, 0035, 0036, 0037, 0042, 0044, 0054 |
| Frame, chrome, status, footer | 0025, 0026, 0027, 0028, 0029, 0030, 0038 |
| Sidebar groups, nested indent, classification | 0018, 0019, 0034, 0049, 0050 |
| Message list, threading, fold | 0041, 0045, 0047, 0048, 0055, 0059, 0060, 0061, 0062, 0063 |
| Vim-first keybindings, no command mode, no multi-key, no modifiers | 0015, 0024, 0051, 0068 |
| Compose, Catkin, editor interface | 0031, 0032, 0033 |
| Per-screen prototype passes | 0022 (superseded by 0070), 0070 |
| Sidebar search shelf, filter-and-hide, thread-level | 0064 |
| Viewer prototype, footnote harvesting, optimistic mark-read | 0065, 0066, 0067, 0069 |
| Help popover modal, future-binding policy | 0071, 0072 |
| Error banner, ErrorMsg, shared spinner | 0073, 0074 |
