# Poplar Rebuild: Canonical Functional Spec

**Status:** In progress. Assembled across the domain passes (charter §9),
consolidated at Pass 8.
**Provenance:** Charter `docs/superpowers/specs/2026-05-29-poplar-rebuild-charter.md`;
field survey `docs/poplar/research/2026-05-29-mail-client-gap-analysis.md`.

This document locks behavior, the stack, the load-bearing seams, and the
numbered acceptance scenarios. It leaves internal structure to the build
plans. Each domain pass appends its section. Decisions are derived from
current best practice and the comparator field first; the existing poplar
implementation is evidence, not the baseline.

---

## 1. Accounts, protocols, sync

Owner: Pass 1. Settles charter §6 decisions 1 and 2.

### 1.1 Account model and multi-account

An account is one mail identity domain: one backend (`jmap` or `imap`), its
own credentials, one or more sending identities, and its own SQLite cache.
Accounts are fully independent sync-and-storage domains; nothing joins across
them at the storage layer.

Poplar is account-partitioned with a unified view (the Thunderbird and K-9
model). Each account presents its own classified folder tree. On top of that
sits a **unified inbox**: a virtual cross-account view that merges every
account's Inbox into one triage list. The unified inbox is a read-side
computation, never a stored entity. Writes are always account-scoped: every
row in the unified list carries its owning account, and reply, move, archive,
flag, and label dispatch to that account's backend and cache. Unified scope is
the inbox alone for v1; unified Sent or unified Flagged are deferred as
low-value, easy later additions.

### 1.2 Folders and labels

Every message has exactly one **folder** and zero or more **labels**.

Folders are single-membership and define a message's location. They classify
per account through the pure function `Classify([]Folder) []ClassifiedFolder`
(Inbox, Sent, Trash, Archive, Drafts, Junk, Custom). Provider names normalize
to canonical display names regardless of JMAP or IMAP naming. Move and triage
semantics operate on the folder.

Labels are the multi-membership overlay: account-scoped, server-backed tags
that persist through the backend's native mechanism (JMAP keyword, Gmail
`X-GM-LABELS`, IMAP custom keyword). Each account advertises a `SupportsLabels`
capability. When false (a plain IMAP server that will not persist custom
keywords, judged from `PERMANENTFLAGS`), the label surface is absent for that
account and the UI says so rather than faking it. Labels are additive. The
folder tree stays the primary navigation, so a folder-only user never meets
label chrome they did not ask for. Gmail is adapted to this model: its system
labels classify as folders, its user labels become poplar labels.

### 1.3 The Backend interface

JMAP and IMAP are coequal behind one synchronous, blocking interface, so no
code above the backend branches on protocol. The backend calls its library
directly with no pump goroutine and no async bridge. Multi-account is a set of
backends, one per account.

The interface covers connect and teardown; reads (list classified folders,
query a folder, fetch a body, fetch attachments); mutators `Flag(uids, flag,
set)`, `Move(uids, dest)`, `Destroy(uids)`, and `Label(uids, label, set)`;
outbound `Send(env, mime)` and `Append(folder, mime, flags)`; and a push
channel the backend feeds from IMAP IDLE or the JMAP event source. Delete is a
move to Trash. `Destroy` is the irreversible permanent-delete primitive.

Each backend advertises its capabilities so the UI gates features at the
boundary instead of failing mid-call. `SupportsLabels` is the capability this
pass adds. The same mechanism already covers MOVE versus COPY-fallback,
SPECIAL-USE, and IDLE versus poll.

### 1.4 Connection and sync model

The cache is the source of truth for everything the UI reads. Sync reconciles
each account's cache with its server in the background and emits change events
the UI redraws from.

Delta sync uses each protocol's native machinery, not a shared lowest common
denominator. JMAP holds the per-type state string and pulls only what moved
through `Email/changes` and `Mailbox/changes`. IMAP uses CONDSTORE and QRESYNC
(RFC 7162) where the server advertises them, fetching by `MODSEQ` and learning
vanished UIDs in one round trip, and falls back to `UIDVALIDITY` plus
UID-range fetch otherwise.

Because the unified inbox is a first-class always-present surface, it stays
live across every account. Every configured account connects at startup and
keeps its Inbox under push (a second IMAP connection running IDLE, or the JMAP
event source). The active account additionally keeps its open folder under
push when that folder is not the Inbox. Other folders refresh by interval poll
and on first open. Connecting all accounts at startup is the v1 default; a
lazy-connect knob waits until a heavy-account user needs it.

### 1.5 Identities and alias auto-selection

An account has one or more identities. An identity is a From address, an
optional display name, and an ordered list of signatures. Each signature sets
exactly one of `text` or `file` and carries the RFC 3676 `"-- \n"` separator;
names are unique within an identity.

On reply, poplar selects the identity whose address matches the address the
message was delivered to (To, Cc, and `Delivered-To` where present), so a mail
that arrived at an alias replies as that alias. The candidate set is the
owning account's identities. Poplar also supports **alias-pattern identities**:
an identity may match an address pattern (for example `*@mydomain`), so a reply
to any address under a wildcard domain sends from the right address without a
config entry per alias. This serves the audience's common Fastmail
custom-domain catch-all setup.

A fresh compose from the unified inbox has no owning message. It defaults to a
**primary account** (first in config order, or an explicit `primary` flag) and
that account's first identity, switchable from the compose header, so
"compose from the merged view" needs no modal account picker.

### 1.6 OAuth and credentials

The audience is Gmail-first, so OAuth onboarding is the majority's first
experience and gets first-class design attention. Fastmail, the primary
maintainer's own backend, authenticates JMAP with a bearer API token and does
not enter the OAuth path.

OAuth follows RFC 8252 (OAuth 2.0 for Native Apps). The consent flow runs in
the system browser, never an embedded webview. Poplar is a public client, so
PKCE (RFC 7636) is always used and no client secret is treated as confidential.
A browser-present session captures the redirect on a loopback address
(`http://127.0.0.1:<random-port>/`). Headless and remote sessions, common for a
TUI over SSH, use the device authorization grant (RFC 8628): poplar prints a
URL and short code, the user approves on another device, and poplar polls for
the token. Provider device-flow support differs; Microsoft supports it broadly,
and Google's device-flow support for Gmail scopes is limited, so the Gmail
headless fallback is loopback over an SSH-forwarded port. Google's exact
device-flow scope support is confirmed at build time. Scopes are
least-privilege: mail read, modify, and send, not full-account access.

Client credentials are bring-your-own for v1. Gmail mail scopes are restricted
(verified against Google policy 2026-05-29): a shipped public client serving
many users would need a CASA security assessment, an annual paid org-shaped
process beyond a pre-1.0 solo project. So the user supplies a client ID and a
provider preset fills the rest, and the setup is a showcase-quality guided
flow. That guided flow drives the user's Google Cloud project to "In
production" status, because a project left in "Testing" issues refresh tokens
that expire in 7 days whenever Gmail scopes are present. Because the credential
source stays in config, shipping a verified client later is a preset change,
not a redesign; verification is a tracked post-1.0 milestone, not a v1
commitment.

Tokens persist in the OS keyring (Secret Service, Keychain, Windows Credential
Manager) with an age-encrypted file as the headless fallback. A stored token
that fails refreshes once and retries before surfacing an auth error. The
account name namespaces everything that could collide between accounts: the
keyring entries, the token store, and the cache DB filename. Password accounts
resolve `password` or `password-cmd` on first connect. CardDAV contacts
credentials fall back to the parent account.

### 1.7 Cache and sync contract

A local message cache is a core requirement, for daily-use performance as much
as for offline availability. The principle is **performance-by-locality**:
every read the user does repeatedly returns from local SQLite and never
round-trips the network. List render, thread expansion, and search all hit the
cache. This is why search is local-index-first: server IMAP SEARCH is too slow
and inconsistent, and a JMAP query is a network hop, so search runs against a
local FTS5 index in the per-account DB (Pass 6 owns its shape). Search stays
responsive enough for search-as-you-type.

Folder lists and message metadata (envelope, flags, thread linkage, `SentAt`)
sync eagerly per folder so lists render instantly. Bodies and attachments
fetch on demand and then cache. The raw MIME is the stored source of truth for
each body, because reply-quoting, re-rendering, and the Pass 4 rendering work
need the original. Attachment bytes store as size-bounded blobs rather than
bloating the hot tables.

Outbound and mutating actions go through a durable, optimistic operation queue
(the outbox). A mutation applies to the local cache and writes its outbox row
in the same SQLite transaction, so the optimistic change and the queued op are
atomic. A drainer dispatches rows to the backend when connected. On failure it
reconciles through a conflict matrix routed by typed sentinel (`ErrAuth`,
`ErrNotFound`, `ErrConnection`) to retry, roll back, or surface. The undo
window is a queued inverse op, not special-cased state.

A read-merge takes each account's inbox query, already sorted by `SentAt`, and
k-way merges in Go. Its cursor is a composite (account, UID, `SentAt`)
position, so it stays stable across incoming mail and across one account's
resync.

Everything in the cache is a rebuildable projection except the two things that
cost real bandwidth or cannot be refetched: cached bodies and attachments, and
OAuth refresh tokens. Schema, indices, and derived state can be dropped and
reconstructed. Migrations and cache resets may cross that line for everything
else and must not cross it for those two.

### 1.8 Acceptance scenarios

1. A config with N `[[account]]` blocks loads N accounts, each backed by its
   own SQLite file; no query reads across two account DBs.
2. The unified inbox lists messages from every account's Inbox ordered by
   `SentAt`; new mail arriving to any account appears in place without a manual
   refresh.
3. Triage on a unified-inbox row (archive, delete, flag, label) dispatches to
   that row's owning account and updates only that account's cache.
4. The unified-inbox cursor holds its position on a given message when new mail
   arrives and when one account resyncs.
5. A move changes a message's single folder; the message leaves its prior
   folder and appears in the destination.
6. On a JMAP account, applying a label persists as a keyword and survives a
   resync and a fetch from another client; removing it clears the keyword.
7. On a plain IMAP account whose `PERMANENTFLAGS` forbids custom keywords,
   `SupportsLabels` is false and the label surface is absent, stated in the UI.
8. A reply to a message delivered to an alias selects the identity matching the
   delivered-to address; a reply under a wildcard domain selects the
   alias-pattern identity without a per-alias config entry.
9. Composing fresh from the unified inbox starts from the primary account's
   first identity and lets the user switch identity before sending.
10. Gmail authorizes via loopback PKCE in the system browser using a
    user-supplied client ID; the headless path uses device-code where the
    provider supports it and loopback-over-SSH otherwise.
11. With the network down, the user reads cached mail, searches, and composes;
    a send queues to the outbox and drains on reconnect.
12. A mutation writes its optimistic cache change and its outbox row in one
    transaction; a forced failure rolls the change back through the conflict
    matrix, and undo fires the inverse op.
13. Search returns matches from the local FTS index with the network down and
    stays responsive under incremental query input.
14. Incremental sync fetches only deltas: a JMAP account via `Email/changes`,
    an IMAP CONDSTORE/QRESYNC server by `MODSEQ`, with a `UIDVALIDITY`
    fallback when QRESYNC is absent.
15. A cache reset preserves OAuth refresh tokens and forces no re-consent;
    cached bodies and attachments refetch on demand without data loss beyond
    bandwidth.
