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
  `internal/ui/` (App + bubbles-shaped subpackages `account`,
  `compose`, `helppopover`, `messagelist`, `movepicker`, `reader`,
  `sidebar`, plus `uicore`; ADRs 0161, 0163), `internal/mail/`
  (`Backend` interface + classifier; `mail.FolderEntry` display
  projection threads through sidebar/movepicker), `internal/mailjmap/`
  (Fastmail via `git.sr.ht/~rockorager/go-jmap`), `internal/mailimap/`
  (generic IMAP via `emersion/go-imap` v2; two physical connections
  per Backend — command + idle), `internal/mailauth/` (vendored
  XOAUTH2 SASL snippet; OAuth subsystem: loopback PKCE `Authorize`,
  cached `Token` + refresh-on-expiry, `TokenStore` keyring/age-file), `internal/config/` (`AccountConfig`,
  `UIConfig`, `LoadUI`, `Provider` registry), `internal/theme/`
  (compiled lipgloss themes), `internal/term/` (`HasNerdFont`,
  `MeasureSPUACells`), `internal/filter/` (markdown→HTML for
  compose, body rendering for the reader), `internal/content/`
  (address-list parsing, MIME plaintext extraction, body+footnote
  rendering, `List-Unsubscribe` parsing), `internal/tidy/`
  (Ctrl+T compose rewrite, ADR-0178), `internal/wizard/` +
  `internal/ui/wizard/` (first-run setup wizard split into UI-free
  domain + bubbletea/huh surface, ADR-0191).
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
- IMAP backend invariants: UIDPLUS required at Connect; MOVE /
  SPECIAL-USE / IDLE negotiated with fallbacks (COPY+STORE+EXPUNGE,
  alias classification, 30s STATUS-poll). IDLE refreshes every 9 min
  (RFC 2177 29-min cap), reconnects via exponential backoff
  mirroring `mailjmap.pushLoop`, emits `mail.Update` on the shared
  channel. `Destroy` issues `UID STORE +FLAGS.SILENT (\Deleted)` +
  `UID EXPUNGE <uids>` (ADR-0092 semantics, no collateral expunge).
  Gmail (`GmailQuirks = true`) asserts `X-GM-EXT-1` and routes
  `Destroy` through `SELECT [Gmail]/Trash` first; XOAUTH2 tokens
  from `mailauth.Token(ctx)`. SMTP is a third connection dialed
  lazily on first `Send` via `emersion/go-smtp`; cached client
  dropped on send error. `Append` runs `APPEND` on the cmd
  connection. `mailimap.ProbeSMTP` is the `poplar config check`
  surface. `dialRawTCP` (in `auth.go`) is the shared TCP helper.

### Send + Append

- `mail.Backend.Send(env Envelope, mime []byte) error` and
  `Append(folder, mime, flags) error` are the outbound primitives;
  `Envelope = { From, Rcpts }` is RFC 5321. `compose.AssembleMIME`
  pre-assembles `mime`; `internal/mail/` does not import compose.
- JMAP `Send` batches `Email/import` + `EmailSubmission/set` in one
  request via the `#k1` creation reference (atomic submission +
  Sent placement). `Identity/get` is lazy and cached on
  `Backend.identityIDs` keyed by lowercased email. JMAP `Append`
  drops the submission call.
- IMAP `Send` runs SMTP `MAIL`/`RCPT`/`DATA`; `Append` runs IMAP
  APPEND on the cmd connection. Sent placement is a separate
  caller-issued `Append`. SASL: plain (default), login, xoauth2.
- Cache outbox dispatches Send and Append. Schema v6 carries
  assembled MIME bytes in `outbox.payload`; v10 adds
  `scheduled_for` (undo-send / send-later) and `draft_id` FK
  (`ON DELETE SET NULL`; drainer deletes the linked draft in the
  OpDone tx on success). `SendArgs{Envelope}` / `AppendArgs{Flag}`
  ride in `outbox.args`. `QueueOutbound` returns op IDs in
  dispatch order — `[send]` on JMAP, `[send, append]` on IMAP. See
  `.claude/rules/cache-invariants.md` for queue / cancel /
  reschedule mechanics. ADRs 0183, 0184.

### Elm architecture & idiomatic bubbletea

- UI runtime is `charm.land/{bubbletea,lipgloss,bubbles}/v2`
  (ADRs 0189a, 0189b). `tea.KeyPressMsg.Code`/`Text`/`Mod`
  canonical; `AdaptiveColor` removed; palette + Styles take
  concrete `color.Color` (`lipgloss.Color(s)` is a function).
  Chrome is declarative: `App.View()` returns a `tea.View` with
  `v.AltScreen` and `v.WindowTitle` set every frame; cursor is
  hoisted via `Cursor() *tea.Cursor` accessors and
  `App.frameCursor()`; `SetVirtualCursor(false)` on every
  textinput/textarea. Paste handling routes by focus: address
  fields atomic-emit chips, Catkin splices and wraps URL tokens
  as markdown links — see ADR-0189b for the full paste contract.
- `internal/ui/` follows the Elm architecture — invoke the
  `elm-conventions` skill before touching any file there. State in
  tea.Model structs; mutations only in Update; I/O only in tea.Cmd;
  children expose accessors, parents read after delegation
  (`App.deriveChromeFromAcct`). `tea.Msg` is reserved for
  cross-tree signals, never child→parent state mirrors.
- Idiomatic bubbletea is the default. UI uses `bubbles` components
  as primary analogues; deviations are ADR'd. `View()` self-enforces
  size via `clipPane`; renderers honor `width` via wordwrap +
  hardwrap; SPUA-aware width math lives in `internal/ansix/` (a thin
  layer over `charmbracelet/x/ansi`): `ansix.Width` for icon-bearing
  strings, `lipgloss.Width` otherwise (never `len()`); truncation via
  `ansix.Truncate` / `ansix.TruncateEllipsis`. ADR-0181.
  `lipgloss.JoinHorizontal`/`JoinVertical` are
  forbidden when `spuaCellWidth != 1`; use row-by-row `strings.Join`
  with pre-padded children (kept under both modes — see ADR-0084).
  Keys declared as `key.Binding`, dispatched via `key.Matches`;
  `WindowSizeMsg` handlers both `SetSize` children and forward the
  msg. Full contract in `docs/poplar/bubbletea-conventions.md`.
- `internal/ui/` is the App parent plus eight bubbles-shaped
  subpackages (`account`, `compose`, `helppopover`, `messagelist`,
  `movepicker`, `reader`, `sidebar`, `wizard`) and the `uicore`
  sibling.
  Subpackages cannot import the parent. `uicore` holds shared
  chrome: `ErrorMsg`, `TriageOp` + `Triage*` constants,
  `ComputeLayout`, `NewSpinner`, `ModalShell`, `PlaceOverlay`,
  `DimANSI`, render primitives (`PadOrTruncate`, `TruncateToWidth`,
  `CenterOverlay`, `ApplyBg`, `FillRowToWidth`, `PickerListSize`,
  `SplitAndPad`), `NewListStyles` (compiled theme →
  `bubbles/v2/list.Styles` for the picker family; ADR-0194), and
  the `LayoutMode`/`IconSet`/`SearchMode` enums plus the
  `SimpleIcons`/`FancyIcons` tables. Each subpackage exposes one
  `Model` + `New(...)` (sub-models like `sidebar.Column`,
  `reader.LinkPicker` exported alongside). Per-subpackage `Styles`
  lives in that subpackage's `styles.go` with a
  `NewStyles(*theme.CompiledTheme)` constructor; those files are
  the only places outside `internal/ui/styles.go` and
  `internal/theme/palette.go` permitted to call
  `lipgloss.NewStyle()`. Cross-boundary msgs are exported in
  `<subpkg>/msgs.go` and qualified at the call site (e.g.
  `account.TriageStartedMsg`, `reader.BodyLoadedMsg`); subpackage-
  private msgs are unexported and never cross the boundary.
  Reader/compose cmds emitting `uicore.ErrorMsg` and orchestrating
  the App `URLOpener` seam live in `internal/ui/cmds.go`.
  `internal/ui/compose` shadows the `internal/compose` domain
  package: App-side imports use `uicompose`, and inside
  `internal/ui/compose/` the domain package is aliased
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
- First-run flow: missing config returns `ErrFirstRun`; root
  removes the freshly-written template and auto-launches the
  wizard. `--no-wizard` / `POPLAR_NO_WIZARD=1` opts out to exit-78.
  `--repair=<name>` seeds the wizard via `wizard.FromAccount` and
  splices `RepairResult` through `config.Render` + atomic rename.
  `--reauth=<name>` re-runs the OAuth consent flow for a named account.
  Legacy `accounts.toml` returns `ErrOldAccountsToml` (exit-78).
  `password-cmd` resolves on first `Connect` and caches on the Backend.
  `AccountConfig.Name` defaults to `Email`; `AccountConfig.Preset`
  round-trips the preset key (`config.Render` prefers it over `Backend`).
  `*config.ConfigError{…}` (sentinel `ErrConfigInvalid`) covers four
  validators: unknown provider, missing host, missing source, missing smtp.host.
- `config.Provider` carries `CredentialStrategy` and `HelpURL` per preset.
- `config.Render(accts, ui, cache) []byte` emits canonical TOML.
  Round-trips through `Load*` are semantic: comments not preserved,
  default-valued fields elided. `[account.smtp]` precedes
  `[[account.identity]]` (TOML array-of-tables rebind quirk).
  `[ui] theme` not yet rendered.
- `poplar config` subcommands: `init` (`--force` to overwrite;
  `--interactive` runs the wizard, `--section=name1,name2` filters
  the registry), `check` (validate + connect-test sequentially via
  the IMAP probe + `mailimap.ProbeSMTP`), `path`, `discover-folders`.
- `mail.ProbeResult{Steps, Err}` (`internal/mail/probe.go`) is the
  shared connect-test transcript; `mailimap.Probe` records 5 steps,
  `mailjmap.Probe` 3 (go-jmap bundles TLS + bearer + Session/get).
  First failure sets `Err` and stops. Test seams `probeDial` /
  `probeAuth`; `mailimap.layerTLS` is the shared TLS helper.
  `mail.IsSelfHosted(host)` reports RFC 1918 / IPv6 ULA / loopback
  / `.local`; the wizard's "skip TLS verify" prompts route through it.
- `wizard.Probe(ctx, cfg)` dispatches on `cfg.Backend` to
  `mailimap.Probe` (appending the SMTP probe) or `mailjmap.Probe`;
  test seams `imap/jmap/smtpProbeFn`. `wizard.SelectStrategy(preset)`
  returns a `config.CredentialStrategy` (`"imap"`/`"jmap"` → plain).
  `wizard.Apply(Model)` returns a ready-to-render `config.AccountConfig`. `internal/ui/wizard/` is the bubbletea +
  huh surface; `defaultSections` registry composes `account` /
  `theme` / `confirm`. ADR-0191.
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
  `AccountConfig.Identities` is length >= 1; legacy top-level
  `from` synthesizes one when blocks are absent. Each signature
  sets exactly one of `text`/`file`; `Signature.Text` carries
  the RFC 3676 `"-- \n"` sentinel; `Signature.Name` is unique
  within its identity. ADR-0177.
- `[account.oauth]` carries `client-id`, `client-secret`, optional
  `auth-url`/`token-url`/`scopes` for `gmail`/`outlook` xoauth2
  accounts (preset defaults fill missing fields). `oauth-store`
  (`"keyring"`/`"age-file"`) is written by the wizard on first
  `Authorize`. `mailauth.Token(ctx)` resolves credentials when
  `[account.oauth]` is present; parallel to `password`/`password-cmd`.
  ADR-0193.
- `[account.contacts]` is the optional CardDAV-ingest sub-table
  (URL, credentials, default-addressbook, refresh-interval,
  insecure-tls); credentials fall back to the parent
  `[[account]]` block. Absent block disables sync. ADR-0175.
- Themes are compiled Go values in `internal/theme/` (15 themes,
  One Dark default). No runtime TOML, no glamour. Components style
  through the `Styles` struct from `theme.CompiledTheme`.
  `lipgloss.NewStyle()` is permitted only in
  `internal/theme/palette.go`, `internal/ui/styles.go`, and
  per-subpackage `styles.go` files (each bubbles-shaped
  subpackage projects a narrow `Styles` from `internal/ui.Styles`
  via `NewStyles(*theme.CompiledTheme)`; `uicore/styles.go` holds
  its own chrome-primitive styles, since uicore does not project
  a `Styles` struct). Hex literals only in `themes.go`.
  The semantic map from palette slots to UI surfaces lives in
  `docs/poplar/styling.md`; update it before changing any color.

### Icon mode

- Icon mode is resolved once at startup. `cmd/poplar/root.go` calls
  `term.HasNerdFont`, `term.MeasureSPUACells`, and `term.Resolve` to
  produce `(IconMode, spuaCellWidth)`; `ansix.SetSPUACellWidth` is
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
  is a value-type To/Cc/Bcc autocomplete sub-model on
  `compose.Model`; `SuggestFn` threads from `App.suggestAddresses`
  → `cache.Account.SuggestAddresses` (recency-decayed,
  LIMIT 7). Renders on focus when the trailing fragment is ≥ 2
  chars; Tab/Enter rewrites as `Name <email>, `. ADR-0174.

### Address book

- `internal/contacts/` is the UI-free contacts surface: value
  types, CardDAV `Client` (wraps `emersion/go-webdav/carddav` for
  discovery, multiget, sync-collection, CTAG), `emersion/go-vcard`
  parser, `Sync` orchestrator + `Store` seam (`internal/cache`
  implements). `internal/ui/contacts/` adds per-package `Styles`,
  pure `RenderDetailCard`, and `Popover`/`Sidebar`/`List`/`Form`
  sub-models. Compose autocomplete + `i`-popover read from
  `cache.Account.SuggestAddresses` / `LookupContact`; `C`/`M`
  toggle Contacts mode. Overlay cascade: confirm > conflict >
  outbox > help > linkpicker > attachpicker > movepicker > form >
  popover.
- `contacts.Form` is the contact edit sub-model. App-owned
  confirm cascade: form-discard > contact-delete > compose-save
  > empty-folder. Save routes through `PatchVCard` (existing)
  or `BuildVCard` (new); multi-book destination is post-1.0.
  ADR-0176.

### Viewer

- Viewer harvests `List-Unsubscribe` / `-Post` headers at
  body-fetch time via `content.ParseListUnsubscribe`; the parsed
  `content.Unsubscribe` rides on `reader.BodyLoadedMsg.Unsub`.
  `U` confirms; routes by precedence https one-click POST >
  mailto into compose > plain http. Success notice surfaces in
  the chrome banner row (5s auto-clear). ADR-0185.

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

## Search

Search layer (FTS5 schema v11, parser, cache `Search`, sidebar
scope toggle, results-mode messagelist, throttle warn) lives in
`.claude/rules/search-invariants.md`. ADR-0188.

## Build & verification

- Makefile targets: `build`, `test`, `vet`, `fmt-check`, `lint`,
  `install`, `check`, `clean`. `make check` is the commit gate
  (fmt-check, vet, voice, test). The voice step is
  `scripts/voice-check.sh`, a grep-tier scan for tells T4, T10,
  T14, T16, T27, T28, T33, T35, T39, T40 (T34 voice-lens only,
  ADR-0173). Voice rules apply to all Claude-authored docs, not
  only Go source. `make install` writes to `~/.local/bin/`.
- Go module: `github.com/glw907/poplar`. `go.mod` 1.26.0; toolchain 1.26.1.
- Skills: invoke `go-conventions` before any Go file,
  `elm-conventions` before any `internal/ui/` file, update
  `docs/poplar/styling.md` before any color/style change. Pass-end
  ritual lives in `poplar-pass`.
- Live UI verification uses tmux (`.claude/docs/tmux-testing.md`); 80×24 is the polish bar, UI passes capture 80×24 and 120×40.

## Decisions

ADRs live in `docs/poplar/decisions/`. Load
`docs/poplar/decisions/INDEX.md` for the themed map from binding
facts to ADR numbers; load specific ADRs for full rationale.
