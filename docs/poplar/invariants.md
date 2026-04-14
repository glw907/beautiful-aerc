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
- Root model owns `mail.Backend` and `theme.CompiledTheme`.
  Children hold a reference to the backend only when they need it
  to construct tea.Cmd closures; they never cache backend results
  as owned state.
- Account view is one pane. No focus cycling. `j/k` always
  navigates messages, `J/K` always navigates folders, every triage
  and reply key is always live.
- Config lives in `~/.config/poplar/accounts.toml`. Both
  `[[account]]` blocks and the `[ui]` table live in the same file;
  `config.ParseAccounts` and `config.LoadUI` decode them
  independently.
- Themes are compiled Go values in `internal/theme/` (15 themes,
  One Dark default). No runtime TOML, no glamour. Components style
  through the `Styles` struct populated from `theme.CompiledTheme`
  — no direct `lipgloss.NewStyle()` calls, no hardcoded hex
  values.
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
- Nested folder names (containing `/`) get one extra leading space
  of indent in the sidebar. Max depth 3. No tree, no
  expand/collapse.
- Compose editor is pluggable behind an `Editor` interface. v1
  ships Catkin (native bubbletea editor, `catkin/` package, no
  poplar dependencies); v1.1 adds neovim via `--embed` RPC.
  Compose renders inline in the right panel — sidebar and chrome
  stay visible. No `tea.ExecProcess` terminal takeover.
- `mail.MessageInfo` carries `ThreadID` and `InReplyTo` on the
  wire. Depth is not a wire field — the UI derives it during the
  prefix walk. A non-threaded message is a thread of size 1 with
  `ThreadID == UID` and `InReplyTo == ""`.
- `MessageList` owns thread grouping and fold state. It holds
  `source []MessageInfo` (the raw backend payload) alongside a
  derived `rows []displayRow` rebuilt by a group→sort→flatten
  pipeline. A transient `*threadNode` tree is built per bucket
  inside `appendThreadRows` only to compute box-drawing prefixes,
  then discarded — the renderer never sees the tree.

## UX

- Poplar is opinionated and not configurable in v1. Users who want
  maximum configurability should use aerc or mutt.
- Vim-first keybindings: single-key motions, visual mode for
  multi-select. No multi-key sequences. Bubbletea sends one
  tea.KeyMsg per keypress.
- No `:` command mode. Every action is a single-key binding or a
  modal picker launched by a key.
- `q` exits the viewer when the viewer is open, quits poplar when
  on the account view. `?` opens the help popover.
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
  mode (Pass 6) `Space` toggles row selection instead. `F` folds
  every multi-message thread; `U` unfolds all.
- Message list encodes read state by brightness (`FgBright` bold
  for unread, `FgDim` for read). Hue is reserved for the cursor
  (`AccentPrimary`) and for the unread+flagged case
  (`ColorWarning`). Read-flagged rows dim their flag glyph along
  with the rest of the row.
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

## Build & verification

- Single Makefile target set: `build`, `test`, `vet`, `lint`,
  `install`, `check`, `clean`.
- `make check` (vet + test) is the gate before any commit.
- `make install` places the `poplar` binary in `~/.local/bin/`.
- Go module: `github.com/glw907/poplar`. Go version in `go.mod`
  matches the installed toolchain (1.26.1).
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
| Vim-first keybindings, no command mode, no multi-key | 0015, 0024, 0051 |
| Compose, Catkin, editor interface | 0031, 0032, 0033 |
| Per-screen prototype passes | 0022 |
