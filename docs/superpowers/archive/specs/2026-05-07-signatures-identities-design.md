---
title: Pass 9n — Per-account identities and signatures
status: design
date: 2026-05-07
issue: "#32"
---

# Per-account identities and signatures

Add support for multiple sending identities per account, with an
ordered list of signatures per identity, surfaced in compose
through a focusable `From:` row plus a global signature-cycle
chord, rendered (dimmed) in the reader, threaded through both JMAP
and IMAP send paths.

## Config

A new `[[account.identity]]` block array under each `[[account]]`,
each carrying an ordered list of `[[account.identity.signature]]`
sub-blocks:

```toml
[[account]]
name = "fastmail"
provider = "fastmail"
from = "Geoff <geoff@907.life>"   # legacy; promoted when no [[account.identity]] blocks

[[account.identity]]
name = "Geoff Wright"
email = "geoff@907.life"

  [[account.identity.signature]]
  name = "default"
  text = "-- \nGeoff Wright\nhttps://907.life"

[[account.identity]]
name = "Geoff @ ASC"
email = "geoff.wright@aksailingclub.org"

  [[account.identity.signature]]
  name = "formal"
  file = "~/.config/poplar/signatures/asc-formal.md"

  [[account.identity.signature]]
  name = "casual"
  text = "-- \nGeoff"
```

`AccountConfig.Identities []Identity` (parsed, ordered slice).
`Identity{Name, Email string; Signatures []Signature}`.
`Signature{Name, Text string}`.

Decode rules (in `config.LoadAccounts`):

- `text` and `file` are mutually exclusive per signature block;
  exactly one must be set.
- `file` is read from disk at config-load and stored in
  `Signature.Text`. Path expands `~`. Read errors fail validation
  (`identity %q signature %q: file: %w`).
- A non-empty `Signature.Text` that does not begin with `-- \n`
  is prepended with `-- \n` (RFC 3676 sentinel injection). A
  `Text` that already begins with `-- \n` is preserved verbatim.
- `Signature.Name` is non-empty and unique within its identity
  (used in the From-row chip and footer hints).
- Identity validation: `Email` parses via `net/mail.ParseAddress`;
  identity `Name` is non-empty. An identity may have zero
  signatures (cycler keys go inert; "no signature" is the only
  state).
- If `Identities` is empty after decode and the legacy
  top-level `from` is set, synthesize one identity from `from`
  with no signatures. The synthesized identity is the default.
  If both are empty, validation fails with the existing
  "account requires from or identities" shape.
- When `[[account.identity]]` blocks are present, the legacy
  top-level `from` is ignored (configs have one source of truth
  for the address list).

The first identity in TOML order is the default; the first
signature within that identity is the default at compose-open.

## Domain (`internal/compose`)

`Draft` gains:

```go
type Draft struct {
    // ...existing...
    Identity  int // index into AccountConfig.Identities
    Signature int // index into Identities[Identity].Signatures, or -1 for none
}
```

`AssembleMIME` signature changes from
`AssembleMIME(d Draft, now time.Time)` to
`AssembleMIME(d Draft, identities []config.Identity, now time.Time)`.
The `From:` header is built from `identities[d.Identity]`. When
`d.Signature >= 0`, the chosen signature's `Text` is appended to
the markdown body with a single `\n\n` separator before goldmark
renders the HTML alternative; the plain-text alternative includes
the signature verbatim. When `d.Signature == -1`, no sig is
appended.

`SeedReply`, `SeedReplyAll`, `SeedForward` initialize
`Identity = 0` and `Signature = 0` (or `-1` if the default
identity has no signatures). Recipient-aware identity routing is
deferred (see *Out of scope*).

## UI (`internal/ui/compose`)

### Focus

A new focus state `focusFrom` is added before `focusTo`. The
ordering becomes: `From → To → Cc → Bcc → Subject → Body`. Tab and
Shift-Tab cycle through it normally; the body's existing
"Shift-Tab back to Subject" behavior is unchanged for non-From
movement.

### Rendering

`headerRow("From:", value)` (already at `model.go:226`) renders
the current identity as `Geoff Wright <geoff@907.life>`, followed
by a `· sig: <name>` chip (or `· no sig` when `Signature == -1`,
or absent when the identity has zero configured signatures). The
chip is rendered in `theme.MutedFg` and stays visible regardless
of focus, providing the always-visible affordance that identity
and signature are stateful.

When focused, a trailing `‹ ›` glyph is appended in
`theme.MutedFg`. The single-identity-and-no-sigs case still
focuses (Tab still walks through it) but the cycler glyph is
suppressed.

### Keys

**Identity cycling (focus = `focusFrom` only).**

- `Space`, `→`, `l` — next identity (wrap to first). On change,
  `Signature` resets to `0` if the new identity has any sigs,
  else `-1`.
- `←`, `h` — previous identity (same reset rule).

**Signature cycling.**

- `g` (focus = `focusFrom`) — cycle the active identity's
  signatures. Bare letter, free in this focus state because no
  textinput is consuming it.
- `Ctrl+G` (any compose focus, including `focusFrom`) — same
  cycle. The chord covers the focus states where bare letters go
  to a textinput / textarea, per ADR-0076's text-entry exemption.
- Cycle order: `0 → 1 → … → N-1 → -1 (none) → 0`. Both keys are
  inert when `len(Signatures) == 0`.
- The From-row chip updates immediately so the user sees the new
  signature name regardless of where focus sits.

**Footer hints.**

- Always (compose chrome footer): include `Ctrl+G sig` in the
  compose hint group when the active identity has at least one
  signature, alongside the existing `Ctrl+X send` / `Ctrl+C
  cancel`. Drops first under width pressure.
- When `focusFrom`: footer adds `Space/←→ identity` and shows
  the sig hint as `Ctrl+G sig` (the universal binding) even
  though bare `g` also fires here. Surfacing the chord teaches
  the key that works in every focus state; the bare-letter
  shortcut is a quietly-available power-user variant.

### Empty config edge case

If the parent passes an account with no `Identities` (legacy
`from`-only), the synthesized identity from config-load is what
the compose model receives, so the slice is always length ≥ 1.
The synthesized identity has zero signatures, so cycler keys are
inert.

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

- Identities decode, ordered; signatures decode under each
  identity, ordered.
- Legacy `from` synthesizes identity #1 (zero signatures) when no
  `[[account.identity]]` blocks exist.
- Signature `file` resolution (absolute + `~`).
- Signature `text` + `file` exclusion.
- RFC 3676 sentinel injection idempotency on signature `Text`.
- Identity with zero signatures decodes successfully.

`internal/compose`:

- `AssembleMIME` per identity index — `From:` header correct.
- `AssembleMIME` per signature index — `Signature == -1` omits
  sig; `>= 0` appends the named signature's `Text`.
- Sentinel idempotency in assembly path.

`internal/ui/compose`:

- Focus order including `focusFrom`.
- Identity cycler wrap; `Signature` resets correctly when
  identity changes (to `0` if new identity has sigs, else `-1`).
- Signature cycler wrap including the `(none)` stop; both `g`
  (focusFrom only) and `Ctrl+G` (any focus) walk the same cycle.
- Inert-key behavior when the active identity has zero
  signatures.
- From-row chip renders `· sig: <name>` / `· no sig` / absent
  cases.

`internal/mailjmap`:

- Identity cache keyed by email; second identity sends after
  first identity has cached cause one additional `Identity/get`
  only on first miss.

`internal/ui/reader`:

- Sig region renders muted; non-sig body untouched.

## ADR

`docs/poplar/decisions/0177-identities-and-signatures.md` —
*Per-account identities, signature lists, and the RFC 3676
sentinel.* Records: identity array shape, per-identity signature
list, legacy `from` migration, RFC 3676 delimiter, signature-file
resolution at config-load, JMAP identity cache by email, the
focusFrom UX (chip + cycler keys + `Ctrl+G`-vs-`g` mapping),
reader dim treatment.

## Out of scope

- Per-recipient identity auto-selection on Reply (route by
  `To:`/`Cc:` match against identities). Tracked separately for a
  follow-up pass.
- Editor-visible signature the user can hand-edit per-message.
- Multi-line `From:` / RFC 5322 group syntax.
- Per-identity SMTP credentials (one `[account.smtp]` per
  account remains the model).
- Per-identity `default-signature = "<name>"` flag. First-in-
  TOML-order is the default at compose-open in v1; add the flag
  only when someone wants a non-first default.
- Global identity-cycle chord (e.g. `Ctrl+T`). Identity is a per-
  message decision typically set at compose-open; mid-body
  identity changes are rare enough that a Tab to From is fine.

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
