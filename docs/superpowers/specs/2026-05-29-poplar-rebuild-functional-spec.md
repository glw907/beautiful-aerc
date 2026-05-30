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

---

## 2. Organization, threading, automation

Owner: Pass 2. Settles charter §6 decisions 3 to 5 and the label UX that
section 1's data model left open.

Section 1 fixed the data model. Folders are single-membership, labels are
server-backed and capability-gated, and the unified inbox is a read-side
merge. Section 2 builds organization, threading, and automation on top of
that model. Each automation feature gates at the backend boundary the way
`SupportsLabels` does, so a backend that cannot support a feature says so
instead of faking it. The field survey behind these decisions is
`docs/poplar/research/2026-05-29-mail-client-gap-analysis.md` §2.

### 2.1 Threading and the conversation model

The list groups by conversation. A thread is a first-class organizational
unit, and the actions that follow (triage, mute, snooze) target it.

Thread identity comes from the backend where it carries one (JMAP
`threadId`, Gmail `X-GM-THRID`) and from a `References` and `In-Reply-To`
walk over the cached headers otherwise. A non-threaded message is a thread
of one, consistent with section 1's `ThreadID == UID` rule.

A collapsed thread row stands for the whole conversation. It sorts by its
latest member's `SentAt`. It reads as unread when any member is unread.
Triage on a collapsed row applies to every message in the thread. Acting
inside an expanded thread targets the single message under the cursor.

Threaded is the default mode. A flat mode, one row per message with depth
markers, ships as well; its rendering and the toggle between modes belong
to Pass 3, which owns list UX and wireframes. Threads stay account-scoped,
because section 1 keeps storage account-partitioned. The same external
conversation seen through two accounts therefore shows as two rows in the
unified inbox.

### 2.2 Labels: operations and views

Section 1 settled the label data model. This section settles what the user
does with labels.

Apply and remove run through a multi-select picker that toggles each label
on or off for the selected messages or thread. Typing a name that does not
exist yet creates the label on the backend and applies it, so there is no
separate create-label step. Every change goes through the `Label(uids,
label, set)` mutator and the outbox, so it is optimistic locally and queued
for the backend like any other write.

A message carries zero or more labels, surfaced in both the list and the
reader; the exact chrome is a Pass 3 and Pass 4 wireframe decision.
Selecting a label opens a label-scoped view, which is a saved search over
that label. Label views and saved searches therefore share one mechanism,
described next.

Backend mapping follows section 1. JMAP labels are keywords set through
`Email/set`, IMAP labels are custom keywords set through `STORE`, and Gmail
user labels ride `X-GM-LABELS` over the IMAP connection. A backend whose
`PERMANENTFLAGS` forbid custom keywords reports `SupportsLabels = false`,
and the label surface is absent for that account.

### 2.3 Saved searches and virtual folders

One stored-query type backs saved searches, virtual folders, and
label-scoped views. A stored query has a name, a query expression, and a
scope. The query grammar is Pass 6's to define; this section commits only
to the stored-query shape and its behavior.

Stored queries persist in config and are also creatable at runtime by
saving the current search. A runtime save writes back through the same
`config.Render` round-trip section 1 relies on, so a saved search the user
creates in the UI survives a restart as a config entry. They appear as
virtual folders in the sidebar next to the classified folder tree.

A stored query is a projection, never a stored result set. Opening one runs
the query against the local FTS index, so it resolves offline and stays
responsive, per section 1's performance-by-locality and local-index-first
rules. It refreshes on open and on the change events that touch its scope.

Scope is one account, a set of folders within an account, or cross-account.
A cross-account stored query is the mechanism that later delivers unified
surfaces beyond the inbox, such as flagged-across-accounts. Section 1
scoped the always-present unified view to the inbox for v1; cross-account
stored queries are opt-in views the user defines, so they extend that
boundary without making every cross-account merge a default surface.

### 2.4 Server-side filters

Filtering rules run on the server so they apply while poplar is closed.
Poplar owns an abstract rule model and presents it as the client-visible
rule config. The model is an ordered list of rules. Each rule pairs a
condition over the message (sender, recipient, subject, an arbitrary
header, or size) with one or more actions, among them file into a folder,
set or clear a flag, set a label, discard, redirect, and stop processing.

The rule list compiles to Sieve (RFC 5228) using the extensions the server
advertises, among them `fileinto`, `imap4flags` (RFC 5232), and `mailbox`
with `:create` (RFC 5490). Poplar writes its compiled output into a managed
block fenced by sentinel comments and regenerates that block wholesale on
every change. Sieve written outside the fence by hand or by another tool is
preserved verbatim. A raw view shows the full active script read-only, so a
power user can audit exactly what runs.

Transport is capability-gated through `SupportsServerRules`. A JMAP account
that advertises `urn:ietf:params:jmap:sieve` (RFC 9661) manages the script
through `SieveScript/get`, `/set`, and `/validate`, and feature-detects
extensions through the capability's `sieveExtensions` list. An IMAP account
reaches ManageSieve (RFC 5804) on its own connection, separate from the
command and idle connections, and feature-detects through the `SIEVE`
capability line. A backend with neither path, including Gmail over IMAP and
plain IMAP without ManageSieve, reports `SupportsServerRules = false`, and
the rule surface states that server rules are unavailable for that account.
Gmail server filters need the Gmail REST API, which this client does not
use for mail in v1, so Gmail rule management is a tracked post-1.0
addition.

Rules are account-scoped. A change validates before it activates, through
`SieveScript/validate` on JMAP or `CHECKSCRIPT` on ManageSieve, and a
compile or validation failure surfaces to the user instead of dropping
silently.

### 2.5 Snooze and thread mute

Snooze and mute present one UX each and select their execution engine from
backend capability. Both features are always offered. The capability
decides how each one runs, and the feature itself is never withheld.

Snooze removes a thread or message from the inbox until a wake time, then
returns it. Where the backend advertises `SupportsServerSnooze`, poplar
uses the server engine, so the wake fires even when poplar is not running.
The JMAP path sets the `snoozed` property through the
`urn:ietf:params:jmap:mail:snooze` extension; a ManageSieve path uses the
Sieve `snooze` extension where the server lists it. Where no server engine
exists, poplar manages snooze itself. It moves the item to a managed
Snoozed folder, records the wake time in the cache, and returns due items
to the inbox on the next sync. The two engines differ in one user-visible
way, and the UI states it. A server snooze returns at the chosen time. A
client snooze returns at the chosen time, or at the next sync after it.
Both spec drafts (`snoozed` and Sieve `snooze`) are unratified, so the
server path is confirmed against the live account capability at build time.

Mute means future replies to a thread skip the inbox, with archive
semantics, so the thread stays reachable in Archive or All Mail. Mute has
no standard primitive, so its engine is also capability-tiered. Gmail uses
its native `Muted` label over `X-GM-LABELS`, which the Gmail server
enforces against future replies. A Sieve-capable backend gets a generated
mute rule, keyed on the thread, written into the same managed block section
2.4 defines; the rule files later thread members into Archive. A plain IMAP
account without Sieve falls back to a mute list in the cache, applied on
sync, so new members of a muted thread auto-archive the next time poplar
reconciles. Unmuting clears the label, the rule, or the list entry.

### 2.6 Triage across folders and the unified inbox

Triage on a unified-inbox row dispatches to that row's owning account, as
section 1 requires, and writes only that account's cache. On a collapsed
thread the action covers the thread; on an expanded message it covers the
one message.

Next-unread crosses folder boundaries. The key advances to the next unread
item in the current folder, then to the next folder that holds unread in
classified order, and in the unified context onward across accounts in
config order. The traversal is deterministic. Pass 3 owns the key binding
and the on-screen prompt; this section fixes the order.

Bulk action by criteria selects a set by a predicate instead of by manual
marking, then applies one action to the whole set. Built-in predicates
cover all unread, all from a sender, the whole thread, and everything
matching the current saved search. Each action queues per owning account
through the outbox, and it operates on the full matching set rather than
the visible page. Pass 3 owns the keyboard surface for selection and apply,
modeled on neomutt's tag-pattern-then-apply adapted to single keys; this
section fixes that bulk acts on a result set and dispatches per account.

### 2.7 Backend interface additions

Section 2 adds `SupportsServerRules`, `SupportsServerSnooze`, and
`SupportsNativeMute` to the capability set section 1 began with
`SupportsLabels`. Each gates a feature's server engine at the boundary, and
each has a defined fallback so the feature still works when the flag is
false.

The interface gains capability-gated operations for managing the compiled
rule set, for server snooze, and for the native-mute label path. An IMAP
backend that supports ManageSieve opens a third connection for it,
alongside the command and idle connections, because ManageSieve is a
separate service. The exact method signatures are build-plan work; this
section fixes the operations and their capability gates.

### 2.8 Acceptance scenarios

1. The message list groups by conversation by default; a thread shows as
   one row that sorts by its latest message and reads unread when any
   member is unread.
2. Triage on a collapsed thread (archive, delete, flag, label, snooze,
   mute) applies to every message in the thread; the same action inside an
   expanded thread applies to the single message under the cursor.
3. The same external conversation arriving on two accounts shows as two
   separate rows in the unified inbox, each dispatching to its own account.
4. Applying a label through the picker persists through the backend's
   keyword mechanism and survives a resync; typing a new name creates and
   applies the label without a separate step; removing it clears the
   keyword.
5. On a backend whose `PERMANENTFLAGS` forbid custom keywords,
   `SupportsLabels` is false and the label surface is absent, stated in the
   UI.
6. Selecting a label opens a label-scoped view that is the saved search
   over that label, using the same stored-query mechanism as a
   user-defined saved search.
7. Saving a search in the UI persists it across a restart as a config
   entry, and it reopens as a virtual folder in the sidebar.
8. A saved search resolves against the local FTS index with the network
   down and refreshes on open.
9. A cross-account saved search lists matching messages from every
   in-scope account, each row dispatching triage to its owning account.
10. Editing the rule config regenerates the managed Sieve block and leaves
    hand-written Sieve outside the fence unchanged; the raw view shows the
    full active script.
11. On a JMAP account advertising `urn:ietf:params:jmap:sieve`, a saved
    rule validates and activates through `SieveScript/set`; an invalid rule
    surfaces the validation error and does not activate.
12. A Gmail-over-IMAP account and a plain IMAP account without ManageSieve
    report `SupportsServerRules = false`, and the rule surface states that
    server rules are unavailable.
13. On a backend advertising server snooze, a snoozed thread leaves the
    inbox and returns at the wake time while poplar is closed; on a backend
    without it, the thread returns on the next sync after the wake time,
    and the UI states the difference.
14. Muting a thread routes its future replies out of the inbox with archive
    semantics: Gmail through its native muted label, a Sieve backend
    through a generated managed-block rule, a plain IMAP backend through a
    cache mute list applied on sync; unmuting restores inbox delivery.
15. Next-unread advances within the current folder, then into the next
    folder with unread in classified order, then across accounts in the
    unified context, deterministically.
16. A bulk action by criteria (all unread, all from a sender, the whole
    thread, or the current saved search) applies to the full matching set
    rather than the visible page, and queues per owning account through the
    outbox.
