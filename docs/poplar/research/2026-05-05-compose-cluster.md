# Compose-cluster prior art

*Research compiled 2026-05-05 to inform Pass 9h.5 (drafts), Pass 9.1
(address autocomplete), and Pass 9.4 (signatures + multiple
identities). Authority of last resort for these passes — when a
brainstorm or review hits a "but how do other clients handle X"
question, this document is the answer. Cells flagged "couldn't
verify" are honest gaps; do not paper over them.*

Three independent research passes against Thunderbird, Apple Mail,
Gmail, Fastmail, mutt/NeoMutt, aerc, plus relevant RFCs. Synthesized
here in pass order.

---

## Drafts (Pass 9h.5)

### Storage location

Server-side is the consensus default for connected clients. None of
the surveyed mainstream clients buffer drafts only locally:

- **Thunderbird (IMAP)** APPENDs to the configured Drafts folder on
  every auto-save tick. No local-only mode.
- **Apple Mail (IMAP)** saves to server Drafts every ~30 s. Has a
  user-accessible "Mailbox Behaviors" toggle to force local-only.
- **Gmail web** stores all drafts server-side; the Gmail API exposes
  draft objects with stable `id` that persists across edits.
- **Fastmail web (JMAP)** writes to the server Drafts mailbox
  immediately. Help docs document the multi-tab policy: "we save
  the most recently edited one."
- **aerc** APPENDs on `:postpone` to `config.Postpone` (default
  `Drafts`). **No auto-save** — explicit only. Autosave-drafts
  ticket #192 is open and unimplemented.
- **NeoMutt/mutt** APPENDs on `:postpone` to `$postponed`. No
  auto-save. On recall, the stored draft is marked `\Deleted`
  + `\Purge` and removed during `hardclose`.

**Drafts mailbox discovery.** IMAP uses RFC 6154 SPECIAL-USE — the
server advertises `\Drafts` on the LIST response; clients query
`LIST "" "*" RETURN (SPECIAL-USE)`. Fallback for non-SPECIAL-USE
servers is name heuristic (Drafts / Draft / `[Gmail]/Drafts`). The
per-message `\Draft` flag (RFC 9051 §2.3.2) is orthogonal — it
marks an individual message, not the mailbox. JMAP uses
`Mailbox/get` filtered by `role: "drafts"`; RFC 8621 §2 guarantees
at most one mailbox per account per role.

### Update semantics

IMAP has no update-in-place. The universal pattern is **APPEND new
copy → mark old `\Deleted` → EXPUNGE**:

- Thunderbird APPENDs new, marks old `\Deleted` in the same save.
  EXPUNGE deferred to folder close. On Gmail this races and produces
  multiple stranded copies (Bugzilla 1505789, 402132). TB uses
  the APPENDUID response to target the previous UID — servers
  without UIDPLUS break the chain.
- Apple Mail same APPEND + deferred-delete. 30 s cadence + slow
  pipeline drain → multiple stranded copies on busy servers.
- aerc on `:postpone` APPENDs only. If recalled (`RecalledFrom()`
  set), the old copy is **not auto-deleted in postpone.go** —
  caller-managed. Gap relative to TB.
- NeoMutt: explicit `:postpone` APPENDs; recall sets `MUTT_DELETE`
  + `MUTT_PURGE`, purged at `hardclose`.

**JMAP** has stable email IDs and `Email/set update` for metadata.
But RFC 8621 §4.3 forbids modifying `bodyStructure`, `bodyValues`,
`textBody`, `htmlBody`, `attachments` via update — so body changes
require **destroy + re-import**, not in-place. The idiomatic
pattern is `Email/set { destroy: ["<old-id>"] }` + `Email/import
{ "k1": { blobId: "<new-blob>", mailboxIds, keywords }}` in one
batch with a back-reference to chain them atomically. Fastmail's
reference impl uses `Email/set create` + `EmailSubmission/set` with
`onSuccessDestroyEmail` to atomically send and clean up.

**Cost quantification (IMAP auto-save).** N saves = N APPENDs +
(N-1) `STORE \Deleted` operations. TB at 5 min over 30 min = ~6
APPENDs. Apple Mail at 30 s = ~60. Gmail web at 3–8 s =
potentially hundreds. **JMAP eliminates this churn entirely** for
metadata-only changes; for body changes it's still destroy +
re-import per save.

### Auto-save cadence

| Client      | Trigger    | Default        | Configurable             |
|-------------|------------|----------------|--------------------------|
| Thunderbird | Time-based | **5 min**      | `mail.compose.autosaveinterval` |
| Apple Mail  | Time-based | **~30 s**      | No documented knob       |
| Gmail web   | Time-based | **3–8 s**      | No                       |
| Fastmail web| Likely event/blur + periodic | Unknown | **Couldn't verify primary source** |
| aerc        | None       | Explicit `:postpone` only | N/A           |
| NeoMutt     | None       | Explicit `:postpone` only | N/A           |

The TUI peers (aerc, NeoMutt) deliberately ship no auto-save. This
is a precedent worth weighing — explicit save avoids the GUI APPEND
storm.

### Resume UX

All surveyed clients surface drafts through the Drafts folder in
the standard sidebar; no separate "Unsent" surface is universal.
Selection opens compose. **No client implements deduplication or
warning for multiple drafts to the same recipient.** aerc uses
`:recall <message>` to open a postponed message in a new compose
tab; NeoMutt presents a list dialog when opening `$postponed`.

### Cross-device sync

**IMAP.** Server-side storage = visible cross-device within one
IDLE/poll cycle. Conflict resolution if two devices edit the same
draft simultaneously: **none** — last writer wins at APPEND level.
Each device's APPEND produces a new UID; whichever delete races
first targets a stale UID. Result: orphaned messages and/or
duplicates. No surveyed client implements CRDT or merge.

**JMAP.** `Email/set update` is optimistic-locked via `ifInState`
(RFC 8621 §4.3). Mismatched state → `stateMismatch` error → client
re-fetches and retries. In practice last-write-wins at ms
resolution; `ifInState` is conflict *detection*, not *resolution*.
Fastmail's offline-sync blog post describes a time-ordered change
log with replay-on-reconnect.

### IMAP/JMAP path summary

| Property | IMAP | JMAP |
|---|---|---|
| First save | APPEND with `\Draft` flag | `Email/import` (blob → ID) |
| Body update | APPEND new + UID EXPUNGE old | Destroy old + `Email/import` new (one batch) |
| Metadata-only update | APPEND new + UID EXPUNGE old | `Email/set update` (keywords, mailboxIds) |
| Mailbox discovery | RFC 6154 `\Drafts` SPECIAL-USE | `Mailbox/get` `role: "drafts"` |
| On send | Caller `UID EXPUNGE <draft-uid>` | `EmailSubmission/set onSuccessDestroyEmail` |

Poplar already has `Append(folder, mime, flags)` and the JMAP
destroy + import pattern. The persistence layer needs to (a) track
the live draft's server-side ID/UID after first save, (b) issue a
replace on each subsequent save, (c) clean up on send.

### Couldn't verify

- Fastmail web's exact auto-save cadence and whether it uses
  `Email/set update` or destroy + re-import for body changes.
- Whether RFC 8508 `REPLACE` is advertised by any production server
  in poplar's target set.
- aerc's full recalled-from cleanup chain (compose.go's `onClose`
  was not fully readable in the GitHub renderer).

---

## Address autocomplete (Pass 9.1)

### Source priority

**Thunderbird** searches all enabled address books (Personal Address
Book + Collected Addresses + CardDAV directories + LDAP) in
parallel; no enforced source priority. Single ranked list sorted
by `popularityIndex`. Bug 1114751 (request to promote PAB over
Collected) was rejected — devs argued the real fix is recency,
not source priority. CardDAV books require
`enable_autocomplete = true`.

**Apple Mail** merges Contacts (synced CardDAV/iCloud) with
"Previous Recipients" (local, non-synced). Contacts entries get a
card icon. No documented priority between sources.

**Gmail** uses Google Contacts (CardDAV-accessible) + sent /
interaction history ("Other contacts"). "Most contacted" weights
frequency server-side; algorithm not public. CardDAV-synced
contacts can take 24 h to index.

**Fastmail web** uses contacts (JMAP Contacts + CardDAV) directly.
Whether sent history contributes is not documented; their
JMAP-direct architecture suggests contacts-only or
contacts-plus-server-side-history.

**mutt + khard / aerc + maildir-rank-addr** delegate entirely to
external processes. mutt has no built-in address store; everything
goes through `query_command`. aerc's `address-book-cmd` is the same
contract. The popular community tool `maildir-rank-addr` adds
sent-history frecency ranked scoring.

### Sent-history collection

**Thunderbird** stores in `history.sqlite` (78+; previously
`history.mab`). Harvest trigger: on send. `popularityIndex` raw
counter, no decay. Fields: primary email, lowercase email, second
email, display name, popularityIndex. Frecency (Bug 382415)
proposed but never shipped.

**Apple Mail** writes "Previous Recipients" on send. Columns: Name,
Email, Last Used. Local, non-synced.

**Gmail / Fastmail** server-side; schema not public.

**maildir-rank-addr** harvests local Maildir. Three classes:
2 = explicit To/Bcc on sent (highest), 1 = Cc on sent, 0 =
everything else. Total rank = frequency rank + recency rank.
Built-in noreply exclusion regex (see below). Output is a TSV the
aerc `address-book-cmd` greps.

### vCard handling

RFC 6350 §5.1: PREF is 1–100, lower = more preferred. "Interpret
only relative to other instances of the same property in the same
vCard." FN (formatted name, RFC 6350 §6.2.1) is mandatory and
canonical for display.

In practice:

- **Thunderbird** extracts "first and second preference email
  addresses" from the vCard, respecting PREF ordering. Both
  queryable for autocomplete.
- **khard** outputs all email addresses as separate rows (one per
  address) tagged with type (work/home/etc.) — user picks. No
  PREF-based suppression.
- **aerc/mutt** inherit whatever the external tool returns.

All mainstream clients use FN for display.

### Display format

`"Name" <email>` is universal. Email-only contacts (no FN, history
entry without name) fall back to bare email. Multiple contacts
sharing a name show both — disambiguation only by visible address.
khard/aerc tab-separated output is `email\tname\ttype`; aerc
ignores the third field, mutt can show it as a third column.

### Ranking

| Client | Ranking |
|---|---|
| Thunderbird | Prefix-match over infix-match, then `popularityIndex` (raw send count, no decay) |
| Apple Mail | UI sort options: Name / Email / Last Used. No frequency surfaced. Contacts-vs-history priority **couldn't verify** |
| Gmail | Server-side "Most Contacted" — frequency + recency; algorithm not public |
| Fastmail web | Likely PREF-first within vCard, then alphabetical, possibly server-side sent-history boost. **Couldn't verify** |
| maildir-rank-addr | class 2 > class 1 > class 0; within class, frequency rank + recency rank |

### CardDAV refresh cadence

RFC 6352 + RFC 6578 standard sync ladder:

1. Session-start `PROPFIND` requesting `getctag` and `sync-token`.
2. CTag unchanged → no work.
3. CTag changed, sync-token available → `sync-collection` REPORT
   with previous token. Server returns only changes + new token.
4. Sync-token unavailable/expired → full `addressbook-query` REPORT
   (`Depth: 1`, request `getetag`), then `addressbook-multiget`
   to fetch changed vCards by ETag diff.
5. Initial fetch → full `addressbook-query` requesting both
   `getetag` and `address-data`.

**Fastmail CardDAV.** Uses vCard 3.0 (not 4.0; confirmed by
their troubleshooting docs and DAVx5 testing). Sync-token support
not explicitly documented; CTag/ETag fallback is the safe path.
Fastmail prefers JMAP for contacts internally — CardDAV is a
compatibility layer.

### Privacy / autoblocking

No mainstream client ships documented noreply exclusion in
first-party autocomplete. The pattern lives in community tooling.
**maildir-rank-addr** excludes by regex: `do-not-reply`,
`donotreply`, `no-reply`, `bounce`, `noreply`, `no.reply`,
`no_reply`. Configurable. The de-facto reference for TUI clients.

For poplar's CardDAV-only v1, exclusion is less urgent (CardDAV
contacts are intentionally curated). When sent-history lands,
maildir-rank-addr's pattern is the right model.

### TUI external-tool contract

**mutt `query_command`:**

- Config: `set query_command = "khard email --parsable %s"`
- `%s` = user query string. Runs on Tab in address fields.
- Output: one result per line, tab-separated `email\tname\ttype`.
  First line may be a header/count; `--remove-first-line` strips it.
- Return: 0 = success with results, non-zero = no results.

**aerc `address-book-cmd`:**

- Config: `address-book-cmd = khard email --remove-first-line --parsable %s`
- `%s` = text after the last comma in the address field. No stdin.
- Output: tab-delimited `email\tname[\t...]`. First field email
  (required). Second field name (optional). Additional fields
  ignored. Alternative: `address-book-cmd = carddav-query %s`.

Both clients treat completion as **pure external invocation** —
the client is a tab-completion dispatcher, not an address store.
Native CardDAV completion in poplar doesn't have to match this
contract, but the `%s → email\tname` line shape is a useful
minimal contract for any future CLI-composable hook.

### Couldn't verify

- Apple Mail's Contacts-vs-Previous-Recipients priority order.
- Fastmail web's autocomplete ranking algorithm.
- Whether Fastmail CardDAV advertises sync-token (treat as
  CTag/ETag fallback only until proven otherwise).

---

## Signatures + identities (Pass 9.4)

### Identity model

| Client | Structure |
|---|---|
| Thunderbird | Sub-records of an account. `mail.identity.idN.*` prefs. Per-identity: `fullName`, `email`, `reply_to`, `organization`, `doBcc`/`doBccList`, `fcc_folder`, `smtpServerKey`, signature text/file, `sig_bottom`, `suppress_signature_separator` |
| Apple Mail | Comma-separated aliases on one account. Signatures per-account, dragged to accounts. **Per-alias signature unsupported in standard Mail** |
| Gmail | "Send mail as" — each identity is a sibling verified sender: name, address, optional reply-to, per-alias HTML signature, default-sender flag. Per-alias SMTP relay optional |
| Fastmail | Per-account "Identities" screen. Each: sending name, email, optional reply-to, signature (HTML or plain), per-folder default-sender |
| mutt/NeoMutt | No first-class identity object. Assembled via `from`, `alternates` regexps, `send-hook`/`reply-hook` patterns |
| aerc | One `[AccountName]` section per account. Identity fields on the account: `from`, `aliases` (fnmatch wildcards in address part), `reply-to`, `signature-file`, `signature-cmd`, `copy-to`, `original-to-header`. Aliases on the account, not sub-records |

**TB's sub-record model is the richest reference and maps cleanly
to poplar's planned `[[account.identity]]` TOML block.** Apple
Mail's flat aliases-only model is the most restrictive and is the
common user complaint about Mail.

### Identity selection on reply

- **Thunderbird** scans the parent's To and Cc against all
  configured identities across all accounts. Match function is
  `emailSimilar()` — normalizes `+suffix` so
  `you+lists@example.com` matches `you@example.com`. First exact
  match wins; fallback is the account holding the folder. Lives in
  `comm/mailnews/compose/src/nsMsgCompose.cpp::getBestIdentity`.
- **Apple Mail** uses the receiving account for replies; for new
  mail, "Automatically select best account" matches first
  recipient's domain.
- **Gmail / Fastmail** match the address the message was *delivered
  to* (the alias) and pick that identity as From.
- **mutt** uses `reply-hook` patterns; `alternates` defines "you".
- **aerc** uses `aliases` field. `original-to-header` (e.g.
  `X-Original-To`) provides catch-all delivery fallback.

### Identity selection on new compose

- **Thunderbird** uses the active account's default identity. No
  documented active last-used logic.
- **Apple Mail** "Send new mail from" — specific account or
  domain-match best-account.
- **Gmail** per-account default sender.
- **Fastmail** per-folder default identity is first-class.
- **mutt** `send-hook` on compose, `folder-hook` per folder.

### Signature placement

| Client | Default | Override |
|---|---|---|
| Thunderbird | Bottom (after quote) | `mail.identity.idN.sig_bottom` (true=bottom, false=above quote) |
| mutt/NeoMutt | After all content | `sig_on_top = yes` flips |
| Apple Mail | Above quote (between new content + quote) | "Place signature above quoted text" toggle |
| Gmail | Above quote | "Insert signature before quoted text" checkbox, default on |
| Fastmail | Configurable | Per-direction (reply vs forward): above original / below original / none |

### Editable in buffer vs auto-appended

Every surveyed mainstream client materializes the signature into
the visible compose buffer at compose time. The user can edit or
delete it before sending. Identity-switch behavior:

- **Thunderbird** swaps the old signature for the new one on
  identity change, locating the boundary via the `-- ` delimiter.
  Unique among the surveyed set.
- **Gmail** Insert Signature menu in the compose toolbar lets users
  override the default with a different stored signature.
- **Apple Mail / Fastmail / mutt / aerc** insert once at compose
  open; identity change does not auto-swap.

### RFC 3676 `-- ` delimiter

RFC 3676 §4.3: "DASH DASH SP" + CRLF. Generators must not end a
paragraph with this line. Receivers must detect before testing
quoted lines, and again after stripping quote marks (handles
quoted signatures).

| Client | Adds automatically? |
|---|---|
| NeoMutt | Yes (`sig_dashes = yes` default) |
| Thunderbird | Yes (`mail.identity.idN.suppress_signature_separator = false` default). Suppressed when `sig_on_top` (above quote) because the marker would tag the quote as sig |
| aerc | No — docs recommend including manually in signature file |
| Apple Mail | No |
| Gmail / Fastmail | No |

**Stripping on quote.** Thunderbird strips the parent's `-- ` block
from quoted text — this is TB's primary use of the delimiter.
Greyed sigs in the viewer come from the same detection. mutt can
colorize via `color signature` but doesn't auto-strip from quotes.
Apple/Gmail/Fastmail do not auto-strip. **TB's
identity-swap-by-marker + parent-sig-strip-by-marker are the two
behaviors that make `-- ` worth honoring even if poplar doesn't
do anything else with it.**

### Plain vs HTML in multipart/alternative

When the sig is HTML (Fastmail, TB HTML mode), no surveyed client
enforces symmetric plain-text-alternative sig handling
automatically:

- **Thunderbird (HTML mode)** HTML sig in `text/html` part. Plain
  alternative auto-generated from HTML; sig formatting collapses.
  No separate plain-text sig field.
- **Gmail / Fastmail** HTML-only sig. Plain alternative is
  HTML-stripped; sig separator not added.
- **mutt / aerc** plain-text only; multipart/alternative comes from
  external tools (`muttdown`, etc.) which handle sig conversion.

**For poplar's markdown model.** Single markdown body (sig
included) goldmark-renders to HTML. The `text/plain` part is the
raw markdown; the `-- ` line passes through to both alternatives
unchanged — appears as a literal line in plain, and as a paragraph
of two-dashes-and-space in HTML (not semantically sig-separator
unless styled). The markdown-to-MIME toolchain (`App::Smarkmail`,
`muttdown`) demonstrates the cleaner pattern: detect the sig block
in the markdown source *before* rendering, render the sig
separately, optionally wrap in `<div class="signature">` in HTML
while preserving raw `-- \n` prefix in the plain part.

### Signature storage

| Client | Storage |
|---|---|
| Thunderbird | Inline text or external file (plain/HTML). `mail.identity.idN.sig_file` / `sig_text`. Per-identity |
| Apple Mail | `~/Library/Mail/V*/MailData/Signatures/*.mailsignature` (HTML). Per-account |
| Gmail | Inline HTML in account settings. Per-alias |
| Fastmail | Inline HTML or plain. Per-identity |
| mutt/NeoMutt | `signature = ~/.sig` (path) or `signature = "fortune \|"` (pipe). Single global; hooks override per-context |
| aerc | `signature-file = /path` or `signature-cmd = command`. Per-account |

**No mainstream client stores signatures as markdown.** Power-user
tools (Drafts app, markdown-here) demonstrate the pattern.
Markdown-as-source is a poplar-specific choice — fits the
"markdown body throughout" principle but has no prior art to
copy.

### Reply quote + signature interactions

Forward behavior:

- **Fastmail** explicit "Signature position for forwards"
  setting — independent of reply.
- **Gmail** same placement as reply.
- **Thunderbird** follows `sig_bottom`; no separate forward toggle.

Parent sig stripping:

- **Thunderbird** strips parent's `-- ` block from quote (primary
  use of the delimiter; greyed sig in viewer same mechanism).
- **mutt** `$quote_regexp` + `$attribution` + `color signature`
  for visualization; no auto-strip from quote.
- **Gmail / Apple / Fastmail** no auto-strip.

### Couldn't verify

- TB `getBestIdentity` Cc-vs-To priority and tie-breaking.
- TB `mail.identity.default.lastused` — listed in pref docs but
  not confirmed as actively used in default reply selection.
- Apple Mail per-alias signatures — community posts say no, no
  official confirmation.
- Fastmail forward sig handling: confirmed separate position
  setting; whether it strips the identity sig entirely (vs just
  positioning) not confirmed.

---

## Cross-cutting observations

A few patterns that surfaced across all three topics:

**Markdown as poplar-specific source.** Drafts (markdown body in
the cache), autocomplete (CardDAV vCard fields are not markdown,
but the surrounding compose buffer is), and signatures (`-- `
delimiter sits in markdown source, passes through goldmark) all
flow through one markdown body that becomes both alternatives at
assembly time. This is poplar's specific choice — none of the
surveyed clients do it. Existing toolchains (`muttdown`,
`App::Smarkmail`) demonstrate sig-aware splitting but neither is a
direct model.

**TUI peers ship simpler.** aerc and mutt/NeoMutt deliberately
ship without auto-save, without first-class identities, and
without built-in autocomplete — they delegate to external
processes (`query_command`, `address-book-cmd`) and pre-existing
files (`signature`, `~/.sig`). This is the precedent space poplar
has been operating in. The mainstream-GUI feature set (auto-save
every N seconds, identity-swap-on-reply, address-book-with-FN)
isn't expected of TUI peers but is expected of any client a user
brings to a non-TUI desktop.

**RFC 3676 `-- ` is worth honoring even passively.** Even if
poplar doesn't auto-strip parent sigs, just emitting `-- ` before
the signature block makes Thunderbird recipients see the right
greyed-sig treatment. Skipping it doesn't break interop (no
mainstream client requires it) but emitting it is courteous and
free.

**JMAP eliminates IMAP draft churn for metadata, not bodies.**
The temptation is to assume `Email/set update` is the JMAP
equivalent of in-place edit. It isn't — body changes still
require destroy + re-import. But the destroy + re-import is one
atomic batch with a back-reference, so the user-visible UID/blob
behavior is single-version. Whereas IMAP every save produces a new
UID and an old `\Deleted` UID, leaving residue if EXPUNGE doesn't
fire promptly. Picking the destroy + re-import shape on JMAP and
the APPEND + UID-EXPUNGE shape on IMAP gives parity.

**Identity-on-reply is delivery-address-match, not header scan.**
Gmail and Fastmail (the two big providers in poplar's path) both
match the address the message was *delivered to* (i.e., the
recipient envelope address) for identity selection — not the To
header. Thunderbird's full-header scan with `+suffix`
normalization is more thorough but rarer. Poplar's brainstorm
should pick its level deliberately; matching delivery-address is
the simpler/safer default.

---

## Sources

### Drafts

- [RFC 8621 — JMAP for Mail](https://www.rfc-editor.org/rfc/rfc8621) §2 (mailbox roles), §4.3 (Email/set), §4.8 (Email/import)
- [RFC 9051 — IMAP4rev2](https://www.rfc-editor.org/rfc/rfc9051) §2.3.2 (\Draft flag)
- [RFC 6154 — IMAP LIST Special-Use Mailboxes](https://www.rfc-editor.org/rfc/rfc6154)
- [RFC 4315 — IMAP UIDPLUS](https://www.rfc-editor.org/rfc/rfc4315)
- [aerc postpone.go](https://github.com/rjarry/aerc/blob/master/commands/compose/postpone.go) — flags, no auto-delete
- [aerc autosave-drafts ticket #192](https://todo.sr.ht/~rjarry/aerc/192) — open
- [NeoMutt postpone_8c.html](https://code.neomutt.org/postpone_8c.html) — `MUTT_DELETE | MUTT_PURGE` on recall
- [Fastmail Drafts help](https://www.fastmail.help/hc/en-us/articles/7962404002319-Drafts) — multi-tab "most recently edited" policy
- [Fastmail JMAP samples (joelparkerhenderson)](https://github.com/joelparkerhenderson/demo-fastmail-api-jmap/blob/main/bin/send-email) — Email/set + EmailSubmission/set + onSuccessDestroyEmail
- [Thunderbird Bugzilla 1505789](https://bugzilla.mozilla.org/show_bug.cgi?id=1505789) — APPEND+deferred-delete race
- [Thunderbird Bugzilla 402132](https://bugzilla.mozilla.org/show_bug.cgi?id=402132) — Gmail no-arbitrary-header-search

### Address autocomplete

- [RFC 6350 — vCard 4.0](https://www.rfc-editor.org/rfc/rfc6350.html) §5.1 (PREF), §6.2.1 (FN)
- [RFC 6352 — CardDAV](https://datatracker.ietf.org/doc/html/rfc6352)
- [sabre/dav — Building a CardDAV Client](https://sabre.io/dav/building-a-carddav-client/)
- [Thunderbird Developer Docs — Address Book](https://developer.thunderbird.net/thunderbird-development/codebase-overview/address-book)
- [Mozilla Bug 1114751 — Autocomplete priority](https://bugzilla.mozilla.org/show_bug.cgi?id=1114751)
- [Mozilla Bug 565465 — Most-frequent suggestion](https://bugzilla.mozilla.org/show_bug.cgi?id=565465)
- [khard scripting docs](https://khard.readthedocs.io/en/latest/scripting.html)
- [aerc-config(5) man page](https://man.archlinux.org/man/aerc-config.5.en)
- [maildir-rank-addr — GitHub](https://github.com/ferdinandyb/maildir-rank-addr)
- [DAVx5 tested with Fastmail](https://www.davx5.com/tested-with/fastmail)
- [Fastmail CardDAV blog](https://www.fastmail.com/blog/carddav-your-contacts-everywhere-you-need-them/)
- [TbSync CardDAV autocomplete issue 461](https://github.com/jobisoft/TbSync/issues/461)

### Signatures + identities

- [RFC 3676 §4.3 — Usenet Signature Convention](https://www.rfc-editor.org/rfc/rfc3676#section-4.3)
- [Thunderbird: Using Identities](https://support.mozilla.org/en-US/kb/using-identities)
- [Thunderbird: Configuration Options for Identity](https://support.mozilla.org/en-US/kb/configuration-options-identity)
- [Thunderbird: Signatures](https://support.mozilla.org/en-US/kb/signatures)
- [Thunderbird Source Docs: Accounts, Servers and Identities](https://source-docs.thunderbird.net/en/latest/backend/accounts.html)
- [Thunderbird nsMsgCompose.cpp — getBestIdentity](https://fossies.org/linux/thunderbird/comm/mailnews/compose/src/nsMsgCompose.cpp)
- [TB Bugzilla 433824 — sig_bottom per-identity](https://bugzilla.mozilla.org/show_bug.cgi?id=433824)
- [NeoMutt neomuttrc(5)](https://manpages.ubuntu.com/manpages/bionic/man5/neomuttrc.5.html)
- [NeoMutt Configuration Guide](https://neomutt.org/guide/configuration)
- [aerc-accounts(5)](https://man.archlinux.org/man/aerc-accounts.5.en)
- [aerc automated signature selection by alias](https://tjex.net/hacks/aerc-automated-signature-selection)
- [Fastmail: How to create a signature](https://www.fastmail.help/hc/en-us/articles/360058753754-How-to-create-a-signature)
- [Fastmail: Identities](https://www.fastmail.help/hc/en-us/articles/1500000280401-Identities)
- [Gmail: Create a signature](https://support.google.com/mail/answer/8395)
- [Gmail: Aliases and signatures via API](https://developers.google.com/workspace/gmail/api/guides/alias_and_signature_settings)
- [Apple Mail: Edit text and email signatures](https://support.apple.com/guide/mail/edit-text-and-email-signatures-mail11943/mac)
- [App::Smarkmail (CPAN)](https://metacpan.org/pod/App::Smarkmail) — markdown-to-multipart with sig handling
