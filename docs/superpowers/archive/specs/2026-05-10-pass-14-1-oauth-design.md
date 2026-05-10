# Pass 14.1 — OAuth refresh for Gmail / Outlook IMAP

Companion to BACKLOG #42. Replaces the 14b OAuth strategy stub
(`echo TODO-pass-14.1-oauth`) with a real loopback-PKCE consent
flow, refresh-token persistence, and proactive access-token
refresh.

## Goal

Wire native OAuth for Gmail and Outlook IMAP onto Pass 14's
`CredentialStrategy` seam so those presets finish the wizard
end-to-end and the resulting account survives access-token
rotation without manual re-entry.

## Constraints inherited from earlier passes

- BYO client ID — user registers their own OAuth client in Google
  Cloud / Azure Portal once. No maintainer-verified client (Google
  restricted-scope verification requires an annual CASA audit;
  Microsoft requires publisher verification; both out of reach for
  solo OSS).
- `password-cmd` stays as the parallel auth path for self-hosted
  IMAP, Fastmail JMAP, and users who prefer their own secret
  manager. OAuth doesn't replace it.
- Pass 14's `CredentialStrategy` enum is the dispatch seam. No
  cross-cutting refactor of the IMAP/JMAP auth path.
- Token-refresh failures surface as `mail.ErrAuth`; the existing
  cmd/idle reconnect logic re-resolves and redials.

## Architecture

Three layers, each independently testable.

### `internal/mailauth/oauth.go` (UI-free)

```go
type Config struct {
    ClientID          string
    ClientSecret      string  // optional for PKCE; Google still expects it for installed apps
    AuthURL, TokenURL string
    Scopes            []string
    RedirectPortRange [2]int  // e.g. {8000, 8099}; loopback listener picks a free port
}

type Client struct {
    cfg         Config
    store       TokenStore
    accountSlug string
    // in-memory access-token cache (token + expiry)
}

func NewClient(cfg Config, store TokenStore, slug string) *Client

// Authorize drives loopback PKCE consent. Picks a free port in the
// configured range, builds the auth URL with PKCE challenge +
// random state, launches the browser, blocks on the loopback
// listener until callback or 2-minute timeout. Exchanges the code
// for refresh + access tokens; persists the refresh token via
// store.Set(slug, refresh).
func (c *Client) Authorize(ctx context.Context) error

// Token returns a valid access token. Refreshes via
// oauth2.TokenSource when <5min remains on the cached token.
// On upstream ErrAuth (401/invalid_grant), forces one refresh
// retry, then surfaces mail.ErrAuth.
func (c *Client) Token(ctx context.Context) (string, error)
```

Wraps `golang.org/x/oauth2` for the token-endpoint exchange.
Loopback callback server is hand-rolled `net/http.Server` on
`127.0.0.1:<port>` with a single-shot handler that validates
`state`, captures `code`, and sends it back to `Authorize`.
Browser launch via `xdg-open` on Linux, `open` on macOS, `cmd /c
start` on Windows — same pattern as the existing viewer
`URLOpener` seam.

### `internal/mailauth/tokenstore.go`

```go
type TokenStore interface {
    Set(account, refresh string) error
    Get(account string) (string, error)  // returns "", nil when missing
    Delete(account string) error
}

type KeyringStore struct{}        // wraps zalando/go-keyring
type AgeFileStore struct{ Dir string }  // per-account .age files

// OpenStore probes the keyring with a sentinel write+read+delete.
// Returns KeyringStore on success, AgeFileStore otherwise.
// Callers persist the chosen backend in config so subsequent
// opens skip the probe.
func OpenStore(slug string) (TokenStore, Backend, error)
```

- **KeyringStore** uses `github.com/zalando/go-keyring` with
  `service = "poplar-oauth"`, `user = <account slug>`. The Linux
  Secret Service backend (gnome-keyring / kwallet) is ambient on
  every modern Linux desktop; macOS Keychain and Windows Credential
  Manager cover the other two platforms.
- **AgeFileStore** persists each refresh token at
  `$XDG_CONFIG_HOME/poplar/oauth/<slug>.age`, encrypted to a
  per-account age identity at `<slug>.key` (mode 0600). No user
  passphrase — protection is filesystem perms (same trust level as
  `password-cmd = cat ~/secret`). Atomic write via temp-file +
  rename; identity created on first `Set`.
- The chosen backend records to the account block as `oauth-store
  = "keyring"` or `"age-file"`. Loaders read this verbatim; the
  probe runs only on first-write (wizard or `--reauth`).

### `internal/wizard/oauth_strategy.go`

`oauthStrategy` implements the existing `Strategy` interface from
14b. Drives a single huh-style sub-screen with a live transcript:

```
Opening browser…
Waiting for consent (max 2 min)…
Exchanging code…
Storing refresh token (keyring)…
Done.
```

- 2-minute timeout → `consent timed out, press Enter to retry` (re-
  runs `Authorize`).
- Browser-side cancel → same retry surface (loopback receives the
  cancel error, surfaced identically to timeout).
- `Esc` aborts the whole wizard step; the partially-written account
  block is dropped (matches existing wizard cancel semantics).
- `Apply()` returns a `config.AccountConfig` with `auth =
  "xoauth2"`, an `[account.oauth]` sub-block, and `oauth-store =
  "<backend>"`. No `password-cmd` field.

## Config shape

```toml
[[account]]
name     = "Work"
provider = "gmail"
email    = "user@gmail.com"
auth     = "xoauth2"
oauth-store = "keyring"

  [account.oauth]
  client-id     = "..."
  client-secret = "..."
  scopes        = ["https://mail.google.com/"]
  # auth-url / token-url filled from the gmail preset;
  # user-overridable but normally absent

  [account.smtp]
  host = "smtp.gmail.com"
  port = 465
  # auth path mirrors IMAP — xoauth2 against the same Client
```

`config.Provider` gains an optional `OAuth *OAuthDefaults{AuthURL,
TokenURL, DefaultScopes}` field. The `gmail` and `outlook` preset
entries fill it; non-OAuth presets leave it nil.

`AccountConfig` gains:

```go
OAuth      *OAuthConfig  // pointer so absence is unambiguous
OAuthStore string        // "keyring" | "age-file"
```

`config.Render` round-trips `[account.oauth]` (after `[[account]]`
but before `[account.smtp]` to match TOML decode order).
Validation: `provider` with `OAuth != nil` requires `client-id`
and (for gmail/outlook today) `client-secret`. Missing
`oauth-store` defaults to `"keyring"` on load — the writer always
emits it after wizard runs.

## Backend integration

`internal/mailimap/auth.go` already routes `auth = "xoauth2"` to
`mailauth.NewXoauth2Client`. The change: when `cfg.OAuth != nil`,
the access-token field comes from a `*mailauth.Client.Token(ctx)`
call instead of resolving `password-cmd`.

`mailimap.Backend` gains an `*mailauth.Client` field, constructed
once at `Connect` time when `cfg.OAuth != nil`. The cmd-goroutine
and idle-goroutine reconnect loops route `mail.ErrAuth` →
`Token(ctx)` (which itself force-refreshes once on upstream
`ErrAuth`) → tear down + redial. This matches the aerc-style
reconnect path already documented in
`docs/poplar/invariants.md`'s IMAP backend section.

SMTP shares the same `*Client`. The lazy-dialed SMTP connection
calls `Token(ctx)` on each `Send`; the cached client is dropped on
any send error (existing behavior — no new code needed beyond
threading the Client into the SMTP path).

## CLI surface

- **`poplar --reauth=<name>`** — sibling of `--repair=<name>`.
  Runs only the OAuth wizard sub-screen against the named account,
  replaces the stored refresh token, exits. Useful when the
  refresh token expires (Google rotates after 6 months of
  inactivity) or the user revokes consent.
- **`poplar config check`** — for OAuth accounts, calls
  `Token(ctx)` as a new first probe step (before TLS/auth) so a
  missing/expired refresh token surfaces clearly in the probe
  transcript rather than as a generic auth failure.

## Tests

- **`oauth_test.go`** — fake token endpoint via `httptest.Server`,
  in-memory `TokenStore` fake. Cases: happy-path Authorize,
  state-mismatch rejection, 2-minute timeout, refresh near expiry,
  forced refresh on upstream `ErrAuth`, refresh failure surfaces
  `mail.ErrAuth`.
- **`tokenstore_test.go`** — `KeyringStore` test skips on
  `keyring.ErrNotFound` so headless CI doesn't fail; `AgeFileStore`
  round-trips through a temp dir, verifies file mode 0600 and
  atomic-rename semantics.
- **`wizard/oauth_strategy_test.go`** — drives the sub-screen with
  a fake `Client` whose `Authorize` returns canned outcomes
  (success, timeout, cancel) and asserts the transcript + Apply()
  shape.
- **`mailimap` integration** — `mailimap.Probe` test with a fake
  IMAP server and a fake OAuth Client; verifies the Token() call
  in the auth step and the refresh-on-ErrAuth retry.

## Documentation

`docs/poplar/oauth-setup.md` — BYO registration walkthrough.

- **Gmail.** Create Google Cloud project → enable Gmail API → OAuth
  consent screen (external, restricted scope `https://mail.google.com/`,
  add yourself as a test user — the app stays in Testing status,
  which is fine for a personal client) → Credentials → Create OAuth
  client ID, type Desktop → copy client_id + client_secret into the
  wizard.
- **Outlook.** Azure Portal → App registrations → New registration
  (any name, "Accounts in any organizational directory and personal
  Microsoft accounts") → Authentication → Add platform → Mobile and
  desktop applications → tick the public-client checkbox → API
  permissions → add `IMAP.AccessAsUser.All`, `SMTP.Send`,
  `offline_access` (delegated, Microsoft Graph) → grant admin
  consent (or first user grants on login) → copy Application ID
  into the wizard.

Plain text — no screenshots in v1. Add later if the doc proves
hard to follow.

## ADR

One new ADR — next available number (likely 0193): **OAuth
refresh for Gmail/Outlook IMAP.** Covers BYO rationale, loopback
PKCE choice, `zalando/go-keyring` + age-file fallback, per-account
token-store layout, `--reauth` CLI surface, refresh-on-`ErrAuth`
reconnect semantics, decision to skip device-code in v1.

## Invariants delta

`docs/poplar/invariants.md`:

- § Architecture > Repo & libraries: add `internal/mailauth/`
  OAuth subsystem to the inventory line ("…XOAUTH2 SASL snippet,
  OAuth Client + TokenStore").
- § Architecture > Config & theming: add `[account.oauth]`
  description (parallel to `[account.smtp]` and
  `[[account.identity]]`). Note `oauth-store` field on
  `AccountConfig`. Note the wizard's OAuth sub-screen as a third
  step alongside account + theme + confirm.
- § Architecture > CLI: add `--reauth=<name>` flag.
- Credential resolver: `mailauth.Token(ctx)` becomes the third
  resolver alongside `password` and `password-cmd`; routing is by
  presence of `[account.oauth]`.

Update `docs/poplar/decisions/INDEX.md` with the new ADR row.

## Out-of-pass

- Maintainer-verified OAuth client (Google CASA audit + Microsoft
  publisher verification). Revisit if/when the project absorbs
  audit cost.
- Device-code flow for SSH-without-tunneling users. Add only if
  loopback-flow feedback identifies a real cohort that can't make
  it work.
- OAuth for JMAP. Fastmail uses long-lived API tokens; no consumer
  wants OAuth on JMAP.
- Per-account passphrase on the age-file fallback. Current trust
  level matches `password-cmd = cat ~/secret`; raise only if a
  shared-machine cohort surfaces.

## Rough task shape

1. `internal/mailauth/oauth.go` — `Client`, `Authorize`, `Token`,
   loopback callback server, PKCE helpers. Fake token endpoint
   tests.
2. `internal/mailauth/tokenstore.go` — `TokenStore` interface,
   `KeyringStore`, `AgeFileStore`, `OpenStore` probe. Tests via
   temp dir + skip-on-headless for keyring.
3. `internal/config/oauth.go` — `OAuthConfig` decode, `OAuthStore`
   field on `AccountConfig`, `Provider.OAuth` defaults, gmail +
   outlook preset updates, validator, `Render` round-trip.
4. `internal/wizard/oauth_strategy.go` — `oauthStrategy` impl,
   sub-screen transcript, retry + abort surfaces. Tests via fake
   Client.
5. `cmd/poplar/root.go` — `--reauth=<name>` flag wiring; calls
   into wizard with a single-strategy run.
6. `internal/mailimap/auth.go` + `smtp.go` — thread
   `*mailauth.Client` into Backend; route Token() into XOAUTH2;
   refresh-on-ErrAuth retry in cmd/idle reconnect.
7. `internal/mailimap/probe.go` — add Token() as first probe step
   for OAuth accounts.
8. `docs/poplar/oauth-setup.md` — BYO registration walkthrough.
9. ADR-0193 + invariants edits + INDEX.md row.
10. Plan/spec archive, STATUS pivot to Pass 15, pass-end commit.

8–10 tasks; well within the pass budget.
