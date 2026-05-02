# IMAP backend (Pass 8) + provider registry, Gmail follow-up (Pass 8.1)

**Status:** accepted, 2026-05-01
**Pass:** 8 (generic IMAP), with follow-up Pass 8.1 (Gmail)

## Context

V1 poplar must support generic IMAP and Gmail. Today only Fastmail
(JMAP) works. The Pass 8 starter prompt named three open
questions:

1. Token refresh ownership (mailauth vs mailimap).
2. IDLE / keepalive strategy.
3. How `Destroy` (ADR-0092) maps to IMAP UID EXPUNGE.

Brainstorming widened scope: a four-way backend taxonomy
(`gmail` / `fastmail` / `imap` / `jmap`) backed by a provider
registry, with the existing `mailjmap` already serving as a
generic JMAP client. Sequencing flipped from "Gmail first, generic
IMAP second" to **"generic IMAP first, Gmail second"**:

- Generic IMAP iterates faster (local Dovecot in Docker, no real-
  account or OAuth dependency).
- Capability negotiation is structurally honest from day one
  rather than baked-in assumptions that have to be backed out.
- Gmail likely collapses to a provider preset on top of `mailimap`
  with one assert-not-negotiate flag and one `Destroy` precondition,
  rather than a separate package. Evidence-driven split if and only
  if real Gmail testing in Pass 8.1 reveals it's needed.

## Decision

### Two protocol packages

- `internal/mailimap/` (new, Pass 8) — generic IMAP. Negotiates
  capabilities, discovers folder roles via SPECIAL-USE (RFC 6154),
  uses MOVE (RFC 6851) when available and falls back to
  COPY+STORE+EXPUNGE otherwise, supports PLAIN/LOGIN/CRAM-MD5/
  XOAUTH2 SASL mechanisms, dual-connection (command + IDLE) model.
- `internal/mailjmap/` (existing) — already generic JMAP. Pass 8
  adds the `fastmail` and `jmap` config aliases that route to it.

Pass 8.1 reuses `mailimap` for Gmail via a `Provider: "gmail"`
flag. Only if that turns out to be untenable does Gmail get its
own `internal/mailgmail/` package.

### Provider registry (data, not code)

`internal/config/providers.go` declares a small map of presets:

```go
type Provider struct {
    Name        string
    Backend     string            // "imap" or "jmap"
    Host        string            // IMAP presets only
    Port        int
    StartTLS    bool
    URL         string            // JMAP presets only
    AuthHint    string            // "app-password" | "bearer" | "xoauth2"
    HelpURL     string
    GmailQuirks bool              // Pass 8.1 — assert-not-negotiate + Trash precondition
}

var Providers = map[string]Provider{
    "fastmail": {Backend: "jmap",
                 URL: "https://api.fastmail.com/jmap/session",
                 AuthHint: "bearer",
                 HelpURL: "https://app.fastmail.com/settings/security/tokens"},
    "yahoo":    {Backend: "imap",
                 Host: "imap.mail.yahoo.com", Port: 993,
                 AuthHint: "app-password",
                 HelpURL: "https://login.yahoo.com/account/security"},
    "icloud":   {Backend: "imap",
                 Host: "imap.mail.me.com",    Port: 993,
                 AuthHint: "app-password",
                 HelpURL: "https://appleid.apple.com"},
    "zoho":     {Backend: "imap",
                 Host: "imap.zoho.com",       Port: 993,
                 AuthHint: "app-password",
                 HelpURL: "https://accounts.zoho.com/home#security/app_password"},
    // Pass 8.1 adds:
    // "gmail": {Backend: "imap", Host: "imap.gmail.com", Port: 993,
    //           AuthHint: "app-password", GmailQuirks: true,
    //           HelpURL: "https://myaccount.google.com/apppasswords"},
}
```

Generic escape hatches stay outside the registry: `backend = "imap"`
takes user-supplied `host`/`port`/`starttls`/`auth`;
`backend = "jmap"` takes user-supplied `source`.

Outlook is deferred indefinitely. Microsoft killed basic auth for
personal Outlook in 2024 and their OAuth verification UX is worse
than Google's. Outlook users can fall back to `backend = "imap"`
with BYO XOAUTH2; no preset.

### Config shape

```toml
# Generic IMAP (Pass 8)
[[account]]
name     = "personal"
backend  = "imap"
email    = "user@example.com"
host     = "mail.example.com"
port     = 993
starttls = false
auth     = "plain"
password = "$IMAP_PASSWORD"

# Yahoo via preset (Pass 8)
[[account]]
name     = "yahoo"
backend  = "yahoo"
email    = "user@yahoo.com"
auth     = "plain"
password = "$YAHOO_APP_PASSWORD"

# Generic JMAP (Pass 8)
[[account]]
name    = "self-hosted"
backend = "jmap"
email   = "user@example.com"
source  = "https://jmap.example.com/.well-known/jmap"
auth    = "bearer"
password = "$JMAP_TOKEN"

# Fastmail via preset (Pass 8 — alias for jmap with preset URL)
[[account]]
name     = "fastmail"
backend  = "fastmail"
email    = "user@fastmail.com"
auth     = "bearer"
password = "$FASTMAIL_API_TOKEN"

# Gmail (Pass 8.1)
[[account]]
name     = "gmail"
backend  = "gmail"
email    = "user@gmail.com"
auth     = "plain"
password = "$GMAIL_APP_PASSWORD"
```

`AccountConfig` gains: `Auth` (string), `Email` (string),
`Host` (string), `Port` (int), `StartTLS` (bool),
`OAuthClientID`, `OAuthClientSecret`, `OAuthRefreshToken` (all
string, all env-var-substituted via the existing `$VAR` mechanism).
Provider presets fill in `Host` / `Port` / `URL` / etc. before
the dispatch step.

### Authentication (Pass 8)

`mailimap` supports four SASL mechanisms, dispatched off
`AccountConfig.Auth`:

- `plain` — `sasl.NewPlainClient("", email, password)` over
  implicit TLS or STARTTLS. Default for `imap`, `yahoo`, `icloud`.
- `login` — `sasl.NewLoginClient(email, password)` for legacy
  servers that lack PLAIN.
- `cram-md5` — `sasl.NewCramMD5Client(email, password)` for
  legacy servers that prefer challenge-response.
- `xoauth2` — Pass 8.1 surfaces this for Gmail Workspace. Spec'd
  here so the SASL adapter can land in either pass.

The XOAUTH2 path uses `golang.org/x/oauth2` (already a transitive
dep, promoted to direct in Pass 8 if used) for the token source
and a small adapter in `internal/mailauth/` (~15 lines) that
bridges `oauth2.TokenSource` → `sasl.Client`. Hand-rolled refresh
is forbidden — the canonical Go library handles it.

**No poplar-owned OAuth client.** Verification for the
`mail.google.com` scope requires an annual third-party CASA
security audit. Aerc and himalaya both punted; we follow.

### IDLE and keepalive

Two IMAP connections per `mailimap.Backend`:

- **Command connection** — used by every synchronous `mail.Backend`
  method (`QueryFolder`, `FetchHeaders`, `Move`, `Destroy`, …).
  Matches the sync contract from ADR-0075. TCP keepalive set via
  `internal/mailauth/keepalive`.
- **IDLE connection** — dedicated goroutine doing `IDLE` on the
  currently-selected folder, emitting `mail.Update` onto the
  channel returned by `Updates()`. TCP keepalive set.

Two connections instead of one because IMAP IDLE blocks the
connection — every user-initiated command would otherwise have to
coordinate a `DONE` with the idle loop, fighting bubbletea's
`tea.Cmd` model.

**9-minute IDLE refresh.** RFC 2177 caps IDLE at 29 minutes; many
servers tear connections down sooner (Gmail at ~10 minutes). Idle
goroutine issues `DONE` and re-`IDLE` every 9 minutes — the most
restrictive known limit, harmless on permissive servers. A
constant `idleRefreshInterval` makes the value tweakable per
provider in Pass 8.1 if needed.

**Folder switch protocol.** `OpenFolder(name)` updates the command
connection, then signals the idle goroutine via a channel. The
idle goroutine `DONE`s, `SELECT`s the new folder, re-`IDLE`s. The
active-IDLE folder always tracks the user-visible folder.

**Reconnect loop** mirrors `mailjmap.pushLoop`: 1s → 30s
exponential backoff on idle-connection failure; emit
`ConnReconnecting` / `ConnConnected` `mail.Update`s. The command
connection reconnects lazily on the next call and surfaces
failure as a normal returned error (handled by the existing
error banner per ADR-0073).

**IDLE capability check.** If the server doesn't advertise IDLE,
the idle goroutine still runs but polls (`STATUS <folder>
(UIDNEXT UNSEEN)` every 60 seconds) instead. Logged once at
Connect; not treated as a hard error since enough small-host IMAP
servers lack IDLE.

### Capability negotiation

At `Connect()`, after authentication, `mailimap` reads the
client's capability list and stores a `caps` struct:
`hasUIDPLUS`, `hasMOVE`, `hasIDLE`, `hasSpecialUse`, `hasXGM`.
All command paths gate behavior on these.

For the `gmail` preset (Pass 8.1), `Connect()` instead asserts
UIDPLUS+MOVE+IDLE+X-GM-EXT-1 are all present and returns a hard
error if any is missing. Gmail has shipped these for a decade;
silent fallback would mask deeper trouble.

### Folder discovery

- If the server advertises SPECIAL-USE (RFC 6154), `ListFolders()`
  uses `LIST "" "*" RETURN (SPECIAL-USE)` and maps the
  `\Drafts` / `\Sent` / `\Trash` / `\Junk` / `\Archive` / `\All`
  attributes to `Folder.Role`.
- Otherwise, falls back to a built-in alias table in
  `internal/mail/classify.go` (extend it for IMAP-name guesses
  like "Sent Items", "Deleted Items", "Junk E-Mail").
- Provider presets may carry hardcoded role maps that shortcut
  discovery — `gmail` (Pass 8.1) hardcodes the `[Gmail]/*`
  mapping.

### Move and Delete

- `Move(uids, dest)` — if MOVE capability advertised,
  `UID MOVE <set> <dest>`. Else COPY+STORE+EXPUNGE fallback in
  one atomic block on the command connection.
- `Delete(uids)` — soft delete to Trash. Resolves the Trash
  folder name from the role classification, then calls Move under
  the hood.
- `Copy(uids, dest)` — `UID COPY <set> <dest>`. No fallback
  needed; COPY is in IMAP4rev1 base.

### `Destroy(uids)` mapping

1. Empty input → return nil. Matches the ADR-0092 contract.
2. Verify UIDPLUS is in the cap set; refuse to construct backend
   at Connect time if not (plain `EXPUNGE` is too dangerous).
3. `UID STORE <set> +FLAGS.SILENT (\Deleted)` — mark only the
   named UIDs.
4. `UID EXPUNGE <set>` (UIDPLUS) — remove only those UIDs.
5. Non-existent UIDs in the set are silently ignored by the
   server. That's the IMAP analogue of JMAP's `notFound`-as-success
   rule.

For the `gmail` preset (Pass 8.1), an additional precondition:
verify the currently-selected folder is `[Gmail]/Trash` and
return `ErrDestroyOutsideTrash` if not. Gmail's label semantics
make `EXPUNGE` outside Trash a silent label-removal — the message
survives in `[Gmail]/All Mail`. Generic IMAP doesn't need this
guard; plain IMAP semantics genuinely permanent-delete.

### Pass 8.1 — Gmail

A separate, smaller spec/plan will be written when Pass 8 is done
and we have real `mailimap` ergonomics to design against. Expected
shape:

- One struct literal in `Providers` map (`gmail` preset).
- Wire `Provider.GmailQuirks` through to a `mailimap.Backend`
  flag that flips assert-not-negotiate for capabilities and
  enables the Trash precondition.
- Surface XOAUTH2 SASL adapter in `internal/mailauth/`.
- Real-Gmail testing pass: live verify against a personal Gmail
  account.
- Tweak `idleRefreshInterval` per provider if Gmail's 9-minute
  limit needs to be different from generic.

If Gmail's quirks turn out to be too pervasive to live as flags
on `mailimap`, split into `internal/mailgmail/` then. Evidence
first, package second.

## Sequencing

**All three passes are v1 ship-blockers.**

| Pass | Scope | v1 status |
|---|---|---|
| 8 | `mailimap/` + `imap` and `jmap` escape hatches + `fastmail`/`yahoo`/`icloud`/`zoho` presets in registry. Capability negotiation, SPECIAL-USE folder discovery, MOVE-with-fallback, PLAIN/LOGIN/CRAM-MD5 SASL. Tested against local Dovecot in Docker. | required |
| 8.1 | Gmail support — `gmail` preset routed to `mailimap` with `GmailQuirks: true`, XOAUTH2 SASL adapter in `mailauth/`, real-Gmail test pass. | required |
| 8.2 | **Configuration UX brainstorm + implementation.** Friendly low-friction setup: pick provider from common list (Gmail, Yahoo, Fastmail, Zoho, …), supply custom domain if applicable, supply username, done. Generated `accounts.toml` is clean and well-documented. Shape (interactive subcommand vs guided template vs config doctor) brainstormed in pass. | required |

## File map (Pass 8)

| Action | File | Purpose |
|---|---|---|
| Create | `internal/mailimap/imap.go` | `Backend` struct, `New`, `Connect`, `Disconnect`, command-connection lifecycle, capability negotiation |
| Create | `internal/mailimap/folders.go` | `ListFolders` (SPECIAL-USE + alias fallback), `OpenFolder` |
| Create | `internal/mailimap/messages.go` | `QueryFolder`, `FetchHeaders`, `FetchBody`, `Search` |
| Create | `internal/mailimap/actions.go` | `Move` (with COPY+STORE+EXPUNGE fallback), `Copy`, `Delete`, `Destroy`, `Flag`, `Mark*`, `Send` (latter returns "not implemented" until Pass 9) |
| Create | `internal/mailimap/idle.go` | Idle connection lifecycle, 9-min refresh loop, folder-switch coordination, reconnect loop, polling fallback when IDLE absent |
| Create | `internal/mailimap/auth.go` | `dialCommand`, `dialIdle` — TLS dial (implicit or STARTTLS), keepalive, SASL mech selection |
| Create | `internal/mailimap/*_test.go` | Table-driven unit tests with a fake IMAP client interface |
| Create | `internal/config/providers.go` | `Provider` struct + `Providers` map (fastmail, yahoo, icloud) |
| Create | `internal/config/providers_test.go` | Registry lookup + preset-resolution tests |
| Modify | `internal/config/account.go` | Add `Auth`, `Email`, `Host`, `Port`, `StartTLS`, `OAuthClientID`, `OAuthClientSecret`, `OAuthRefreshToken` fields |
| Modify | `internal/config/accounts.go` | Resolve `backend = "<preset>"` against `Providers`, fill in fields, env-var substitute the new fields |
| Modify | `internal/config/accounts_test.go` | Cover preset resolution + new field decode |
| Modify | `cmd/poplar/root.go` | Dispatch `imap`/`yahoo`/`icloud` → `mailimap.New`, `jmap`/`fastmail` → `mailjmap.New` |
| Modify | `internal/mail/classify.go` | Extend alias table for IMAP-style folder name guesses |
| Modify | `internal/mail/classify_test.go` | Cover new aliases |
| Modify | `go.mod` | Add `github.com/emersion/go-imap` v1 |
| Modify | `docs/poplar/invariants.md` | Update backend roster fact |
| Create | `docs/poplar/decisions/00NN-*.md` | ADR(s) per design decision (numbered at consolidation step) |

## Testing

- Unit tests use a fake IMAP client interface (subset of
  emersion/go-imap v1 surface that mailimap uses), the same
  pattern `mailjmap` uses for `jmapClient`.
- **Integration test against local Dovecot.** A `make test-imap`
  target spins up `docker run -d -p 1143:143 dovecot/dovecot`,
  creates a test user with a known password, runs a tagged
  `go test -tags=integration ./internal/mailimap/...` against it,
  and tears down. Skipped in `make test`; opt-in for local dev.
  Documented in `internal/mailimap/README.md`.
- Live verification: connect to Yahoo or iCloud with a real
  account, walk through inbox listing, fetch a body, mark read,
  move to Trash, empty Trash. Manual.
- No tmux capture required (no UI surface changes). If any
  error-banner copy is touched, the 80×24 capture rule applies
  per ADR-0097.

## Open risks

- `emersion/go-imap` v1 IDLE handler signature: confirm during
  implementation that the v1 API exposes the unilateral EXISTS /
  EXPUNGE / FETCH responses we need to translate into
  `mail.Update` values. Should be straightforward; v1 is the
  current mainline.
- Dovecot's default capabilities may not match the polish target
  (e.g., SPECIAL-USE may need to be enabled in `dovecot.conf`).
  Document the test-server config in `internal/mailimap/README.md`.
- IMAP rate limits are server-specific. Two connections per
  backend is comfortably under any documented limit. If multi-
  account dashboards ever share a connection pool, revisit.

## Consequences

- Poplar gains its second protocol package and a registry-driven
  provider model. Adding new presets in future is one struct
  literal each.
- `internal/mailauth/` becomes the home for SASL adapters
  generally — XOAUTH2 (Pass 8 or 8.1), possibly OAUTHBEARER
  later.
- The two-connection-per-backend pattern sets the precedent for
  any future protocol with an "RPC channel + push channel" shape.
- ADR-0092's "bypassing Trash" semantics: generic IMAP genuinely
  destroys via STORE+UID EXPUNGE; Gmail (Pass 8.1) needs the
  Trash precondition. Worth a sentence in the invariants update.
- Pass 9 (compose / send) inherits the SASL machinery and the
  expanded `AccountConfig`; SMTP's auth code drops in with no
  new design.
