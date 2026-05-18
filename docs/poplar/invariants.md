# Poplar Invariants

Universal binding facts for the poplar codebase. Edited in place —
new facts replace or narrow old facts, they do not append. When a
pass changes a binding fact, update this file before committing.
Component- and UX-level invariants live in `.claude/rules/ui-invariants.md`
and load when editing `internal/ui/`, planning a UI pass, or reading
wireframes / keybindings. The authoritative key map is
`docs/poplar/keybindings.md`. Every fact here is codified in an ADR
under `docs/poplar/decisions/`; the index at the bottom maps each
section's claims back to the justifying ADR(s).

## Architecture

### Repo & libraries

- Poplar is a single-binary bubbletea terminal email client built
  from one Go module: `cmd/poplar`.
- Repository organization: `cmd/poplar/` (CLI wiring only) and
  `internal/{ansix,cache,catkin,config,contacts,content,filter,
  icalendar,mail,mailauth,mailcompose,mailimap,mailjmap,search,
  strdist,term,theme,tidytext,ui,wizard}` (plus `internal/ui/wizard`).
  `internal/ui/` is App + bubbles-shaped subpackages (`account`,
  `compose`, `helppopover`, `messagelist`, `movepicker`, `reader`,
  `sidebar`, `wizard`) and `uicore` (ADRs 0161, 0163).
  `internal/mail/` carries the `Backend` interface + classifier;
  `mailjmap` wraps `go-jmap`, `mailimap` wraps `go-imap` v2 (two
  physical connections per Backend — command + idle), `mailauth`
  holds the XOAUTH2 snippet and OAuth subsystem (loopback PKCE
  Authorize default + opt-in RFC 8628 device-code, cached Token
  + refresh, keyring/age-file TokenStore). System map
  in `docs/poplar/system-map.md`. ADRs 0193, 0220, 0227.
- Mail backends call upstream libraries directly: emersion
  (`go-imap` v2, `go-message`, `go-smtp`, `go-sasl`, `go-webdav`,
  `go-vcard`) plus `rockorager/go-jmap`. Vendored MIT snippets
  (XOAUTH2, Gmail X-GM-EXT) carry a top-of-file provenance
  comment. TCP keepalive uses stdlib `net.KeepAliveConfig`.
- Backends in v1: JMAP (`provider = "jmap"` / `"fastmail"`) and
  generic IMAP (`provider = "imap"` or one of the presets `yahoo`,
  `icloud`, `zoho`, `outlook`, `mailbox-org`, `posteo`, `runbox`,
  `gmx`, `protonmail`, `gmail`). `config.ResolvePreset(*AccountConfig)`
  fills empty Backend, Host, Port, StartTLS, InsecureTLS,
  GmailQuirks, Source, and SMTP fields from `Providers[c.Preset]`;
  non-empty slots win. Called by both the TOML decoder and
  `wizard.Apply` so the pre-save probe matches the runtime.
  Self-hosted IMAP uses explicit `host`/`port` + `insecure-tls`;
  the dial path emits a "set insecure-tls" hint on TLS failure for
  RFC 1918 / `.local` / `127.x`. No maildir.
- `mail.Backend` is synchronous blocking; both packages call their
  libraries synchronously — no pump goroutine, no async bridge.
  `mail.ConnConnecting` is the pre-authenticated initial state;
  distinct from `ConnReconnecting` ("was connected, lost it").
  `App.backendState` overrides `mail.ConnState` in the status-bar
  render pre-wire. ADR-0242.
- IMAP backend invariants: UIDPLUS required at Connect; MOVE /
  SPECIAL-USE / IDLE negotiated with fallbacks (COPY+STORE+EXPUNGE,
  alias classification, 30s STATUS-poll). IDLE refreshes every 9 min
  (RFC 2177 29-min cap), emits `mail.Update` on the shared channel.
  Both connections are drop-and-redial on `mail.ErrConnection` with
  exponential backoff; cmd-path actions go through `b.cmdClient()`,
  idle through `b.dialIdle`. `Destroy` issues `UID STORE
  +FLAGS.SILENT (\Deleted)` + `UID EXPUNGE <uids>` (ADR-0092
  semantics, no collateral expunge). Gmail (`GmailQuirks = true`)
  asserts `X-GM-EXT-1` and routes `Destroy` through `SELECT
  [Gmail]/Trash` first; XOAUTH2 tokens from `mailauth.Token(ctx)`.
  IMAP `dial` and SMTP `smtpAuth` ForceRefresh-and-retry once on
  `mail.ErrAuth` when `b.oauth != nil`; `classifyErr` covers
  `*imap.Error` codes + `*gosmtp.SMTPError` 530/535/538. SMTP
  dials lazily on first `Send` via `emersion/go-smtp`; cached
  client dropped on send error. `Append` runs on the cmd connection.
  `mailimap.ProbeSMTP` is the `poplar config check` surface.
  ADRs 0208, 0222.

### Send + Append

- `mail.Backend.Send(env Envelope, mime []byte) error` and
  `Append(folder, mime, flags) error` are the outbound primitives;
  `Envelope = { From, Rcpts }` is RFC 5321. `mailcompose.AssembleMIME`
  pre-assembles `mime`; `internal/mail/` does not import mailcompose.
- JMAP `Send` batches `Email/import` + `EmailSubmission/set` via
  the `#k1` creation reference (atomic submission + Sent
  placement); `Identity/get` is lazy and cached on
  `Backend.identityIDs` (lowercased-email keyed). JMAP `Append`
  drops the submission call. IMAP `Send` runs SMTP
  `MAIL`/`RCPT`/`DATA`; `Append` runs IMAP APPEND on the cmd
  connection; sent placement is a separate caller-issued
  `Append`; SASL: plain (default), login, xoauth2.
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
  `v.AltScreen`, `v.WindowTitle`, `v.ProgressBar`, `v.ReportFocus
  = true`, `v.KeyboardEnhancements.ReportEventTypes = true`, and
  `v.MouseMode = tea.MouseModeCellMotion` set every frame (ADRs
  0217, 0218). ProgressBar follows the fixed ladder
  attachment > outbox > sync sourced from
  `cache.Account.{AttachmentDownloadProgress,OutboxDrainProgress,
  SyncProgress}`. `tea.FocusMsg`/`BlurMsg` toggle `App.focused`;
  `tea.KeyboardEnhancementsMsg` lands in `App.kbdCaps` (Flags
  bitmask via `SupportsKeyDisambiguation`/`SupportsEventTypes`).
  Cursor is hoisted via `Cursor() *tea.Cursor` accessors and
  `App.frameCursor()`; `SetVirtualCursor(false)` on every
  textinput/textarea. Paste handling routes by focus — see
  ADR-0189b.
- `internal/ui/` follows the Elm architecture — invoke the
  `elm-conventions` skill before touching any file there. State in
  tea.Model structs; mutations only in Update; I/O only in tea.Cmd;
  children expose accessors, parents read after delegation
  (`App.deriveChromeFromAcct`). `tea.Msg` is reserved for
  cross-tree signals, never child→parent state mirrors.
- Idiomatic bubbletea is the default. `bubbles` components are the
  primary analogues; deviations are ADR'd. SPUA-aware width math
  lives in `internal/ansix/`: `ansix.Measurer` (value type,
  resolved cell width) is constructed once in
  `cmd/poplar/root.go` via `ansix.NewMeasurer(cellWidth)` and
  threaded through `ui.NewApp` into every subpackage Model. Use
  `m.Width`/`m.Truncate`/`m.TruncateEllipsis` for icon-bearing
  strings, `lipgloss.Width` otherwise (never `len()`);
  `ansix.SpuaCount` is a free function; `uicore.FillRowToWidth`
  takes a Measurer. `lipgloss.JoinHorizontal`/`JoinVertical` are
  forbidden when cellWidth != 1 — use row-by-row `strings.Join`
  with pre-padded children. Keys are `key.Binding`, dispatched
  via `key.Matches`; `WindowSizeMsg` handlers both `SetSize`
  children and forward the msg. Full contract in
  `docs/poplar/bubbletea-conventions.md`. ADRs 0084, 0181, 0209.
- `internal/ui/` is the App parent plus eight bubbles-shaped
  subpackages (`account`, `compose`, `helppopover`, `messagelist`,
  `movepicker`, `reader`, `sidebar`, `wizard`) and the `uicore`
  sibling. Subpackages cannot import the parent. `uicore` holds
  shared chrome — `ErrorMsg`, `TriageOp` constants,
  `ComputeLayout`, `NewSpinner`, `ModalShell`, `PlaceOverlay`,
  render primitives, `NewListStyles` (ADR-0194), and the
  `LayoutMode`/`IconSet`/`SearchMode` enums plus icon tables.
  Each subpackage exposes one `Model` + `New(...)`; per-subpackage
  `Styles` lives in `styles.go` with `NewStyles(*CompiledTheme)`,
  the only places outside `internal/ui/styles.go` and
  `internal/theme/palette.go` permitted to call
  `lipgloss.NewStyle()`. Cross-boundary msgs export in
  `<subpkg>/msgs.go` and qualify at the call site; private msgs
  stay unexported. Reader/compose Cmds emitting `uicore.ErrorMsg`
  and the App `URLOpener` seam live in `internal/ui/cmds.go`.
  `internal/ui/compose` is the UI surface (`uicompose` alias);
  `internal/mailcompose` is the domain. `App` splits across
  `app.go` (model + dispatcher), `app_view.go`, `app_keys.go`,
  and per-domain `app_{chrome,outbox,compose,modals,contacts}.go`;
  each domain owns `updateXMsg(msg) (App, tea.Cmd, claimed bool)`
  chained in `App.Update`. ADRs 0161–0163, 0203, 0214.
- `App` threads `*cache.Account` + `*theme.CompiledTheme` into the
  tree. `account.Model` holds the cache handle (backend reachable via
  `account.Model.Backend()` for `pumpUpdatesCmd`); reads come from
  `cache.Account.QueryFolder`/`ListFolders`, writes from
  `cache.Account.QueueOp`. `messagelist.Model` is presentation-only;
  `RefreshSource` re-reads cache state after every write, preserving
  cursor on UID. `reader.Model` holds the theme reference for
  markdown rendering. `mail.Backend.Flag(uids, flag, set)` is the
  canonical mutator; "delete" is `MoveArgs{Dest: trash}` queued.
  IMAP `Move` falls back to `cmd.Copy` when MOVE is absent.
  `App` owns `backendState uicore.BackendState` + `backendErr`; `Init` dispatches `connectBackendCmd`; `BackendReadyMsg` fans out pump + contacts sync; `BackendErrMsg` stores the error. ADR-0242.
- Mouse is keyboard shorthand. `App.updateMouse` absorbs on
  overlay; else translates Y to account-local and branches on
  `viewerOpen` × pane: viewer-ready right pane →
  `account.Model.UpdateViewer`; viewer-open sidebar click →
  `CloseViewer()` then `account.Model.Update`; else →
  `account.Model.Update`. Divider inert. Account partitions by
  X: sidebar click on a real folder = `J`/`K`, on a
  synthesized intermediate = toggle expand; right-pane click =
  `Enter`; wheel = the cursor key per notch.
  `messagelist.Model.ItemIndexAt` and
  `sidebar.Model.RowAtLineOffset` are the hit-test seams.
  Reader: wheel → viewport, chip → `OpenAttachmentMsg`,
  `[N]: <url>` row → `LaunchURLMsg`; inline `[^N]` not a
  target. `content.RenderBodyWithFootnotes` returns
  `[]FootnoteRow`. ADRs 0218, 0219.

### Config & theming

- Config lives in `~/.config/poplar/config.toml` (XDG on Linux and
  macOS, overriding Apple's Application Support default;
  `%APPDATA%\poplar\config.toml` on Windows). `[[account]]` blocks
  and the `[ui]` table share the file; `config.Load` and
  `config.LoadUI` decode them independently. Path precedence:
  `--config`, `$POPLAR_CONFIG`, OS default (`config.Resolve`). The
  TOML preset key is `provider`.
- First-run flow: missing config returns `ErrFirstRun`; root
  removes the freshly-written template and auto-launches the
  wizard. `--no-wizard` / `POPLAR_NO_WIZARD=1` opts out to exit-78.
  `--repair=<name>` seeds the wizard via `wizard.FromAccount` and
  splices `RepairResult` through `config.Render` + atomic rename.
  `--reauth=<name>` re-runs the OAuth consent flow. Legacy
  `accounts.toml` returns `ErrOldAccountsToml` (exit-78).
  `password-cmd` resolves on first `Connect` and caches on the
  Backend. `AccountConfig.Name` defaults to `Email`; `Preset`
  round-trips (`config.Render` prefers it over `Backend`).
  Decoding is strict (`config.strictDecode` walks `md.Undecoded()`
  and rejects unknown keys with a Levenshtein sibling suggestion;
  map-typed sections keep accept-anything semantics).
  `*config.ConfigError{…}` (`ErrConfigInvalid`) covers unknown TOML
  keys, unknown provider, missing host / source / smtp.host /
  bare-imap port, unknown `auth` / `smtp.auth` / `oauth-store` enum
  values, empty `contacts.url`, and missing contacts credentials
  after parent-account fallback. ADR-0211.
- `config.Render(accts, ui, cache) []byte` emits canonical TOML;
  round-trips are semantic (comments lost, default-valued fields
  elided). `[account.smtp]` precedes `[[account.identity]]`. Multi-
  line signature `text` emits as a TOML `"""…"""` basic string.
- `poplar config` subcommands: `init` (`--force`; `--interactive`
  runs the wizard, `--section=…` filters the registry), `check`
  (validate + connect-test via IMAP probe + `mailimap.ProbeSMTP`),
  `path`, `discover-folders`.
- `mail.ProbeResult{Steps, Err}` is the shared connect-test
  transcript; `mailimap.Probe` runs 5 steps (or 6 with an
  `*mailauth.Client`, adding `oauth-token`), `mailjmap.Probe` 3.
  First failure sets `Err` and stops. `mail.IsSelfHosted(host)`
  covers RFC 1918 / IPv6 ULA / loopback / `.local` for the
  wizard's TLS-skip prompts.
- `wizard.Probe(ctx, cfg, oauthCli)` dispatches on `cfg.Backend`
  to `mailimap.Probe` (+ `ProbeSMTP`) or `mailjmap.Probe`; both
  IMAP paths receive `oauthCli`. `wizard.Apply(Model)` calls
  `config.ResolvePreset` so the probe sees the round-trip config.
  Account-section stages: provider → email → credentials → probe
  → identity → signature → label; signature hosts catkin with a
  dim `-- ` chrome row, sentinel-free body. ADRs 0191, 0207, 0220.
- `[account.smtp]` is a TOML sub-table under each `[[account]]`;
  presets fill submission endpoints, `Auth`/`Password`/`PasswordCmd`
  default to the IMAP-side credentials, JMAP ignores it, and
  `smtp.host` is required for `provider = "imap"` after preset
  resolution.
- `mail.MockBackend` is gated behind the `dev` build tag at
  `cmd/poplar` (`backend_{dev,nodev}.go`); release binaries reject
  `provider = "mock"`. `make test`/`make check` pass `-tags=dev`.
  ADR-0207.
- `[[account.identity]]` carries ordered
  `[[account.identity.signature]]` sub-blocks.
  `AccountConfig.Identities` is length >= 1 (legacy top-level
  `from` synthesizes one). Each signature sets exactly one of
  `text`/`file`; `Signature.Text` carries the RFC 3676 `"-- \n"`
  sentinel; `Signature.Name` is unique within its identity.
  ADR-0177.
- `[account.oauth]` carries `client-id`, `client-secret`, optional
  `auth-url`/`token-url`/`device-auth-url`/`scopes` (preset
  defaults fill missing fields). `oauth-store`
  (`"keyring"`/`"age-file"`) and `oauth-mode` (`"loopback"`
  default, `"device-code"`) are written by the wizard.
  `mailimap.Probe` / `ProbeSMTP` take an optional
  `*mailauth.Client` (threaded by `wizard.Probe`) so the pre-save
  reachability check authenticates like the live backend. The two
  wizard construction sites share `ui/wizard.buildOAuthClient`;
  template documents native consent + `--reauth`. ADRs 0193,
  0220, 0227.
- `[account.contacts]` is the optional CardDAV-ingest sub-table
  (URL, credentials, default-addressbook, refresh-interval,
  insecure-tls); credentials fall back to the parent. Absent
  block disables sync. ADR-0175.
- Themes are compiled Go values in `internal/theme/` (15 themes,
  One Dark default). No runtime TOML, no glamour. Components style
  through `theme.CompiledTheme.Styles`. `lipgloss.NewStyle()` is
  permitted only in `internal/theme/palette.go`,
  `internal/ui/styles.go`, and per-subpackage `styles.go` files
  (each subpackage projects a narrow `Styles` from
  `internal/ui.Styles` via `NewStyles(*theme.CompiledTheme)`;
  `uicore/styles.go` holds its own chrome-primitive styles).
  Hex literals only in `themes.go`. The palette-to-surface map
  lives in `docs/poplar/styling.md`; update it before changing
  any color.

### Icon mode

- Icon mode resolved once at startup: `cmd/poplar/root.go` runs
  `term.HasNerdFont` + `term.MeasureSPUACells` + `term.Resolve` for
  `(IconMode, cellWidth)`; resulting `uicore.IconSet` +
  `ansix.NewMeasurer(cellWidth)` thread into `ui.NewApp`. No runtime
  toggle. Icon literals live only in `uicore/layout.go`: `SimpleIcons`
  are EAW Na/N, `FancyIcons` in `[U+F0000, U+FFFFD]`.

### Catkin — see `.claude/rules/catkin-invariants.md` (auto-loaded on `internal/catkin/`). ADRs 0144–0147, 0149, 0150, 0152.

### Compose

- `internal/mailcompose/` is the UI-free outbound surface: the
  `Draft` value type, pure `AssembleMIME(d, identities, now)`
  (multipart/alternative text+html via `filter.MarkdownToHTML`;
  multipart/mixed when attachments present), and
  `SeedReply`/`SeedReplyAll`/`SeedForward` parsing parent
  Message-Id + References from raw RFC 5322 bytes with
  depth-preserving `>` quoting. `gomail.Address` is the Draft
  address type; `content.ParseAddressList` is the list parser.
  `internal/filter` exposes `MarkdownBody`/`MarkdownToHTML` as
  the shared goldmark entries (Linkify + Table). `compose.Model`
  (`internal/ui/compose/`) holds a named `editor catkin.Model`
  field. ADRs 0212, 0213.
- `internal/ui/compose/` owns the live compose surface. `Dropdown`
  is a value-type To/Cc/Bcc autocomplete sub-model on
  `compose.Model`; `SuggestFn` threads from `App.suggestAddresses`
  → `cache.Account.SuggestAddresses` (recency-decayed,
  LIMIT 7). Renders on focus when the trailing fragment is ≥ 2
  chars; Tab/Enter rewrites as `Name <email>, `. ADR-0174.

### Address book

- `internal/contacts/` is the UI-free contacts surface (CardDAV
  `Client` wrapping `emersion/go-webdav/carddav`,
  `emersion/go-vcard` parser, `Sync` + `Store` seam — `cache`
  implements). `internal/ui/contacts/` adds `Styles`, pure
  `RenderDetailCard`, and `Popover`/`Sidebar`/`List`/`Form`
  sub-models; compose autocomplete + `i`-popover read from
  `cache.Account.SuggestAddresses`/`LookupContact`; `C`/`M`
  toggle Contacts mode. Overlay cascade: confirm > conflict >
  outbox > help > linkpicker > attachpicker > movepicker > form
  > popover. `contacts.Form` confirm cascade: form-discard >
  contact-delete > compose-save > empty-folder; save via
  `PatchVCard`/`BuildVCard`; multi-book destination is post-1.0.
  ADR-0176.

### Viewer

Viewer harvests `List-Unsubscribe` / `-Post` at body-fetch
(`content.ParseListUnsubscribe` → `reader.BodyLoadedMsg.Unsub`).
`U` confirms; routes https one-click POST > mailto > http; 5s
banner. ADR-0185.

## Mail model

- Folder classification is a pure function:
  `mail.Classify([]Folder) []ClassifiedFolder`. Priority:
  `Folder.Role` → alias table → `Custom`. Provider folder names are
  normalized to canonical display names (Inbox, Sent, Trash, …)
  regardless of JMAP/IMAP naming.
- `mail.MessageInfo` carries `ThreadID`, `InReplyTo`, and
  `SentAt time.Time`. Depth is derived during the prefix walk, not
  wired. A non-threaded message is a thread of size 1 with
  `ThreadID == UID` and `InReplyTo == ""`. `SentAt` is the sole
  date carrier; renderers fall back to "" when zero. Cache schema
  v12 dropped the legacy `messages.date_str` column. ADR-0203.
- `mail.Backend.Destroy(uids)` is the irreversible permanent-delete
  primitive (no inverse). Empty input is a no-op. JMAP issues
  `Email/set { destroy }` (idempotent — `notFound` is success).
  IMAP issues `UID STORE +FLAGS.SILENT (\Deleted)` then `UID
  EXPUNGE <uids>`, scoped by UIDPLUS so unrelated pre-marked
  messages are unaffected.
- Typed sentinels `mail.{ErrAuth, ErrNotFound, ErrConnection}`
  attach via `mail.WrapSentinel` inside each backend's
  `classifyErr`: JMAP `*jmap.RequestError` 401/403 → `ErrAuth`,
  404 → `ErrNotFound`; IMAP `AUTHENTICATIONFAILED` /
  `AUTHORIZATIONFAILED` / `PRIVACYREQUIRED` → `ErrAuth`,
  `NONEXISTENT` → `ErrNotFound`, transport errors → `ErrConnection`.
  Cache drainer's conflict matrix routes via `errors.Is`.

Attachment wire shape + picker/viewer surface: see
`.claude/rules/attachments-invariants.md` (auto-loaded on
`internal/ui/attachpicker*.go` / `viewer*.go` and plan/spec docs).

## Cache

Per-account SQLite, schema versions, drainer, outbox state machine,
body + attachment storage: see `.claude/rules/cache-invariants.md`
(auto-loaded on `internal/cache/`, `cmd/poplar/cache*.go`, plans).

## Search

Search layer (FTS5 schema v11, parser, cache `Search`, sidebar scope toggle, results-mode messagelist, throttle warn) lives in `.claude/rules/search-invariants.md`. ADR-0188.

## Logging

`log/slog` for `internal/`, stderr for `cmd/poplar/`. `installLogger`
from `main()`, re-run in `runRoot` after `LoadUI`; `POPLAR_LOG=debug`
beats `[ui] log-level`. TTY → `lumberjack.Logger` on
`$XDG_STATE_HOME/poplar/poplar.log` (10 MB × 2). `logctx.Handler`
injects ctx `op_id`; mutating `Backend` methods take `ctx` first,
drainer wraps with `WithOpID(ctx, row.ID)`. Backend ctors take
`*slog.Logger` + `wireTrace bool`; true wires IMAP/SMTP
`DebugWriter` + JMAP `loggingTransport` to `logctx.WireWriter`.
Debug checkpoints at connect/TLS/auth/idle/redial, eventsource, and cache migrations; `slog.Info("poplar start", ...)` marks sessions. ADRs 0197, 0209, 0240, 0241.

## Build & verification

- `make check` (commit gate) = fmt-check + vet + voice +
  modern-go-check + skipcheck + test; `-tags=dev` keeps MockBackend
  in scope. Grep-tier voice/modern-go checks per ADRs 0173, 0196;
  `scripts/skipcheck` rejects unguarded `t.Skip*` (ADR-0234).
  `make check-deep` runs gremlins per-package (ADRs 0232, 0234–0237).
  `make install` → `~/.local/bin/`. Module `github.com/glw907/poplar`;
  `go.mod` 1.26.0, toolchain 1.26.1.
- Skills: `go-conventions` (any Go file), `elm-conventions`
  (`internal/ui/`), `styling.md` (any color), `poplar-pass`
  (pass-end). UI via tmux (`.claude/docs/tmux-testing.md`).

## Decisions — `docs/poplar/decisions/` (themed map: `INDEX.md`).
