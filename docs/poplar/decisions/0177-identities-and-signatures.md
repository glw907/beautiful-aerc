---
title: Per-account identities and signatures
status: accepted
date: 2026-05-08
---

## Context

Poplar accounts had a single sending address (legacy top-level
`from = "..."`) and no signature support. Users wanted per-account
identities (e.g. personal vs work address on the same Fastmail
account) and ordered signature lists per identity. Issue #32.

JMAP submission already required an `Identity/get` probe to pick
up the server's identity ID; the cache held one ID per Backend,
which broke as soon as a second identity was used.

## Decision

Add `[[account.identity]]` block arrays under each `[[account]]`,
each carrying ordered `[[account.identity.signature]]` sub-blocks.
`AccountConfig.Identities []Identity` is always length >= 1; an
account with no identity blocks synthesizes one from the legacy
`from`. First-in-order is the default. `Signature.Text` is read
from disk at config-load when `file = "..."` is set, and the
RFC 3676 `"-- \n"` sentinel is prepended idempotently.

`compose.Draft` carries `Identity int` and `Signature int` indices
(`Signature == -1` means "no signature appended"). `AssembleMIME`
takes the identities slice and appends the chosen signature's text
to the body before goldmark renders the HTML alternative.

Compose UI adds a `focusFrom` focus state at the end of the Tab
cycle (To→Cc→Bcc→Subject→Body→From). The From row renders
`Name <email>` plus a `· sig: <name>` chip (or `· no sig` when
the identity has signatures but the user cycled past the last
one; the chip is suppressed entirely when the identity has zero
sigs). When focus is on From, a dim ` ‹ ›` cycler glyph appears.
`Space`/`→`/`l` cycles identity forward; `←`/`h` backward; the
signature index resets to 0 (or -1 if the new identity has no
sigs). `Ctrl+G` cycles the active signature from any focus state;
bare `g` is the focusFrom-only convenience. ADR-0076's text-entry
exemption permits the chord. The footer surfaces `Ctrl+G sig`
when the identity has signatures and adds `Space/←→ identity` as
a second group when From has focus.

JMAP `Backend.identityID` becomes `identityIDs map[string]jmap.ID`.
On the first cache miss, one `Identity/get` probe populates the
map for every identity the server returns; subsequent sends hit
the cache regardless of which identity is in From.

## Consequences

- Multi-identity accounts work end-to-end: the chosen identity
  drives the From header, the JMAP submission identity, and (in
  the IMAP path) the assembled MIME From.
- Markdown signatures flow through the same goldmark pipeline as
  the body, so links, emphasis, and code render uniformly in the
  HTML alternative.
- The reader's existing RFC 3676 sentinel-dim treatment
  (`internal/content` + `theme.Signature`) carries over without
  change — outgoing signatures are dimmed in the reader on
  receipt by virtue of the sentinel.
- Migration is automatic and lossless: existing configs with only
  `from = "..."` keep working with one synthesized identity.
- Out of scope and queued for follow-ups: per-recipient identity
  auto-selection on Reply, editor-visible signatures, per-
  identity SMTP credentials, explicit `default-signature` flag,
  and a global identity-cycle chord.
