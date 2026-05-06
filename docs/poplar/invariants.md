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
  `Identity/get` resolves the identity ID on first Send and caches
  it on the Backend. `Append` is the same shape minus the
  submission call.
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
  hardwrap; width math uses `displayCells(s, spuaCellWidth)` for
  icon-bearing strings and `lipgloss.Width` for icon-free strings
  (never `len()`); truncation of icon-bearing strings goes through
  `displayTruncate`. `lipgloss.JoinHorizontal`/`JoinVertical` are
  forbidden when `spuaCellWidth != 1`; use row-by-row `strings.Join`
  with pre-padded children (kept under both modes — see ADR-0084).
  Keys declared as `key.Binding`, dispatched via `key.Matches`;
  `WindowSizeMsg` handlers both `SetSize` children and forward the
  msg. Full contract in `docs/poplar/bubbletea-conventions.md`.
- `App` threads `*cache.Account` + `*theme.CompiledTheme` into the
  tree. `AccountTab` holds the cache handle (backend reachable via
  `AccountTab.Backend()` for `pumpUpdatesCmd`); reads come from
  `cache.Account.QueryFolder`/`ListFolders`, writes from
  `cache.Account.QueueOp`. `MessageList` is presentation-only —
  `RefreshSource` re-reads cache state after every write,
  preserving cursor on UID. `Viewer` holds the theme reference for
  markdown rendering. `mail.Backend` was shrunk in cutover and
  trimmed further in Pass 8.5:
  `MarkRead`/`MarkUnread`/`MarkAnswered`/`Delete`/`Search`/`Copy`
  are all gone. `Flag(uids, flag, set)` is canonical and "delete"
  is a `MoveArgs{Dest: trash}` queued op. IMAP `Move` still uses
  the lower-level `cmd.Copy` as its no-`MOVE` fallback (internal
  helper, not on the interface).

### Config & theming

- Config lives in `~/.config/poplar/config.toml` (XDG on Linux and
  macOS, deliberately overriding Apple's Application Support
  default; `%APPDATA%\poplar\config.toml` on Windows). Both
  `[[account]]` blocks and the `[ui]` table live in the same file;
  `config.Load` (accounts) and `config.LoadUI` decode them
  independently. Path precedence: `--config` flag, `$POPLAR_CONFIG`,
  OS default, resolved by `config.Resolve`. The TOML key for the
  preset selector is `provider`.
- First-run flow: when the default-or-env path is missing,
  `config.Load` writes the self-documenting `config.Template()`
  to disk and returns `ErrFirstRun`; the root command prints a
  hint and exits 78 (EX_CONFIG). A legacy `accounts.toml` at the
  same dir returns `ErrOldAccountsToml` with a rename hint.
  `password-cmd` resolution is deferred to first `Connect` and
  cached on the Backend (mu-guarded `password` field) so secret-
  manager prompts surface near the action that needs them.
  Validation errors carry `account "<name>" (provider = "<p>"): ...`
  context; unknown providers get a Levenshtein "did you mean"
  suggestion within edit distance 2.
- `poplar config` subcommands: `init` (write template; refuses to
  overwrite without `--force`), `init --force`, `check` (validate
  + connect-test each account, sequentially — IMAP probe followed
  by `mailimap.ProbeSMTP` for IMAP-backed accounts), `path` (print
  resolved path), `discover-folders` (connect each account and
  merge default folder ordering into `[ui.folders]`).
- `[account.smtp]` is a TOML sub-table under each `[[account]]`.
  Provider presets carry `SMTPHost`/`SMTPPort`/`SMTPStartTLS`/
  `SMTPInsecureTLS` fields filling the canonical submission
  endpoints (gmail/fastmail/yahoo/zoho on 465 implicit-TLS;
  outlook/icloud on 587 STARTTLS; protonmail on the bridge's
  loopback 1025 STARTTLS with `insecure-tls`). `SMTPConfig.Auth`/
  `Password`/`PasswordCmd` default to mirroring the IMAP-side
  credentials when unset; the explicit block overrides only when
  SMTP differs from IMAP. JMAP accounts ignore `[account.smtp]`
  (submission rides the JMAP session). Validation requires
  `smtp.host` for `provider = "imap"` after preset resolution.
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

### Catkin

Auto-loaded via `.claude/rules/catkin-invariants.md` when
editing `internal/catkin/` or planning passes. ADRs 0144–0147,
0149, 0150, 0152.

### Compose

- `internal/compose/` is the outbound-mail surface. UI-free package
  owning the `Editor` seam (bubbletea sub-model contract used by
  ComposeTab and the future v1.1 neovim adapter), `CatkinEditor`
  (v1's only Editor impl, wraps `catkin.Model`), the `Draft` value
  type (headers + raw markdown body + filesystem attachment paths),
  `AssembleMIME(d, now)` (pure function emitting
  multipart/alternative text/plain + text/html via
  `filter.MarkdownToHTML`, wrapped in multipart/mixed when
  attachments are present), and `SeedReply` /
  `SeedReplyAll(parent, body, self)` / `SeedForward`. Reply seeders
  parse Message-Id and References from the parent's raw RFC 5322
  bytes (no `mail.MessageInfo` wire extension); quoting is
  depth-preserving (`> ` runs deepen). `gomail.Address` is the
  address type on Drafts; `content.ParseAddressList` is the shared
  RFC 5322 list parser. `internal/filter` gained `MarkdownBody` /
  `MarkdownToHTML` as the shared goldmark entry points (Linkify +
  Table extensions).

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
  T10, T14, T16, T27, T28, T33–T35. Semantic tells stay with the
  `/simplify` voice lens. `make install` writes to `~/.local/bin/`.
- Go module: `github.com/glw907/poplar`. `go.mod` 1.26.0; toolchain 1.26.1.
- Skills: invoke `go-conventions` before any Go file,
  `elm-conventions` before any `internal/ui/` file, update
  `docs/poplar/styling.md` before any color/style change. Pass-end
  ritual lives in `poplar-pass`.
- Live UI verification uses tmux (`.claude/docs/tmux-testing.md`).
  80×24 is the polish bar; UI passes capture 80×24 and 120×40.

## Decision index

Load the relevant ADR when you need rationale. Numbering is chronological.

| Invariant theme | ADRs |
|---|---|
| Monorepo, single binary | 0001, 0058 |
| Direct-on-libraries mail stack (no aerc fork) | 0002, 0006, 0008, 0010, 0012 (all superseded by 0075), 0075 |
| Lipgloss + compiled themes, styling discipline | 0004, 0043, 0046 |
| JMAP + IMAP only, minimal account config | 0009, 0075, 0098, 0101, 0104 |
| Mail backend interface synchronous | 0010 (superseded by 0075), 0075, 0099 |
| Config layout, folder classifier, UI config | 0013, 0052, 0053, 0102, 0103 |
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
| Responsive sidebar; 80×24 polish bar | 0096 (superseded by 0109), 0097, 0109 |
| Release model — pre-beta / beta soak / post-1.0 | 0105 |
| Gmail preset, X-GM-EXT-1 assertion, Destroy routing, XOAUTH2 via password-cmd | 0106, 0107, 0108 |
| Local cache architecture — per-account SQLite, typed Op sum + drainer, drain-first sync, outbox state machine, UIDVALIDITY re-key, IMAP scan-and-diff | 0110, 0111, 0112, 0113, 0114, 0115, 0116, 0117, 0118, 0120, 0121, 0122, 0123, 0124 |
| Backend error sentinels (mail.ErrAuth, mail.ErrNotFound) — typed at the protocol→cache boundary; drainer routes via errors.Is | 0119 |
| Cache cutover — UI reads/writes via cache.Account; mail.Backend shrunk (Mark*/Delete dropped); MessageList Apply* removed; folders.exists_total/unseen_total seed sidebar | 0121 |
| Cache II policy — lazy body population, single max-size backstop, no LRU; FetchBody returns []byte; poplar cache CLI | 0122, 0123, 0124 |
| Pass 8.5 overengineering audit — mail.Backend trim (Search/Copy removed), config.Source enum dropped, Provider doc fields removed, AccountConfig dead-field cleanup, cache.Cache aggregator removed, theme palette deletions, helper inlines, --config flag threading | 0125, 0126, 0127 |
| Pass 8.5b Elm conformance audit — URLOpener seam on App; ConfirmModalYesMsg + pendingEmptyConfirm replaces ConfirmRequest.OnYes callback; OpenLinkPickerMsg replaces Viewer.LinkPickerRequest accessor; help_popover row-by-row column join; typed triageOp enum; ui.FolderGroup→mail.Group; key.Bindings for raw keypress dispatch; AccountTab.now seam | 0128 |
| Pass 8.5c UI structural cleanup — `ModalShell` (named-field embed) for Box-rendering overlays; `SidebarColumn` composite hoists account-line + folders + spacer + shelf; `*<T>Cache` pointer + dirty flag escape hatch for view-stable overlays (MovePicker, HelpPopover) | 0129, 0130 |
| Pass 8.5d content/filter cleanup — `Block`/`Span` sealed-sum markers reduced to `isBlock()`/`isSpan()`; dead `blockKind`/`spanKind` enums deleted | 0131 |
| Cache III — outbox visibility (Q/! overlays, RetryOp/DiscardOp + revert mirror, status-bar outbox depth segment, offline UI hint) | 0132, 0133, 0134 |
| Attachments I (Pass 8.6) — backend support: `mail.Attachment` + `Attachments`/`FetchAttachment` on Backend; cache schema v5 `attachments` table with lazy metadata + lazy bytes under separate `MaxAttachmentSize` backstop; JMAP via Email/get bodyStructure + cli.Download; IMAP via UID FETCH BODYSTRUCTURE + BODY[<part>]; canonical `mail.ClassifyDisposition` (Disposition-first, ContentID fallback); MIME normalization at the protocol→mail boundary | 0135, 0136, 0137 |
| Attachments II (Pass 8.7) — viewer surface: chip row between header panel and body (hidden when empty); `@` opens App-owned `AttachPicker` overlay (`o`/Enter/digit open via `xdg-open` on tempfile, `s` saves to `[ui] download_dir`); `[ui] download_dir` resolves explicit > `$XDG_DOWNLOAD_DIR` > `~/Downloads`; collision suffixing capped at 999; `internal/humanize` package shared with cache CLI | 0138, 0139, 0140 |
| Human-voice policy — research-grounded style guide at `~/.claude/docs/go-comment-voice.md` (37-tell catalogue, voice palette); `go-conventions` skill carries the catalogue inline + experienced-Go-developer persona; `/simplify` voice lens flags tells by number; 8.8/8.9 split (string-only fixes vs. structural) against one frozen triage; grep-tier voice-check in `make check` covers T4, T10, T14, T16, T27, T28, T33, T34, T35 (T33–T35 added Pass 8.11 after a tree-wide cleanup) | 0141, 0142, 0148 |
| JMAP per-folder baseline pull on nil SyncToken — Email/query paged by inMailbox + sentinel-id Email/get for state in the same roundtrip; FetchHeaders chunked at 500 | 0143 |
| Catkin — core, live styling, commands, QoL, annotation pipeline + spellcheck, render-cursor splice; 9d.1 cleanup; 9d.3 targeted lint sweep; 9d.4 popover overlay padding | 0144, 0145, 0146, 0147, 0149, 0150, 0151, 0152, 0154, 0155 |
| Compose foundation — Editor interface + CatkinEditor adapter, Draft, AssembleMIME (multipart/alternative via shared filter.MarkdownToHTML, multipart/mixed when attachments), Seed{Reply,ReplyAll,Forward} parsing parent headers from raw bytes; content.ParseAddressList exported | 0156 |
| Backend Send + Append — `Send(env Envelope, mime []byte)` + `Append(folder, mime, flags)` on mail.Backend; JMAP via `Email/import` + `EmailSubmission/set` (atomic via `#k1` ref); IMAP via lazy `emersion/go-smtp` for Send + APPEND for Append; `[account.smtp]` block with provider preset SMTP defaults; `mailimap.ProbeSMTP` for `poplar config check` | 0157 |
| Cache outbox Send/Append dispatch — schema v6 adds `outbox.payload`; `SendArgs{Envelope}` + `AppendArgs{Flag}`; `QueueSend`/`QueueAppend` payload-bearing entry points; drainer `dispatch(args, row)` routes to `Backend.Send`/`Append`; `revertOptimisticTx` no-ops on Send/Append so `DiscardOp` works; IMAP enqueues two ops, JMAP one | 0158 |
| Path-scoped subsystem invariants — Cache, Catkin, Attachments split into `.claude/rules/<name>-invariants.md`; extraction-readiness criteria (settled, ≥ ~25 lines, natural path scope) | 0153 |
