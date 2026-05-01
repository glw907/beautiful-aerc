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
  `internal/mailjmap/` implements `Backend` against
  `git.sr.ht/~rockorager/go-jmap` (Fastmail). `internal/mailimap/`
  implements `Backend` against `github.com/emersion/go-imap` v1
  (Gmail). `internal/mailauth/` vendors small XOAUTH2 + TCP
  keepalive snippets with provenance comments.
  `internal/config/` holds `AccountConfig`, `UIConfig`, and
  `LoadUI`. `internal/theme/` holds compiled lipgloss themes.
  `internal/term/` handles terminal capability detection
  (`HasNerdFont`, `MeasureSPUACells`). `internal/filter/`,
  `internal/content/`, `internal/tidy/` await their consumers.
- Mail backends call upstream libraries directly. No aerc fork.
  The library family is emersion (`go-imap` v1, `go-message`,
  `go-smtp`, `go-sasl`, `go-webdav`, `go-vcard`) plus
  `rockorager/go-jmap`. Vendored snippets are MIT-licensed helpers
  (XOAUTH2 against `go-sasl`, Gmail X-GM-EXT against `go-imap`);
  each carries a top-of-file provenance comment.
- Backends supported in v1: Fastmail JMAP and Gmail IMAP. No
  maildir, mbox, or notmuch.
- The `mail.Backend` interface is synchronous blocking. Both
  backend packages call their underlying libraries synchronously
  — no pump goroutine, no async-to-sync bridge.
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
- `mail.MessageInfo` carries both `Date string` and
  `SentAt time.Time`. `SentAt` is the authoritative instant — used
  for every sort comparison and for rendering the date column.
  `Date` is a legacy wire field kept only as a display fallback for
  test fixtures that predate `SentAt`; real workers must populate
  `SentAt`. The UI sort helper `lessMessage` falls back to `Date`
  lex comparison only when `SentAt` is zero on both operands.
- Message list date column formatting lives in
  `internal/ui/date_format.go` as `formatRelativeDate(t, now)`.
  Same calendar day as `now` → 12-hour time (e.g. `10:23 AM`); any
  other day → `Mon 2006-01-02`; zero time → empty. All in `now`'s
  location. `MessageList` snapshots `now` at construction and on
  `SetMessages`; `rebuild` precomputes `displayRow.dateText` so the
  render path does no I/O and no per-frame formatting.
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
  wrap at the panel content width (uncapped). Outbound links are
  harvested by `content.RenderBodyWithFootnotes` into `[N]: <url>`
  rows below a horizontal rule; inline link text gets ` [^N]` glued
  to its last word with U+00A0 so wrap can never orphan the marker.
  Short bare URLs (`Text == URL`, ≤30 cells) render inline in link
  style without a marker.

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
- Search is activated by `/` from the account view. The search
  shelf is a 3-row region pinned to the bottom of the sidebar
  column. Filter-and-hide: non-matching threads disappear; matching
  threads render fully expanded (root + all children) regardless of
  saved fold state, which is preserved unmutated and restored on
  `Esc`. `Esc` clears the query and restores the pre-search cursor
  row.
- Search modes cycle between `[name]` (subject + sender) and `[all]`
  (subject + sender + date text) via `Tab` while focused. Case-
  insensitive substring; current folder only — folder jumps clear
  the search. Fold keys are no-ops while filter is committed.
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
- Viewer link launch: `1`–`9` open the Nth harvested URL via
  `xdg-open` (fire-and-forget; `xdg-open` itself detaches and exit
  status is unreliable). `Tab` opens the `LinkPicker` modal overlay
  when at least one URL is harvested (inert otherwise). The picker
  is App-owned, viewer-context-only, mirrors the help-popover
  overlay pattern (centerOverlay + DimANSI + PlaceOverlay): `j/k`
  cursor, `Enter`/`1`-`9` launch + close, `Esc`/`Tab` close, `q`
  swallowed. Index column is right-aligned with leading-space pad;
  inline URL truncated to 50 cells; 2-row preview footer wraps the
  full URL.
- Bare URL footnoting: a `Link{Text: url, URL: url}` span whose
  `lipgloss.Width(URL) > 30` cells is harvested into the footnote
  list with a `trimURL(url) + nbsp + [^N]` inline form. Short bare
  URLs pass through unchanged. `trimURL` strips the scheme, keeps
  host (with port), and appends `/<first-segment>` when present;
  appends `…` when anything was removed.

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
| Bubbletea conventions: research-grounded, lint hook, displayCells, key dispatch, WindowSizeMsg, displayCells-everywhere | 0077, 0078, 0079 (superseded by 0084), 0080, 0081, 0083 (narrowed by 0084) |
| Icon-mode policy: NF autodetect + CPR probe + simple/fancy tables | 0084 |
