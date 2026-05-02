# Poplar Invariants

Universal binding facts for the poplar codebase. Edited in place —
new facts replace or narrow old facts, they do not append. When a
pass changes a binding fact, update this file before committing.

Component- and UX-level invariants live in
`.claude/rules/ui-invariants.md` and load when editing
`internal/ui/`, planning a UI pass, or reading wireframes /
keybindings. The authoritative key map is
`docs/poplar/keybindings.md`.

Every fact here is codified in an ADR under `docs/poplar/decisions/`.
The decision index at the bottom maps each section's claims back to
the ADR(s) that justify them.

## Architecture

### Repo & libraries

- Poplar is a single-binary bubbletea terminal email client built
  from one Go module: `cmd/poplar`.
- Repository organization: `cmd/poplar/` (CLI wiring only),
  `internal/ui/` (tea.Model tree), `internal/mail/` (`Backend`
  interface + classifier), `internal/mailjmap/` (Fastmail via
  `git.sr.ht/~rockorager/go-jmap`), `internal/mailimap/` (generic
  IMAP via `emersion/go-imap` v2; two physical connections per
  Backend — command + idle), `internal/mailauth/` (vendored XOAUTH2
  + keepalive snippets), `internal/config/` (`AccountConfig`,
  `UIConfig`, `LoadUI`, `Provider` registry), `internal/theme/`
  (compiled lipgloss themes), `internal/term/` (capability
  detection: `HasNerdFont`, `MeasureSPUACells`). `internal/filter/`,
  `internal/content/`, `internal/tidy/` await their consumers.
- Mail backends call upstream libraries directly. No aerc fork. The
  library family is emersion (`go-imap` v2, `go-message`, `go-smtp`,
  `go-sasl`, `go-webdav`, `go-vcard`) plus `rockorager/go-jmap`.
  Vendored snippets are MIT-licensed helpers (XOAUTH2 against
  `go-sasl`, Gmail X-GM-EXT against `go-imap`); each carries a
  top-of-file provenance comment.
- Backends in v1: JMAP (`backend = "jmap"` / `"fastmail"`) and
  generic IMAP (`backend = "imap"` or one of the presets `yahoo`,
  `icloud`, `zoho`; `gmail` lands with X-GM-EXT support). Provider
  presets in `config.Providers` resolve at decode time to the
  canonical `imap`/`jmap` backend with host/port/URL/auth-hint
  filled in. Self-hosted IMAP uses explicit `host`/`port` plus
  `insecure-tls = true` for self-signed certs. No
  maildir/mbox/notmuch.
- `mail.Backend` is synchronous blocking; both packages call their
  libraries synchronously — no pump goroutine, no async bridge.
- IMAP backend invariants: UIDPLUS is required at Connect (asserted
  in `capSet`). MOVE / SPECIAL-USE / IDLE are negotiated; absence
  triggers documented fallbacks (COPY+STORE+EXPUNGE for Move, name-
  alias classification for SPECIAL-USE, 30s STATUS-poll for IDLE).
  The idle goroutine refreshes IDLE every 9 minutes (well under the
  RFC 2177 29-minute cap), reconnects with exponential backoff
  mirroring `mailjmap.pushLoop`, and emits `mail.Update` values on
  the shared updates channel. `Destroy` issues
  `UID STORE +FLAGS.SILENT (\Deleted)` then `UID EXPUNGE <uids>`,
  matching ADR-0092 semantics with no risk of expunging unrelated
  pre-marked messages.

### Elm architecture & idiomatic bubbletea

- `internal/ui/` follows the Elm architecture — invoke the
  `elm-conventions` skill before touching any file there. State in
  tea.Model structs; mutations only in Update; I/O only in tea.Cmd;
  children expose accessors, parents read after delegation
  (`App.deriveChromeFromAcct`). `tea.Msg` is reserved for
  cross-tree signals, never child→parent state mirrors.
- Idiomatic bubbletea is the default. UI uses `bubbles` components
  as primary analogues; deviations are ADR'd. `View()` self-enforces
  size via `clipPane`; renderers honor `width` via wordwrap +
  hardwrap; width math uses `displayCells(s, spuaCellWidth)` for
  icon-bearing strings and `lipgloss.Width` for icon-free strings
  (never `len()`); truncation of icon-bearing strings goes through
  `displayTruncate`. `lipgloss.JoinHorizontal`/`JoinVertical` are
  forbidden when `spuaCellWidth != 1`; use row-by-row `strings.Join`
  with pre-padded children (kept under both modes — see ADR-0084).
  Keys declared as `key.Binding`, dispatched via `key.Matches`;
  `WindowSizeMsg` handlers both `SetSize` children and forward the
  msg. Full contract in `docs/poplar/bubbletea-conventions.md`.
- `App` constructs the model tree and threads `mail.Backend` and
  `*theme.CompiledTheme` into the components that need them.
  `AccountTab` holds the backend reference for tea.Cmd closures;
  `Viewer` holds the theme reference for markdown rendering. No
  component caches backend results as owned state.

### Config & theming

- Config lives in `~/.config/poplar/accounts.toml`. Both
  `[[account]]` blocks and the `[ui]` table live in the same file;
  `config.ParseAccounts` and `config.LoadUI` decode them
  independently.
- Themes are compiled Go values in `internal/theme/` (15 themes,
  One Dark default). No runtime TOML, no glamour. Components style
  through the `Styles` struct from `theme.CompiledTheme`.
  `lipgloss.NewStyle()` is permitted only in `internal/ui/styles.go`
  and `internal/theme/palette.go`. Hex literals only in `themes.go`.
  The semantic map from palette slots to UI surfaces lives in
  `docs/poplar/styling.md`; update it before changing any color.

### Icon mode

- Icon mode is resolved once at startup. `cmd/poplar/root.go` calls
  `term.HasNerdFont`, `term.MeasureSPUACells`, and `term.Resolve` to
  produce `(IconMode, spuaCellWidth)`. `ui.SetSPUACellWidth` is
  called before `tea.NewProgram`. The resolved `IconSet` is threaded
  into `ui.NewApp`. No runtime mode toggling.
- `internal/ui/icons.go` is the only place icon literals live.
  `SimpleIcons` runes are East Asian Width Na/N
  (`lipgloss.Width == 1`). `FancyIcons` runes are in
  `[U+F0000, U+FFFFD]`. Both class invariants are unit-tested.

## Mail model

- Folder classification is a pure function:
  `mail.Classify([]Folder) []ClassifiedFolder`. Priority:
  `Folder.Role` → alias table → `Custom`. Provider folder names are
  normalized to canonical display names (Inbox, Sent, Trash, …)
  regardless of JMAP/IMAP naming.
- `mail.MessageInfo` carries `ThreadID` and `InReplyTo` on the wire.
  Depth is not a wire field — the UI derives it during the prefix
  walk. A non-threaded message is a thread of size 1 with
  `ThreadID == UID` and `InReplyTo == ""`.
- `mail.MessageInfo` carries `Date string` + `SentAt time.Time`.
  `SentAt` is authoritative for sorts + date-column rendering;
  `Date` is a legacy display fallback for fixtures predating
  `SentAt`. `lessMessage` falls back to `Date` lex only when
  `SentAt` is zero on both operands.
- `mail.Backend.Destroy(uids)` is the irreversible permanent-delete
  primitive (no inverse). Empty input is a no-op. JMAP impl issues
  `Email/set { destroy }` and treats `notFound` as success
  (idempotent). IMAP impl issues `UID STORE +FLAGS.SILENT (\Deleted)`
  then `UID EXPUNGE <uids>`, scoped by UIDPLUS so unrelated
  pre-marked messages are unaffected.

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
  `.claude/docs/tmux-testing.md`. 80×24 is the design polish bar:
  every UI surface must look intentional at the default-launch
  terminal size on every VT100-lineage terminal. Below 80, rendering
  is best-effort. UI passes capture both 80×24 and 120×40.

## Decision index

Load the relevant ADR when you need the rationale behind an
invariant. ADR numbering is chronological.

| Invariant theme | ADRs |
|---|---|
| Monorepo, single binary | 0001, 0058 |
| Direct-on-libraries mail stack (no aerc fork) | 0002 (superseded by 0075), 0006 (superseded by 0075), 0008 (superseded by 0075), 0010 (superseded by 0075), 0012 (superseded by 0075), 0075 |
| Lipgloss + compiled themes, styling discipline | 0004, 0043, 0046 |
| JMAP + IMAP only, minimal account config | 0009, 0075, 0098, 0101 |
| Mail backend interface synchronous | 0010 (superseded by 0075), 0075, 0099 |
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
| Permanent-delete primitive, retention sweep, manual empty + ConfirmModal | 0092, 0093, 0094, 0100 |
| Bubbletea conventions: research-grounded, lint hook, displayCells, key dispatch, WindowSizeMsg, displayCells-everywhere | 0077, 0078, 0079 (superseded by 0084), 0080, 0081, 0083 (narrowed by 0084) |
| Icon-mode policy: NF autodetect + CPR probe + simple/fancy tables | 0084 |
| Path-scoped UI rule (split from invariants) | 0095 |
| Responsive sidebar; 80×24 polish bar | 0096, 0097 |
