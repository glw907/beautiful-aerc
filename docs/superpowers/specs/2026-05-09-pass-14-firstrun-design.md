# Pass 14 — First-run wizard

## Goal

A stepped, in-TUI configuration wizard that builds
`~/.config/poplar/config.toml` for new users (#27) and surfaces helpful
errors when the file exists but is malformed (#27 extension, #29).
OAuth refresh-token handling and the Gmail/Outlook flow split out into
**Pass 14.1**.

## Scope

In:
- New cobra subcommand `poplar config init --interactive` and a
  first-run auto-launch path in `runRoot`.
- `internal/wizard/` (UI-free domain) and `internal/ui/wizard/`
  (bubbletea, huh-driven). Section-pluggable architecture.
- Account section (provider preset → email → credentials → probe →
  identity → label) covering app-password, API-token, and plain-IMAP /
  plain-JMAP credential strategies.
- Theme section (live-previewable theme picker).
- Stub sections for contacts / signatures / tidy that slot in later
  without restructuring.
- Malformed-config UX: typed `config.ConfigError`, friendly formatter
  in `runRoot`, `--repair=<name>` flag to wizard the broken block.
- Config template rewrite to drop wizard-doesn't-exist-yet comments and
  fix the empty-`name` validator gotcha (#29 fix).

Out (Pass 14.1):
- OAuth interactive flow, refresh-token storage, mailauth
  refresh-on-401 hook, Gmail/Outlook credential strategy.

Out (post-1.0):
- Editing existing accounts via wizard.
- Multi-account-in-one-run.
- `[ui]` retention / undo-window settings (defaults are fine, sections
  added in a later pass).

## Settled decisions inherited from STATUS

Wizard runs in bubbletea (not a separate prompt loop); reuses
`internal/config/`'s provider preset machinery; OAuth lives at the
auth layer per ADR-0098.

## Settled inline (this brainstorm)

- **Flow shape**: stepped (a wizard is sequenced by definition).
- **Probe failures**: retry inline, with `[s]ave and quit` escape that
  writes the partial block as commented-out TOML.
- **OAuth**: internalized (Pass 14.1) — wizard runs the device-code /
  loopback flow, persists refresh tokens via OS keyring, refreshes
  access tokens on each Connect from `internal/mailauth`. Pass 14
  ships the wizard scaffold + `CredentialStrategy` seam where OAuth
  slots in; placeholder `password-cmd` for Gmail/Outlook in 14.
- **Pass split**: Pass 14 = wizard + non-OAuth + #29 plumbing.
  Pass 14.1 = OAuth subsystem.
- **UI library**: `charm.land/huh/v2` for stepped form pages. Probe
  and (eventually) OAuth use a custom `tea.Model` orchestrated
  around the form.
- **Wizard organization**: Sections, not steps. `wizard.Run(mode)`
  composes ordered `[]Section`. Adding a new configurable surface
  means adding a `Section` to the registry, keyed by name and
  individually addressable from `--section=`.

## Architecture

### New packages

**`internal/wizard/`** — UI-free domain layer.
- `Model` value type holding `Provider`, `Email`, `Credential`,
  `IdentityName`, `AccountLabel`, `ProbeResult`, current step.
- `Strategy` interface, one method `RenderStep(model)` returning the
  typed credential field shape (no UI). Implementations:
  `appPasswordStrategy`, `apiTokenStrategy`, `plainIMAPStrategy`,
  `plainJMAPStrategy`. `oauthStrategy` added in Pass 14.1.
- `Section` interface: `{ Name() string; Required() bool;
  Hide() bool; Run(opts SectionOpts) tea.Cmd }`.
- `Apply(model) (config.AccountConfig, error)` — pure conversion.
- `Probe(ctx, AccountConfig) ProbeResult` — backend-agnostic
  dispatcher routing on `cfg.Provider`. IMAP family →
  `mailimap.Probe` + `mailimap.ProbeSMTP` (already exist). JMAP family
  → new `mailjmap.Probe` (Pass 14 add, mirrors the IMAP one).
  Returns `ProbeResult { Steps []ProbeStep, Err error }`. Each
  `ProbeStep { Label, Status, Detail }`.

**`internal/ui/wizard/`** — bubbles-shaped subpackage paralleling
`account`, `compose`, `reader`, etc.
- `Model` owns the stepped flow.
- Sub-models per section use `huh.NewSelect`, `huh.NewInput`,
  `huh.NewText`, `huh.NewConfirm`, `huh.NewNote`. Custom `tea.Model`
  for the probe screen (huh's `Note` is static; we need a live
  spinner + transcript).
- `theme.go` adapts `theme.CompiledTheme` → `huh.Theme` so the wizard
  inherits poplar's palette.
- `logo.go`: typographic wordmark via lipgloss (interim). The
  `art/poplar-logo.ans` artifact is committed but unused at startup;
  swapping it in is a one-line change to `logo.go`.
- `Styles` per the per-subpackage convention, constructed by
  `NewStyles(*theme.CompiledTheme)`.

### Extensions to existing packages

`internal/config/providers.go`: add
`Provider.CredentialStrategy CredentialStrategy` (enum
`StrategyAppPassword | StrategyAPIToken | StrategyOAuth |
StrategyPlainIMAP | StrategyPlainJMAP`). Every preset gets a
`HelpURL` (most already do).

`internal/config/`: new typed error
`ConfigError { Path string; Line int; Field string; Message string;
Suggest string }` wrapping existing validators.

`internal/mailjmap/`: new `Probe(ctx, AccountConfig) ProbeResult`
function. Wraps the existing Connect-time discovery + auth +
Account/get into step-by-step status output.

`internal/config/template.go`: `name` becomes optional with default
= `email`. OAuth comment section reworded ("Gmail and Outlook are
configured by the wizard"). Drop the
"Until poplar's first-run wizard ships..." block.

`cmd/poplar/`: new `config init --interactive` subcommand. `runRoot`
auto-launches the wizard on `ErrFirstRun` (replacing the current
exit-78 path). Opt-outs via `--no-wizard` and
`POPLAR_NO_WIZARD=1` so existing automation that relied on
"missing config exits 78" still works.

## Wizard flow

1. **Welcome** (`huh.Note` + `huh.Confirm`) — wordmark, "continue or
   skip and edit by hand."
2. **Account section** (always required for first-run):
   1. **Provider** (`huh.Select`) — preset list with one-line
      descriptions. `Other / self-hosted IMAP` and
      `Other / self-hosted JMAP` at the bottom.
   2. **Email** (`huh.Input` + email validator).
   3. **Credentials** (one huh group per `CredentialStrategy`,
      hidden by `WithHideFunc`):
      - `AppPassword`: `Note` showing provider's app-password URL +
        confirm-to-open-in-browser, then masked `Input` for the password.
      - `APIToken` (Fastmail): `Note` showing the Fastmail token URL,
        masked `Input` for the token.
      - `PlainIMAP`: `Input` host, `Input` port (default 993),
        `Input` username (default = email), masked `Input` password,
        `Confirm` for `insecure-tls` if host parses to RFC1918 /
        `.local` / `127.x`.
      - `PlainJMAP`: `Input` session URL, `Input` username (default =
        email), masked `Input` token, `Confirm` for `insecure-tls`
        on the same heuristic.
      - `OAuth` (Pass 14.1): placeholder `Input` for `password-cmd`
        in 14.
   4. **Probe** (custom `tea.Model`) — spinner + transcript:
      ```
      Connecting to imap.fastmail.com:993…   ✓
      TLS handshake…                          ✓
      AUTHENTICATE PLAIN…                     ✓
      CAPABILITY (UIDPLUS)…                   ✓
      STATUS INBOX…                           ✓ (1,247 messages)
      ```
      On success, auto-advance after 800ms. On failure: `[r]` retry,
      `[e]` edit credentials only (jumps back to step 4.iii within
      the section), `[s]` save and quit (writes commented-out block).
   5. **Identity name** (`huh.Input`, default = title-cased local
      part of email).
   6. **Account label** (`huh.Input`, default = preset name or
      email domain).
3. **Theme section** (non-required, gated on `Confirm`) —
   `huh.Select` over compiled themes; live-preview the chosen theme
   on the next render so the user sees it before confirming. The
   wizard starts in `one-dark` (poplar's invariant default); picking
   a different theme propagates the new `theme.CompiledTheme` through
   the huh.Theme adapter so subsequent steps (confirm, done) render
   in the chosen palette. If the user takes the default or declines
   the section gate, `config.toml` omits `[ui]` `theme` and the
   default kicks in via `config.LoadUI`'s zero-value — minimal file
   for users who took defaults.
4. **Stub sections** (hidden via `Hide()=true` until their backing
   feature ships): contacts, signatures, tidy. No-op for Pass 14.
5. **Confirm + write** (`huh.Note` showing the assembled
   `[[account]]` + `[ui.theme]` blocks + `huh.Confirm`) — write to
   `~/.config/poplar/config.toml`.
6. **Done** (`huh.Note`) — "Setup complete. poplar will start. Run
   `poplar config init --interactive` again to add another account
   or change settings."

### CLI surface

- `poplar config init --interactive` runs all sections (full
  first-run experience). Each section's confirm-gate defaults Yes
  in this mode.
- `poplar config init --interactive --section=theme` (or
  comma-separated names) runs only named sections, skipping the
  confirm gate. The "let me change my theme without leaving the
  TUI" path.
- `poplar --repair=<account_name>` jumps straight into the account
  section pre-populated from the broken block in `config.toml`.

## Wireframes

Welcome (80×24):

```
┌────────────────────────────────────────────────────────────────┐
│                                                                │
│                       ─────────────                            │
│                            poplar                              │
│                       ─────────────                            │
│                    a terminal email client                     │
│                                                                │
│  This wizard sets up your first mail account and a few         │
│  preferences. Should take about a minute.                      │
│                                                                │
│  [enter] continue   [s] skip and edit config.toml manually     │
└────────────────────────────────────────────────────────────────┘
```

Provider picker (80×24):

```
┌─ poplar setup ──────────────────────────────────── step 2 of 8 ┐
│                                                                │
│  Choose your mail provider                                     │
│                                                                │
│  > Fastmail              JMAP, paste an API token              │
│    Gmail                 OAuth (browser flow)                  │
│    Outlook / Microsoft   OAuth (browser flow)                  │
│    iCloud                IMAP, app password                    │
│    Yahoo                 IMAP, app password                    │
│    Zoho                  IMAP, app password                    │
│    Mailbox.org           IMAP, app password                    │
│    ProtonMail (Bridge)   local IMAP, Bridge required           │
│    Other / self-hosted IMAP   manual IMAP host + port          │
│    Other / self-hosted JMAP   manual JMAP session URL          │
│                                                                │
│  ↑/↓ select   enter confirm   ctrl-c quit                      │
└────────────────────────────────────────────────────────────────┘
```

Probe failure (80×24):

```
┌─ poplar setup ─────────────────────────── step 5 of 8 — probe ─┐
│                                                                │
│  Testing connection to imap.fastmail.com:993                   │
│                                                                │
│  Connecting…                                  ✓                │
│  TLS handshake…                               ✓                │
│  AUTHENTICATE PLAIN…                          ✗ AUTH failed    │
│                                                                │
│  Server says: Authentication failed. Check that you used an    │
│  app password, not your Fastmail account password. Generate    │
│  one at https://app.fastmail.com/settings/security/tokens      │
│                                                                │
│  [r] retry   [e] edit credentials   [s] save and quit          │
└────────────────────────────────────────────────────────────────┘
```

Probe success (sister to the failure wireframe; auto-advances 800ms):

```
┌─ poplar setup ──────────────────────── step 5 of 8 — probe ─┐
│                                                             │
│  Testing connection to imap.fastmail.com:993                │
│                                                             │
│  Connecting…                                  ✓             │
│  TLS handshake…                               ✓             │
│  AUTHENTICATE PLAIN…                          ✓             │
│  CAPABILITY (UIDPLUS)…                        ✓             │
│  STATUS INBOX…                                ✓ 1,247 msgs  │
│                                                             │
│              Connected. Continuing…                         │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

JMAP probe — same screen layout, JMAP-specific transcript:

```
┌─ poplar setup ──────────────────────── step 5 of 8 — probe ─┐
│                                                             │
│  Testing JMAP connection                                    │
│                                                             │
│  Resolving session URL…                       ✓             │
│  TLS handshake…                               ✓             │
│  Bearer authentication…                       ✓             │
│  Session/get…                                 ✓             │
│  Account/get…                                 ✓ 3 mailboxes │
│                                                             │
│              Connected. Continuing…                         │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

`wizard.Probe` dispatches on `cfg.Provider`: IMAP family produces the
IMAP transcript, JMAP family the JMAP transcript. Both feed the same
generic `ProbeStep { Label, Status, Detail }` shape, so the screen
layout is identical and only the strings differ.

Credentials — app-password variant (most providers):

```
┌─ poplar setup ───────────────────────── step 4 of 8 — credentials ┐
│                                                                   │
│  Fastmail uses app-specific passwords for IMAP, not your main     │
│  account password. Generate one at:                               │
│                                                                   │
│      https://app.fastmail.com/settings/security/tokens            │
│                                                                   │
│  Open in browser?      ( ) yes    (•) no, I have one              │
│                                                                   │
│  App password:        [••••••••••••••••••••              ]        │
│                                                                   │
│  ↑/↓ navigate   tab next field   enter confirm                    │
└───────────────────────────────────────────────────────────────────┘
```

Credentials — API-token variant (Fastmail JMAP):

```
┌─ poplar setup ───────────────────────── step 4 of 8 — credentials ┐
│                                                                   │
│  Fastmail uses a personal API token for JMAP. Generate one at:    │
│                                                                   │
│      https://app.fastmail.com/settings/security/tokens            │
│                                                                   │
│  Tokens don't expire unless you revoke them, so this is a         │
│  one-time setup.                                                  │
│                                                                   │
│  API token:           [••••••••••••••••••••••••••••••    ]        │
│                                                                   │
│  tab next field   enter confirm                                   │
└───────────────────────────────────────────────────────────────────┘
```

API-token differs from app-password by dropping the
"open in browser?" prompt (the token-management page is the only
place to get the token; the prompt is redundant) and by explaining
no-expiry to set expectations.

Credentials — plain-IMAP variant (self-hosted):

```
┌─ poplar setup ───────────────────────── step 4 of 8 — credentials ┐
│                                                                   │
│  Self-hosted IMAP server                                          │
│                                                                   │
│  Host:           [mail.example.com                       ]        │
│  Port:           [993        ]                                    │
│  Username:       [user@example.com                       ]        │
│  Password:       [••••••••••••••••                       ]        │
│                                                                   │
│  Skip TLS verify?  ( ) yes — only for self-signed certs           │
│                    (•) no                                         │
│                                                                   │
│  tab next field   enter confirm   esc back                        │
└───────────────────────────────────────────────────────────────────┘
```

The `Skip TLS verify?` confirm is gated on `Host` parsing to RFC1918
/ `.local` / `127.x` — for hosted providers the question stays
hidden.

Credentials — plain-JMAP variant (self-hosted):

```
┌─ poplar setup ───────────────────────── step 4 of 8 — credentials ┐
│                                                                   │
│  Self-hosted JMAP server                                          │
│                                                                   │
│  Session URL:    [https://jmap.example.com/.well-known/jmap     ] │
│  Username:       [user@example.com                              ] │
│  Token:          [••••••••••••••••                              ] │
│                                                                   │
│  Skip TLS verify?  ( ) yes — only for self-signed certs           │
│                    (•) no                                         │
│                                                                   │
│  tab next field   enter confirm   esc back                        │
└───────────────────────────────────────────────────────────────────┘
```

Differs from plain-IMAP by replacing `Host` + `Port` with a single
`Session URL` (the JMAP discovery endpoint), and using `Token`
instead of `Password` since most JMAP servers expect bearer tokens.
Stalwart and a few others accept passwords on the same field — the
same TOML key flows through, only the prompt label changes.

Theme picker with live preview (Section 8 only — every other step
is a single-pane form):

```
┌─ poplar setup ─────────────────────────────── step 8 — theme ────────┐
│                                                                      │
│  Choose a color theme — preview updates as you scroll                │
│                                                                      │
│  ┌─ themes ────────────┐  ┌─ preview ──────────────────────────┐     │
│  │   one-dark          │  │  ▎ Inbox                    ▎      │     │
│  │ > gruvbox           │  │  ▎ ──────                   ▎ Re:  │     │
│  │   nord              │  │  ▎ • Geoff Wright    11:47  ▎ The  │     │
│  │   dracula           │  │  ▎   Re: poplar setup...    ▎ wiza │     │
│  │   solarized-dark    │  │  ▎   Hannah W.        9:14  ▎ Yes! │     │
│  │   solarized-light   │  │  ▎   weekend plans          ▎      │     │
│  │   monokai           │  │  ▎ • Sarah K.        Tue    ▎      │     │
│  │   tokyonight        │  │  ▎   build status: green    ▎      │     │
│  │   catppuccin-mocha  │  │  ▎                          ▎      │     │
│  │   ...               │  │  ▎ q quit  / search  c new  ▎      │     │
│  └─────────────────────┘  └────────────────────────────────────┘     │
│                                                                      │
│  ↑/↓ navigate   enter confirm                                        │
└──────────────────────────────────────────────────────────────────────┘
```

The preview is a static fake email view rendered with the candidate
theme so the palette change reads at a glance.

Confirm + write:

```
┌─ poplar setup ─────────────────────────── step 9 — confirm ────────┐
│                                                                    │
│  Ready to write ~/.config/poplar/config.toml                       │
│                                                                    │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │  [[account]]                                               │    │
│  │  name         = "Fastmail"                                 │    │
│  │  provider     = "fastmail"                                 │    │
│  │  email        = "geoff@907.life"                           │    │
│  │  password-cmd = "op read op://Personal/Fastmail/credential"│    │
│  │                                                            │    │
│  │  [ui]                                                      │    │
│  │  theme = "gruvbox"                                         │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                    │
│  Write file and start poplar?    ( ) yes, but show me the diff     │
│                                  (•) yes, just write               │
│                                  ( ) no, cancel and quit           │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

The boxed TOML preview lets the user verify the assembled config
before commit. Email and identity steps (single inputs with no
visible decisions) are not wireframed — `huh.NewInput` defaults
render those without surprise.

## Error handling

**Malformed config (BACKLOG #29 extension)**: typed
`config.ConfigError` from validators. `runRoot` formats:

```
poplar: ~/.config/poplar/config.toml:42: account "fastmail":
  field "email" is required
  fix: add `email = "you@yourdomain.com"` under the [[account]] block
Run `poplar --repair=fastmail` to fix this account interactively.
Or edit the file by hand and rerun poplar.
```

**Probe failures**: `[r]` retry, `[e]` edit credentials only, `[s]`
save and quit. Save-and-quit writes the partial `[[account]]` block
as commented-out TOML so the validator doesn't reject the file on
next launch.

**Cancel mid-wizard**: `ctrl-c` triggers `Discard setup? [y/N]` via
the existing `uicore.ModalShell` confirm. Yes → exit 130, no config
written. No → return to current step.

## Config writing

`config.Render(accts []AccountConfig, ui UIConfig, cache CacheConfig)
[]byte` — emits canonical TOML, mirrors the template's
commented-options shape but uncommenting wizard choices. Idempotent:
rendering a config that was just loaded produces byte-identical
output. Atomic write via `config.toml.tmp` + fsync + rename, the
standard poplar pattern.

## Testing

- `internal/wizard/`: unit tests on `Apply`, `Probe` dispatcher,
  `Strategy` selection. Table-driven, no UI.
- `internal/ui/wizard/`: `wizard.Model.Update` tests with synthesized
  `tea.Msg`s. No live `tea.Program`. Per `elm-conventions`.
- `internal/config/`: `ConfigError` shape, `Render` round-trip,
  template-name-optional fix, providers' `CredentialStrategy`
  dispatch.
- `internal/mailjmap/`: `Probe` golden test against a recorded JMAP
  session.
- Live tmux capture at 80×24 and 120×40: every section visited,
  probe success + probe failure screens, malformed-config error
  formatting.

## Pass 14 task list (8–12 budget)

1. `internal/wizard/` skeleton: `Model`, `Section`, `Strategy`
   interface + non-OAuth implementations.
2. `wizard.Probe` dispatcher + new `mailjmap.Probe`. (IMAP probes
   already exist.)
3. `wizard.Apply` (state → AccountConfig); UIConfig delta application.
4. `internal/ui/wizard/`: huh integration, theme adapter, per-section
   sub-models, custom probe-screen `tea.Model`.
5. Section registry: `accountSection`, `themeSection`. Stub
   `contactsSection`, `signatureSection`, `tidySection` with
   `Hide()=true`.
6. `wizard.Run(mode)` orchestrator + state machine.
7. `config.ConfigError` typed errors + `runRoot` formatting +
   `--repair=<name>` flag.
8. `cmd/poplar config init --interactive` cobra subcommand;
   first-run auto-launch in `runRoot`; `--no-wizard` and
   `POPLAR_NO_WIZARD=1` opt-outs.
9. Config template rewrite (#29 fix).
10. Wordmark logo in `internal/ui/wizard/logo.go`. Embed
    `art/poplar-logo.ans` via `//go:embed` for future replacement.
11. Tests: wizard unit tests, config error tests, tmux captures.
12. ADR-0189 — wizard architecture, sections, huh dependency.

## Out-of-pass — Pass 14.1 preview

- `oauthStrategy` implementation (device-code + loopback).
- Token storage via OS keyring (`99designs/keyring` or similar) —
  exact selection brainstormed at the start of 14.1.
- `mailauth` refresh-on-401 helper, wired into `mailimap.Connect` so
  Gmail/Outlook accounts stay connected past first access-token
  expiry.
- Gmail/Outlook step in the wizard switches from placeholder
  password-cmd to live consent.
- ADR-0190 (or whatever number is next) — OAuth subsystem decisions.

## Dependencies

New: `charm.land/huh/v2` (MIT, Charm-maintained). Already in the
ecosystem poplar uses (lipgloss, bubbles, glamour-the-lib).

Build-time only (not vendored, not at runtime): `cbonsai`
(`gitlab.com/jallbrit/cbonsai`, GPLv3). Used to generate the
`art/poplar-logo.ans` artifact when whoever picks up the proper
logo decides on a final tree. Output is committed; runtime has no
dependency.
