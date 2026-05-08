---
title: Pass 9n — Per-account identities and signatures
status: design
date: 2026-05-07
issue: "#32"
---

# Per-account identities and signatures

Add support for multiple sending identities per account and a
per-identity signature, surfaced in compose through a focusable
`From:` row, rendered (dimmed) in the reader, threaded through both
JMAP and IMAP send paths.

## Config

A new `[[account.identity]]` block array under each `[[account]]`:

```toml
[[account]]
name = "fastmail"
provider = "fastmail"
from = "Geoff <geoff@907.life>"   # legacy; promoted when no [[account.identity]] blocks

[[account.identity]]
name = "Geoff Wright"
email = "geoff@907.life"
signature = "-- \nGeoff Wright\nhttps://907.life"

[[account.identity]]
name = "Geoff @ ASC"
email = "geoff.wright@aksailingclub.org"
signature-file = "~/.config/poplar/signatures/asc.md"
```

`AccountConfig.Identities []Identity` (parsed, ordered slice).
`Identity{Name, Email string; Signature string}`.

Decode rules (in `config.LoadAccounts`):

- `signature-file` is read from disk at config-load and stored in
  `Signature`. Path expands `~`. Read errors fail validation
  (`identity %q: signature-file: %w`).
- `signature` and `signature-file` are mutually exclusive per
  block.
- A non-empty `Signature` that does not begin with `-- \n` is
  prepended with `-- \n` (RFC 3676 sentinel injection). A
  `Signature` that already begins with `-- \n` is preserved
  verbatim.
- Validation: `Email` parses via `net/mail.ParseAddress`; `Name`
  is non-empty.
- If `Identities` is empty after decode and the legacy
  top-level `from` is set, synthesize one identity from `from`
  with empty `Signature`. The synthesized identity is the
  default. If both are empty validation fails with the existing
  "account requires from or identities" shape.
- When `[[account.identity]]` blocks are present, the legacy
  top-level `from` is ignored (configs have one source of truth
  for the address list).

The first identity in TOML order is the default.

## Domain (`internal/compose`)

`Draft` gains:

```go
type Draft struct {
    // ...existing...
    Identity         int  // index into AccountConfig.Identities
    IncludeSignature bool // default true
}
```

`AssembleMIME` signature changes from
`AssembleMIME(d Draft, now time.Time)` to
`AssembleMIME(d Draft, identities []config.Identity, now time.Time)`.
The `From:` header is built from `identities[d.Identity]`.
When `d.IncludeSignature` and the chosen identity's `Signature`
is non-empty, the signature is appended to the markdown body
with a single `\n\n` separator before goldmark renders the HTML
alternative. The plain-text alternative includes the signature
verbatim.

`SeedReply`, `SeedReplyAll`, `SeedForward` initialize
`Identity = 0` and `IncludeSignature = true`. Recipient-aware
identity routing is deferred (see *Out of scope*).

## UI (`internal/ui/compose`)

### Focus

A new focus state `focusFrom` is added before `focusTo`. The
ordering becomes: `From → To → Cc → Bcc → Subject → Body`. Tab and
Shift-Tab cycle through it normally; the body's existing
"Shift-Tab back to Subject" behavior is unchanged for non-From
movement.

### Rendering

`headerRow("From:", value)` (already at `model.go:226`) renders the
current identity as `Geoff Wright <geoff@907.life>`. When focused,
a trailing `‹ ›` glyph is appended in `theme.MutedFg`; the
single-identity case still focuses but the cycler glyph is
suppressed.

### Keys (focus = `focusFrom`)

- `Space`, `→`, `l` — next identity (wrap to first).
- `←`, `h` — previous identity (wrap to last).
- All other keys forwarded to the parent (Tab / Shift-Tab / global
  send keys still work).

### Signature toggle

A footer hint reads `[F2] sig: on` (or `off`) when the active
identity's `Signature` is non-empty; absent otherwise. `F2` flips
`Draft.IncludeSignature`. Bare F-keys satisfy the
no-modifier-keybinding rule.

### Empty config edge case

If the parent passes an account with no `Identities` (legacy
`from`-only), the synthesized identity from config-load is what
the compose model receives, so the slice is always length ≥ 1.

## Send paths

### JMAP

`internal/mailjmap.Backend.identityID jmap.ID` becomes
`identityIDs map[string]jmap.ID` keyed by identity email.
`resolveIdentityID(accountID, email)` looks up by email; on cache
miss it issues `Identity/get` and matches by `Email`. The
`Identity/get` request is reused (single round-trip) and populates
the cache for all identities returned.

`Send` already takes the chosen From through `Envelope.From`; the
lookup uses it. If no matching identity is returned by the server,
the existing `"identity/get: no identity for %q"` error fires.

### IMAP / SMTP

`MAIL FROM` already uses `Envelope.From`. The MIME `From:` header
is now set per identity during `AssembleMIME`. The cached SMTP
client is reused across identities; no per-identity reconnect.

### Outbox

`SendArgs` carries `Envelope` and `Payload`. Identity choice
collapses into the From address (in `Envelope`) and the assembled
MIME (in `Payload`) at queue time. No schema bump needed.

## Reader

`internal/ui/reader.Model.renderBody` detects the RFC 3676 sentinel
(a line equal to `-- ` — note the trailing space) and wraps the
trailing region in `theme.MutedFg`. Detection is on the
post-decode plain-text body; HTML-only bodies are unaffected
(glamour-rendered text would lose the sentinel, which is the
correct stdlib-style behavior).

The signature is always visible — no fold/expand. This is the
"render the dimming, don't add chrome" treatment.

## Tests

`internal/config`:

- Identities decode, ordered.
- Legacy `from` synthesizes identity #1 when no
  `[[account.identity]]` blocks exist.
- `signature-file` resolution (relative + `~`).
- `signature` + `signature-file` exclusion.
- RFC 3676 sentinel injection idempotency.

`internal/compose`:

- `AssembleMIME` per identity index — From header correct.
- Signature append on / off.
- Sentinel idempotency in assembly path.

`internal/ui/compose`:

- Focus order including `focusFrom`.
- Cycler wrap (single identity no-op).
- F2 toggles signature only when the identity has one.

`internal/mailjmap`:

- Identity cache keyed by email; second identity sends after
  first identity has cached cause one additional `Identity/get`
  only on first miss.

`internal/ui/reader`:

- Sig region renders muted; non-sig body untouched.

## ADR

`docs/poplar/decisions/0177-identities-and-signatures.md` —
*Per-account identities, signatures, and the RFC 3676 sentinel.*
Records: identity array shape, legacy `from` migration, RFC 3676
delimiter, sig-file resolution at config-load, JMAP identity cache
by email, reader dim treatment.

## Out of scope

- Per-recipient identity auto-selection on Reply (route by
  `To:`/`Cc:` match against identities). Tracked separately for a
  follow-up pass.
- Editor-visible signature the user can hand-edit per-message.
- Multi-line `From:` / RFC 5322 group syntax.
- Per-identity SMTP credentials (one `[account.smtp]` per
  account remains the model).

## Invariants delta

`docs/poplar/invariants.md`:

- Replace the *Send + Append* note that today says "Identity/get
  resolves the identity ID on first Send and caches it on the
  Backend" with the per-email cache shape.
- Add the identity-array decode rules to the *Config & theming*
  section: top-level `from` is the legacy default; identity
  blocks override; first-in-order is the default.
- Add the reader sentinel-dim rule to the reader-rendering
  invariant set (lives in `.claude/rules/ui-invariants.md` —
  follow-up edit there as part of the pass).
