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
  `internal/ui/` (App + App-owned chrome; bubbles-shaped subpackages
  `account`, `compose`, `helppopover`, `messagelist`, `movepicker`,
  `reader`, `sidebar`, plus `uicore` for shared chrome. ADRs 0161,
  0163),
  `internal/mail/` (`Backend`
  interface + classifier; `mail.FolderEntry` is the display
  projection threaded through sidebar/movepicker),
  `internal/mailjmap/` (Fastmail via
  `git.sr.ht/~rockorager/go-jmap`), `internal/mailimap/` (generic
  IMAP via `emersion/go-imap` v2; two physical connections per
  Backend — command + idle), `internal/mailauth/` (vendored XOAUTH2
  SASL snippet), `internal/config/` (`AccountConfig`,
  `UIConfig`, `LoadUI`, `Provider` registry), `internal/theme/`
  (compiled lipgloss themes), `internal/term/` (capability
  detection: `HasNerdFont`, `MeasureSPUACells`). `internal/filter/`,
  `internal/content/`, `internal/tidy/` await their consumers.
- Mail backends call upstream libraries directly. No aerc fork. The
  library family is emersion (`go-imap` v2, `go-message`, `go-smtp`,
  `go-sasl`, `go-webdav`, `go-vcard`) plus `rockorager/go-jmap`.
  Vendored snippets are MIT helpers (XOAUTH2 against `go-sasl`,
  Gmail X-GM-EXT against `go-imap`); each carries a top-of-file
  provenance comment. TCP keepalive uses stdlib `net.KeepAliveConfig`.
- Backends in v1: JMAP (`provider = "jmap"` / `"fastmail"`) and
  generic IMAP (`provider = "imap"` or one of the presets `yahoo`,
  `icloud`, `zoho`, `outlook`, `mailbox-org`, `posteo`, `runbox`,
  `gmx`, `protonmail`, `gmail`). Provider presets in `config.Providers` resolve at
  decode time to the canonical `imap`/`jmap` backend with
  host/port/URL/auth-hint filled in (and `InsecureTLS = true` on
  the `protonmail` preset for the local Bridge's self-signed
  loopback cert). Self-hosted IMAP uses explicit `host`/`port`
  plus `insecure-tls = true` for self-signed certs; the dial path
  surfaces a "set insecure-tls = true if self-signed" hint when
  TLS handshake fails on RFC 1918 / `.local` / `127.x` hosts and
  `InsecureTLS` is not already on. No maildir/mbox/notmuch.
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
  pre-marked messages. Gmail accounts (`GmailQuirks = true`) assert
  `X-GM-EXT-1` at Connect and route `Destroy` through `SELECT
  [Gmail]/Trash` first so EXPUNGE truly deletes; XOAUTH2 access
  tokens come from `password-cmd` with no internal refresh until
  Pass 9.6. SMTP is a third connection dialed lazily on first
  `Send` via `emersion/go-smtp`; the cached client is dropped on
  any send error so the next call redials. `Append(folder, mime,
  flags)` runs `APPEND` on the cmd connection. `mailimap.ProbeSMTP`
  is the connect-test surface for `poplar config check`. `dialRawTCP`
  in `auth.go` is the shared TCP-setup helper used by both IMAP and
  SMTP dials.

### Send + Append

- `mail.Backend.Send(env Envelope, mime []byte) error` and
  `Append(folder string, mime []byte, flags Flag) error` are the
  outbound primitives. `Envelope = { From, Rcpts }` is the
  RFC 5321 envelope; mime is pre-assembled by
  `compose.AssembleMIME`. `internal/mail/` does not import
  `internal/compose/`; bytes flow one way through the stack.
- JMAP `Send` batches `Email/import` (into the Sent mailbox) and
  `EmailSubmission/set` in one request, using the JMAP `#k1`
  creation reference so submission and Sent placement are atomic.
  `Identity/get` resolves identity IDs lazily and caches them on
  `Backend.identityIDs map[string]jmap.ID` keyed by lowercased
  email; one probe per cache miss populates the map for every
  identity the server returns. `Append` is the same shape minus
  the submission call.
- IMAP `Send` runs SMTP `MAIL`/`RCPT`/`DATA`; `Append` runs IMAP
  APPEND on the cmd connection. Sent placement is a separate
  `Append` issued by the caller (the cache outbox).
  SASL: plain (default), login, xoauth2.
- Cache outbox dispatches Send and Append. Schema v6 adds
  `outbox.payload BLOB` carrying assembled MIME bytes, locked in
  at queue time (no reassembly on dispatch).
  `(*Account).QueueSend(ctx, sentFolder, env, mime)` and
  `(*Account).QueueAppend(ctx, folder, flag, mime)` are the
  payload-bearing entry points; `SendArgs{Envelope}` /
  `AppendArgs{Flag}` carry the metadata in `outbox.args`. The
  drainer's `dispatch(args, row)` routes to `Backend.Send`/`Append`
  using `row.FolderName` / `row.Payload`. `revertOptimisticTx`
  no-ops on Send/Append, so `DiscardOp` works on conflicted rows.
  IMAP enqueues two ops (Send then Append-to-Sent); JMAP enqueues
  one Send (server lands the Sent copy atomically). Partial
  failure surfaces through the standard outbox visibility.

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
  hardwrap; width math uses `uicore.DisplayCells(s, uicore.SPUACellWidth())`
  for icon-bearing strings and `lipgloss.Width` for icon-free strings
  (never `len()`); truncation of icon-bearing strings goes through
  `uicore.DisplayTruncate` / `uicore.DisplayTruncateEllipsis`.
  `lipgloss.JoinHorizontal`/`JoinVertical` are
  forbidden when `spuaCellWidth != 1`; use row-by-row `strings.Join`
  with pre-padded children (kept under both modes — see ADR-0084).
  Keys declared as `key.Binding`, dispatched via `key.Matches`;
  `WindowSizeMsg` handlers both `SetSize` children and forward the
  msg. Full contract in `docs/poplar/bubbletea-conventions.md`.
- `internal/ui/` is the App parent plus seven bubbles-shaped
  subpackages (`account`, `compose`, `helppopover`, `messagelist`,
  `movepicker`, `reader`, `sidebar`) and the `uicore` sibling.
  Subpackages cannot import the parent. `uicore` holds shared
  chrome: `ErrorMsg`, `TriageOp` + `Triage*` constants,
  `ComputeLayout`, `NewSpinner`, `ModalShell`, `PlaceOverlay`,
  `DimANSI`, render primitives (`PadOrTruncate`, `TruncateToWidth`,
  `DisplayCells`, `DisplayTruncate`, `CenterOverlay`, `ApplyBg`,
  `FillRowToWidth`, …), and the `LayoutMode` / `IconSet` /
  `SearchMode` enums plus the `SimpleIcons`/`FancyIcons` tables.
  Each subpackage exposes a single `Model` + `New(...)` (sub-models
  like `sidebar.Column`, `sidebar.Search`, `reader.LinkPicker`,
  `reader.AttachPicker` are exported alongside). Per-subpackage
  `Styles` lives in that subpackage's `styles.go` with a
  `NewStyles(*theme.CompiledTheme)` constructor. Those `styles.go`
  files are the only places outside `internal/ui/styles.go` and
  `internal/theme/palette.go` permitted to call
  `lipgloss.NewStyle()`. Msg types live in the package that
  produces them: subpackage-private msgs are unexported and never
  cross the boundary; cross-boundary msgs are exported in
  `<subpkg>/msgs.go` and qualified at the call site
  (`account.TriageStartedMsg`, `account.CacheEventMsg`,
  `account.OpenConfirmEmptyMsg`, `account.EmptyFolderConfirmedMsg`,
  `account.FolderLoadedMsg`, `reader.BodyLoadedMsg`,
  `sidebar.ClearSearchMsg`). Reader/compose cmds emitting
  `uicore.ErrorMsg` and orchestrating App seams (`URLOpener`,
  `TidyFn`) live in `internal/ui/cmds.go`, accepting seams as
  function-typed parameters. `internal/ui/compose` shadows the
  `internal/compose` domain package: App-side imports use `uicompose`,
  and inside `internal/ui/compose/` the domain package is aliased
  `mailcompose`. ADRs 0161, 0162, 0163.
- `App` threads `*cache.Account` + `*theme.CompiledTheme` into the
  tree. `account.Model` holds the cache handle (backend reachable via
  `account.Model.Backend()` for `pumpUpdatesCmd`); reads come from
  `cache.Account.QueryFolder`/`ListFolders`, writes from
  `cache.Account.QueueOp`. `messagelist.Model` is presentation-only;
  `RefreshSource` re-reads cache state after every write, preserving
  cursor on UID. `reader.Model` holds the theme reference for
  markdown rendering. `mail.Backend` shrunk in cutover and Pass 8.5:
  `Mark{Read,Unread,Answered}`/`Delete`/`Search`/`Copy` are gone.
  `Flag(uids, flag, set)` is canonical; "delete" is a
  `MoveArgs{Dest: trash}` queued op. IMAP `Move` falls back to
  `cmd.Copy` when MOVE is absent (internal helper, not on interface).

### Config & theming

- Config lives in `~/.config/poplar/config.toml` (XDG on Linux and
  macOS, deliberately overriding Apple's Application Support
  default; `%APPDATA%\poplar\config.toml` on Windows). Both
  `[[account]]` blocks and the `[ui]` table live in the same file;
  `config.Load` (accounts) and `config.LoadUI` decode them
  independently. Path precedence: `--config` flag, `$POPLAR_CONFIG`,
  OS default, resolved by `config.Resolve`. The TOML key for the
  preset selector is `provider`.
- First-run flow: missing config writes `config.Template()` and
  returns `ErrFirstRun`; root exits 78 (EX_CONFIG). Legacy
  `accounts.toml` returns `ErrOldAccountsToml`. `password-cmd`
  resolves on first `Connect` and caches on the Backend.
  Validation errors carry `account "<name>" (provider = "<p>"):
  ...` context; unknown providers get a Levenshtein suggestion.
- `poplar config` subcommands: `init` (writes template, `--force`
  to overwrite), `check` (validate + connect-test each account
  sequentially — IMAP probe then `mailimap.ProbeSMTP`), `path`,
  `discover-folders` (merge server folder ordering into
  `[ui.folders]`).
- `[account.smtp]` is a TOML sub-table under each `[[account]]`.
  Presets fill canonical submission endpoints (465 implicit-TLS
  for gmail/fastmail/yahoo/zoho; 587 STARTTLS for outlook/icloud;
  protonmail bridge on loopback 1025 STARTTLS with `insecure-tls`).
  `Auth`/`Password`/`PasswordCmd` default to mirroring IMAP-side
  credentials. JMAP accounts ignore the block (submission rides
  the JMAP session). Validation requires `smtp.host` for
  `provider = "imap"` after preset resolution.
- `[[account.identity]]` carries ordered
  `[[account.identity.signature]]` sub-blocks.
  `AccountConfig.Identities` is always length >= 1; the legacy
  top-level `from` synthesizes one when blocks are absent. First-
  in-order is the default and syncs back into `AccountConfig.From`.
  Each signature sets exactly one of `text` or `file` (mutually
  exclusive); `file` resolves at config-load. `Signature.Text`
  always carries the RFC 3676 `"-- \n"` sentinel (prepended
  idempotently). `Signature.Name` is unique within its identity.
  ADR-0177.
- `[account.contacts]` is an optional TOML sub-table for CardDAV
  ingest (URL, username, password / password-cmd,
  default-addressbook, refresh-interval, insecure-tls). Credentials
  fall back to the parent `[[account]]` block when unset. URL must
  be https (or http with insecure-tls = true); refresh-interval
  ≥ 1m, default 15m. Password resolves via the shared
  `resolvePasswordCmd` helper at constructor time; on failure the
  account silently skips contact sync. Absent block → nil
  `Contacts` pointer → no sync. ADR-0175.
- Themes are compiled Go values in `internal/theme/` (15 themes,
  One Dark default). No runtime TOML, no glamour. Components style
  through the `Styles` struct from `theme.CompiledTheme`.
  `lipgloss.NewStyle()` is permitted only in
  `internal/theme/palette.go`, `internal/ui/styles.go`, and
  per-subpackage `styles.go` files (each subpackage's `Styles` is
  a narrow projection of `internal/ui.Styles` constructed by
  `NewStyles(*theme.CompiledTheme)`). Hex literals only in `themes.go`.
  The semantic map from palette slots to UI surfaces lives in
  `docs/poplar/styling.md`; update it before changing any color.

### Icon mode

- Icon mode is resolved once at startup. `cmd/poplar/root.go` calls
  `term.HasNerdFont`, `term.MeasureSPUACells`, and `term.Resolve` to
  produce `(IconMode, spuaCellWidth)`. `uicore.SetSPUACellWidth` is
  called before `tea.NewProgram`. The resolved `uicore.IconSet` is
  threaded into `ui.NewApp`. No runtime mode toggling.
- `internal/ui/uicore/layout.go` is the only place icon literals live.
  `uicore.SimpleIcons` runes are East Asian Width Na/N
  (`lipgloss.Width == 1`). `uicore.FancyIcons` runes are in
  `[U+F0000, U+FFFFD]`. Both class invariants are unit-tested.

### Catkin

Auto-loaded via `.claude/rules/catkin-invariants.md` when
editing `internal/catkin/` or planning passes. ADRs 0144–0147,
0149, 0150, 0152.

### Compose

- `internal/compose/` is the UI-free outbound-mail surface: the
  `Editor` seam (CatkinEditor wraps `catkin.Model`; v1.1 will add a
  neovim adapter), the `Draft` value type, pure
  `AssembleMIME(d, now)` (multipart/alternative text+html via
  `filter.MarkdownToHTML`; multipart/mixed when attachments are
  present), and `SeedReply`/`SeedReplyAll`/`SeedForward` parsing
  parent Message-Id and References from raw RFC 5322 bytes with
  depth-preserving `>` quoting. `gomail.Address` is the Draft
  address type; `content.ParseAddressList` is the shared list
  parser. `internal/filter` exposes `MarkdownBody`/`MarkdownToHTML` as the shared goldmark entries (Linkify + Table).
- `internal/ui/compose/` owns the live compose surface. `Dropdown`
  is a value-type sub-model on `compose.Model` for To/Cc/Bcc
  autocomplete; the `SuggestFn func(prefix string) []contacts.Suggestion`
  seam threads through `compose.New` / `compose.Open` and is wired
  to `App.suggestAddresses` (which delegates to
  `cache.Account.SuggestAddresses` — the recency-decayed query over
  `message_recipients` joined to the carded pool, capped at 7 rows).
  The dropdown renders only
  when focus is To/Cc/Bcc and the trailing fragment (text after
  the last comma, leading whitespace trimmed) has ≥ 2 chars and
  the seam returns rows. Up/Down wrap; Tab/Enter accept (rewrite
  the trailing fragment as `Name <email>, ` and clear); Esc
  dismisses. Splices positionally below the focused-header row.
  ADR-0174.

### Address book

- `internal/contacts/` is the UI-free contacts surface: value
  types (`Contact`/`Email`/`Phone`/`Suggestion`/`Kind`/`AddressBook`),
  the CardDAV `Client` (wraps `emersion/go-webdav/carddav` for
  discovery, multiget, sync-collection, CTAG via raw PROPFIND), the
  vCard parser (`emersion/go-vcard`), and the `Sync` orchestrator
  with its `Store` seam (`internal/cache` implements). `ClientConfig`
  is the runtime input to `NewClient`. `internal/ui/contacts/` is
  the address-book UI surface: per-package `Styles`, pure
  `RenderDetailCard`, fixture pool kept for the Contacts-mode
  browse list, and `Popover`/`Sidebar`/`List`/`Form` sub-models.
  Compose autocomplete and the `i`-popover read from
  `cache.Account.SuggestAddresses` / `LookupContact`; the fixture
  pool is gone from those paths. `i` opens the popover via
  `parseSender`↔`content.ParseAddressList`; `C`/`M` toggle Contacts
  mode (Sidebar T9 groups `ABC`…`WXYZ` with `J/K` group + `a`–`z`
  letter + `┃` tick | List `bubbles/viewport`, `n`/`e` emit
  `OpenFormMsg`, `D` opens delete-confirm | RenderDetailCard). Sidebar
  fixed at 14; sidebar/list rows pad to `contentH` so a tall Form
  doesn't collapse the body. Overlay cascade tail: confirm >
  conflict > outbox > help > linkpicker > attachpicker > movepicker
  > form > popover.
- `contacts.Form` is the contact edit sub-model — one value type,
  two render contexts. `fromPopover=true` renders as a ModalShell
  box; `fromPopover=false` renders body+footer without chrome so
  the Contacts-mode frame supplies borders. Focus is one `focusIdx`
  against `focusList()` (kind toggle, name fields per kind,
  `(input, cycler, ★, −)` quartets per email/phone row, add buttons,
  note, save destination); Tab/Shift+Tab cycle; Space/← /→ flips
  kind; ★ on row > 0 rotates to primary; − removes (disabled at one
  email). Dirty is `currentContact() != initial` (initial =
  post-construction snapshot). `Ctrl+S` validates (Person: First or
  Last; Business: Name; ≥1 email via `net/mail.ParseAddress`;
  saveIdx in range) and emits `ContactSaveMsg{Contact, SaveTo}`;
  `Esc` emits `ContactCancelMsg{Dirty}`. `D` (gated on
  `existingUID != ""` and on focus not being a text input)
  emits `OpenContactDeleteConfirmMsg{UID, DisplayName}`. App owns
  `form` + `pendingFormDiscard` + `pendingContactDelete`;
  Yes-confirm cascade orders form-discard before contact-delete
  before compose-save before empty-folder. Save: `queueContactPutCmd`
  → `PatchVCard` (existing) or `BuildVCard` (new, `uuid.NewString()`)
  → `QueueContactPut`. Delete: confirm-Yes → `queueContactDeleteCmd`
  → `QueueContactDelete`. Multi-book destination is post-1.0; the
  cmd uses `Account.DefaultBookHref` and ignores
  `ContactSaveMsg.SaveTo`. ADR-0176.

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
- `mail.ErrAuth` and `mail.ErrNotFound` are typed sentinels each
  backend wraps onto its native error shape via package-local
  `classifyErr`. JMAP uses `errors.As(*jmap.RequestError)` and
  routes 401/403 → `ErrAuth`, 404 → `ErrNotFound`. IMAP uses
  `errors.As(*imap.Error)` and routes
  `AUTHENTICATIONFAILED`/`AUTHORIZATIONFAILED`/`PRIVACYREQUIRED` →
  `ErrAuth`, `NONEXISTENT` → `ErrNotFound`. The cache drainer's
  conflict matrix routes via `errors.Is` against these sentinels
  (no substring matching).

Attachment wire shape and the picker/viewer surface live in
`.claude/rules/attachments-invariants.md` (auto-loaded on
`internal/ui/attachpicker*.go` / `internal/ui/viewer*.go` and on
plan/spec docs).

## Cache

The cache layer (per-account SQLite, schema versions, drainer,
outbox state machine, body + attachment storage) lives in
`.claude/rules/cache-invariants.md`. Auto-loaded when editing
`internal/cache/`, `cmd/poplar/cache*.go`, or planning passes.

## Build & verification

- Makefile targets: `build`, `test`, `vet`, `fmt-check`, `lint`,
  `install`, `check`, `clean`. `make check` runs fmt-check (`gofmt
  -l .`), vet, voice, and test as the commit gate. The voice step
  is `scripts/voice-check.sh`, a grep-tier scan for AI-tells T4,
  T10, T14, T16, T27, T28, T33, T35, T39, T40. T34 (semicolon
  clause-joiner) is voice-lens only as of ADR-0173. Semantic tells
  stay with the `/simplify` voice lens. The voice rules apply to
  all Claude-authored docs (skills, ADRs, plan docs, the catalogue
  itself), not only Go source. `make install` writes to `~/.local/bin/`.
- Go module: `github.com/glw907/poplar`. `go.mod` 1.26.0; toolchain 1.26.1.
- Skills: invoke `go-conventions` before any Go file,
  `elm-conventions` before any `internal/ui/` file, update
  `docs/poplar/styling.md` before any color/style change. Pass-end
  ritual lives in `poplar-pass`.
- Live UI verification uses tmux (`.claude/docs/tmux-testing.md`).
  80×24 is the polish bar; UI passes capture 80×24 and 120×40.

## Decisions

ADRs live in `docs/poplar/decisions/`. The themed index that maps
binding facts to their justifying ADRs is in
`docs/poplar/decisions/INDEX.md` — load it when you need to chase
rationale, not on every turn.
