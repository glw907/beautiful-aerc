# Pass 9f — Mail Backend Send + Append

Goal. Add `Send` and `Append` to `mail.Backend` and implement them on
both v1 backends. JMAP uses `Email/import` (Append) and the
`Email/import → EmailSubmission/set` pair (Send-with-atomic-Sent-copy).
Generic IMAP grows a third connection — a lazy SMTP client via
`emersion/go-smtp` — and gains `APPEND` for the Sent copy. The
`[account.smtp]` config block lands with provider-preset defaults.

## Settled (skipped brainstorm — see chat)

- **Backend.Send signature.** `Send(env Envelope, mime []byte) error`,
  where `Envelope = { From string; Rcpts []string }`. Layering rule:
  `internal/mail/` does not import `internal/compose/`. `compose.AssembleMIME`
  already returns bytes; `APPEND` and `Email/import` need known length.
  No `io.Reader`.
- **`Backend.Append` signature.** `Append(folder string, mime []byte,
  flags Flag) error`. Used for the IMAP Sent-copy path (JMAP collapses
  Send + Sent into one submission). Empty `flags` is the common case;
  `FlagSeen` is set on Sent copies.
- **SMTP connection lifetime.** Lazy on first `Send`. Major clients
  (mutt/alpine/aerc) all dial SMTP lazily. Servers drop idle SMTP
  aggressively. No third always-on connection. Reconnect on next Send
  if the cached client returned an error.
- **`config check` SMTP probe.** Sequential, alongside existing
  IMAP/JMAP probes. Matches existing pattern.

## Plan

### Task 1 — `mail.Backend` interface change

In `internal/mail/backend.go`:

- Define `type Envelope struct { From string; Rcpts []string }`.
- Replace `Send(from string, rcpts []string, body io.Reader) error`
  with `Send(env Envelope, mime []byte) error`.
- Add `Append(folder string, mime []byte, flags Flag) error`.

Update `internal/mail/mock.go` MockBackend, the cache fakeBackend in
`internal/cache/cache_test.go`, and `internal/mailjmap/jmap_test.go`
test that calls Send. Drop the `io` import.

### Task 2 — `[account.smtp]` config + presets

In `internal/config/`:

- Add `SMTPConfig struct { Host string; Port int; StartTLS bool;
  InsecureTLS bool; Auth string; Password string; PasswordCmd string }`
  to `accounts.go` and an embedded TOML-decoded `SMTP SMTPConfig` on
  `accountEntry` (TOML key `smtp`).
- Field on `AccountConfig`: `SMTP SMTPConfig`. Defaults filled from
  the provider preset's new SMTP fields when unset.
- Extend `Provider` in `providers.go` with `SMTPHost`, `SMTPPort`,
  `SMTPStartTLS`, `SMTPInsecureTLS`. Fill the existing IMAP presets
  with provider-published submission endpoints (gmail
  `smtp.gmail.com:465`, fastmail `smtp.fastmail.com:465`, yahoo
  `smtp.mail.yahoo.com:465`, etc. — implicit TLS / 465 by default,
  STARTTLS for the protonmail-bridge preset).
- Auth/credentials default to mirroring the IMAP credentials when the
  `[account.smtp]` block is absent or partial — typical case is "same
  username + password as IMAP." Explicit fields override.
- Validation: SMTP host required when `Backend == "imap"` (after preset
  resolution). JMAP accounts ignore `[account.smtp]` (warn at decode
  time).
- Update `template.go` and `template.golden` to document the block.

### Task 3 — JMAP Send + Append

In `internal/mailjmap/jmap.go`:

- Replace `Send` placeholder. `Send(env, mime)`:
  1. `client.Upload(accountID, bytes.NewReader(mime))` → `blobID`.
  2. Single JMAP request invoking `Email/import` (creates the
     EmailImport into the Sent mailbox role with `$seen $draft=false`)
     **then** `EmailSubmission/set` referencing the imported email's
     id via JMAP back-reference (`#emailId`). Atomic submit + Sent.
  3. Identity: cache `IdentityID` on first use via `Identity/get`.
     Match by `email == cfg.Email` (or `cfg.From.Address` when set).
- `Append(folder, mime, flags)`: resolve folder → mailboxID via the
  existing folder cache, `Upload`, then `Email/import` with the right
  keywords. Used by the cache outbox for non-Sent appends (e.g.
  manual `Drafts` save) — landing now keeps the surface uniform with
  IMAP.

Errors route through the existing `classifyErr` for `ErrAuth` /
`ErrNotFound`.

### Task 4 — IMAP Append + SMTP sibling

In `internal/mailimap/`:

- New file `smtp.go`. `smtpClient` interface (test seam). `smtpDial(cfg)`
  mirroring `auth.go`: implicit TLS (465) or STARTTLS (587), keepalive
  tuning, SASL via the same `authenticate`-style switch (plain / login /
  xoauth2). Reuse `mailauth.NewXoauth2Client` (it returns a
  `sasl.Client` and works for both IMAP and SMTP).
- `Backend.smtpMu`, `Backend.smtp` fields on `imap.go`. Lazy: first
  `Send` calls `smtpDial`, caches client. On any send error, drop the
  cached client (next Send redials). No goroutine.
- `Send(env, mime)`: if no SMTP cached, dial. `Mail(env.From) →
  Rcpt(each) → Data(mime)`. On error, classify (auth → `ErrAuth`),
  drop cached client, return wrapped error.
- `Append(folder, mime, flags)`: `cmd.Append(folder, &imap.AppendOptions{
  Flags: ...}, bytes.NewReader(mime), int64(len(mime))).Wait()`.
  Wraps `cmd.Append` from `go-imap/v2`.

`Backend.Disconnect`: also logout SMTP if connected.

### Task 5 — `poplar config check` SMTP probe

In `cmd/poplar/check.go` (or wherever `config check` lives) extend the
sequential per-account probe to additionally `smtpDial` + `Noop`-equivalent
+ logout for IMAP-backed accounts. JMAP accounts skip (submission is
in-band on the existing JMAP session).

### Task 6 — Tests

- `internal/mail/backend_test.go` (or extend mock test): `Envelope`
  shape exercised through MockBackend.
- `internal/config/accounts_test.go`: SMTP block decode, preset fill,
  validation errors, mirror-from-IMAP fallback.
- `internal/mailimap/smtp_test.go`: fake `smtpClient`, lazy connect,
  drop-on-error, auth dispatch.
- `internal/mailimap/actions_test.go`: `Append` round-trip via existing
  `fake_test.go` infrastructure.
- `internal/mailjmap/jmap_test.go`: `Send` happy path with fake
  `httpClient`, `Append` happy path, auth error → `ErrAuth`.

### Task 7 — Consolidation

ADR-0157 (Backend Send + Append shape). Update invariants.md
(`mail.Backend` row gets Send/Append; mailimap row mentions third
SMTP connection lazily dialed; mailjmap row mentions
`Email/import + EmailSubmission/set` pair). STATUS.md → Pass 9g
starter prompt. Archive plan. `make check`. Commit + push + install.

## Out of scope

- Cache outbox dispatch of Send/Append (Pass 9g).
- ComposeTab UI wiring (Pass 9h).
- DSN / delivery status surfacing.
- XOAUTH2 token refresh (Pass 9.6 first-run wizard).
