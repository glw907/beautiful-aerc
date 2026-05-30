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

---

## 3. Reading, triage, navigation

Owner: Pass 3. Sections 1 and 2 settled the data model and the
organization, threading, and automation semantics. Section 3 designs the
surfaces that drive them: the keyboard model, the pane and sidebar layout,
the message list, the reader, and the triage and bulk surfaces. Rendering
fidelity belongs to Pass 4; this section fixes layout and affordances only.
The wireframes below are canonical and carry the reference layout for each
screen.

### 3.1 The keyboard model

Poplar binds modifier-free single keys. No user-facing action carries a
Ctrl, Alt, or Meta chord, and no action is a multi-key sequence, because
bubbletea delivers one key event per press. There is no `:` command mode.
Every action is a bare key, or a modal picker that a bare key opens.
`Ctrl-c` survives as an unadvertised terminal-kill alias on Quit.
Text-entry surfaces are the single exemption: compose, the search input,
and picker filters take the whole keyboard, and Pass 5 owns the compose
chords.

The account view and the viewer share one pane. The account view is the
sidebar and the message list together, and the viewer is the open reader.
There is no focus cycling in the account view. `j`/`k` always move the message list,
`J`/`K` always walk the sidebar, and every triage and reply key stays live.
Uppercase keys carry the folder jumps and the less-frequent verbs, clear of
the lowercase motions and triage, so the pairs never collide: `d` deletes
and `D` opens Drafts, `s` stars and `S` opens Sent, `m` moves and `M` mutes,
and `l` expands a sidebar node while `L` opens the label picker.

The full account-view key map:

| Key | Action |
|-----|--------|
| `j` / `k` | Message cursor down / up |
| `J` / `K` | Sidebar cursor down / up |
| `h` / `l` | Collapse / expand the sidebar node (`←`/`→` alias) |
| `g` / `G` | Message list top / bottom |
| `Space` / `F` | Fold the thread under the cursor / fold all threads |
| `Enter` | Open the message in the full reader |
| `Tab` / `Shift-Tab` | Next / previous unread (across folders, then accounts) |
| `n` / `N` | Next / previous search match (under an active search) |
| `[` / `]` | Previous / next account |
| `I` | Unified Inbox |
| `D` / `S` / `A` / `X` / `T` | Active account's Drafts / Sent / Archive / Junk / Trash |
| `d` / `a` / `s` / `.` | Delete / archive / star / toggle read |
| `L` / `z` / `M` | Label picker / snooze / mute |
| `m` | Move picker |
| `E` | Empty the current Disposal folder (confirm, no undo) |
| `r` / `R` / `f` / `c` | Reply / reply all / forward / compose |
| `t` / `P` | Threaded-flat toggle / preview-pane toggle |
| `v` / `V` | Manual visual select / select-by-criteria |
| `/` | Search shelf |
| `u` | Undo last triage (within the undo window) |
| `Q` / `!` | Outbox overlay / conflict overlay |
| `?` / `q` | Help / quit |

The viewer keeps `j`/`k`/`Space`/`b`/`g`/`G` for scroll, `n`/`N` for the
next and previous visible message, `Tab` and `1`-`9` for links, `@` for
attachments, `U` for unsubscribe, and `q` to close. Triage, reply, label,
snooze, and mute stay live in the viewer on the open message.

### 3.2 Pane model and responsive tiers

One pane is the default, true to the Pine lineage. The sidebar sits at the
left, the message list fills the rest, and opening a message replaces the
list with the reader in place. `q` returns to the list. Reading happens at
full width.

A widescreen tier adds an opt-in follower preview pane. When the terminal
is wide enough (around 130 columns, with the exact breakpoint left to the
build plan), `P` toggles a reader pane to the right of the list. The pane
re-renders the message under the list cursor, top-aligned, and never
scrolls on its own. `j`/`k` still move the list, so the keyboard model is
unchanged. `Enter` expands the message to the full-width reader and marks
it read. Cursoring the preview does not mark read, so scanning a folder
leaves unread state intact. The pane renders through the reader's own code
at its narrower width, so the Pass 4 rendering contract covers both
surfaces without a second path. The toggle is sticky for the session. A
pane open past the tier auto-collapses when the terminal narrows below it
and returns when there is room again.

Responsive tiers carry the legacy three-tier ladder and add the widescreen
pane tier above it. Sidebar width, the sender and date columns, the flag
column, icon mode, and the label chips all derive from the terminal width
through one layout computation. Spartan is the narrow floor: it trims the
sidebar, drops the date and flag columns, and hides label chips. Flags and
a compact date return in the intermediate tier. Every chrome element turns
on at the full tier. Widescreen is the full tier plus the offer of the
preview pane. Spartan is also the minimum supported width, and below it the
layout is undefined.

### 3.3 The sidebar

The sidebar follows the Thunderbird and K-9 shape that section 1 fixed. A
Unified Inbox row pins to the top. Below it, each account is a collapsible
section over its own classified folder tree. A Saved Searches group sits at
the bottom, where label views and user saved searches share one list,
since section 2 made a label view a saved search.

```
 sidebar -- Full tier
────────────────────────────────
 ★  Unified Inbox          5        pinned cross-account merge
 ▾  geoff@907.life         3        account header, expanded
    󰇰  Inbox               3
    󰏫  Drafts
    󰑚  Sent
    󰀼  Archive
    󰍷  Junk              12
    󰩺  Trash
    󰡡  Lists/golang
 ▸  work@company.com       2        collapsed, badge sums unread
 ── Saved searches ──
    󰈻  Flagged
    󰓹  golang                       a label view is a saved search
```

`J`/`K` walk the whole column as one vertical list: the Unified Inbox row,
each account header, that account's folders, and the saved searches. The
active account is wherever the cursor sits. `h` and `l` collapse and expand
an account section and the custom-folder subtrees within it, with `←`/`→`
as aliases. A collapsed account shows the sum of its unread as a badge.

Folder jumps target the active account. `D`/`S`/`A`/`X`/`T` open that
account's Drafts, Sent, Archive, Junk, and Trash. `I` opens the Unified
Inbox, which is the primary triage surface, so a specific account's own
Inbox is reached by walking into its section. `[` and `]` step to the
previous and next account, cycling through the Unified Inbox and each
account in turn and landing the cursor on that scope's Inbox, so a
multi-account user moves to "work in this account now" in one keystroke.
`Tab` and `Shift-Tab` stay reserved for next and previous unread (3.1) and
do not switch accounts.

`[ui] unified-inbox` defaults to true and turns the unified view off when
set false. With the unified view off, the pinned row is absent and `I`
opens the active account's Inbox. A configuration with one account hides
the unified row and the account-header chrome, so the sidebar reads exactly
like a single classified tree.

### 3.4 The message list

The list groups by conversation by default, per section 2.1. Each row
carries a flag column, a sender, a subject, optional label chips, and a
date. Read state shows through brightness: an unread sender and subject
render bright, a read row dims, and hue stays reserved for the cursor and
the unread-and-flagged case. The flag column glyphs mark unread, replied,
and flagged, as in the legacy list.

```
 ▍󰇮 Alice Johnson    Re: Q2 launch          ·golang      10:32 AM
 ▍  Bob Smith        Weekly standup notes                  9:15 AM
 ▎󰑚 Carol White      Re: Budget review                  Yesterday
 ▎󰈻 Eve Martinez [3] Re: Server migration    ·ops          Apr 06
```

Cross-account views carry an account color marker. In the Unified Inbox and
in any cross-account saved search, a one-cell marker at the row's left
edge takes each account's stable accent color. The glyph is constant and
the color carries the identity. A single-account folder omits the marker
column, so a per-account folder reads like the legacy list.

Labels render as compact chips between the subject and the date, dim and
in the label's color. Several labels overflow to a `+N` count. The Spartan
tier drops the chips, and the reader shows the full set.

`t` toggles the current folder between threaded and flat display. Threaded
collapses a conversation to one row that sorts by its latest member and
carries an `[N]` count badge; expanding shows the members with box-drawing
prefixes. Flat lists every message as its own row in the folder's date
order, with a dim reply marker as a light depth cue and no folding. The
default and any per-folder override come from the `[ui.folders.<name>]
threading` setting that section 1 carries; `t` is a session toggle over
that default. In threaded display, `Space` folds and unfolds the thread
under the cursor, and `F` folds or unfolds every thread; `Space` instead
toggles the row's selection while a visual selection is active.

A saved search, a label view, and the search shelf all render in the same
results mode: a flat list of matching messages, each tagged with its
account marker when the scope crosses accounts, with no folder-tree
context. Triage on a result dispatches to that row's owning account, as
section 2 requires.

### 3.5 The reader

The full reader opens in place of the list, with the sidebar still drawn.

```
 ▍ geoff@907.life
  From:     Alice Johnson <alice@example.com>
  To:       Geoff Wright <geoff@907.life>
  Date:     Thu, 10 Apr 2026 10:32 AM
  Subject:  Re: Q2 launch
  Labels:   ·golang  ·priority           󰒲 snoozed → Mon 9:00 AM
  ──────────────────────────────────────────────────────────────

  Hey Geoff,

  Just wanted to follow up on the Q2 launch timeline.

  ## Key changes
  - Beta release moved to April 15 [^1]

  § 1. invite.ics (2.1 KB)   § 2. agenda.pdf (88 KB)

  [1]: https://example.com/q2-plan
```

The header shows an account chip, the standard From, To, Date, and Subject
rows, and a Labels row. In cross-account contexts the account chip carries
the account color marker and name, so the reader states which account owns
the message and which identity a reply defaults to. A Labels row lists the
message's labels as chips, and its right edge shows snooze or mute state
when set, such as `󰒲 snoozed → Mon 9:00 AM` or `󰂛 muted`. That row is
absent when the message has no labels and no snooze or mute state.

The body region, the footnote link handling, the invite block, the
attachment chip row, and unsubscribe carry forward from the legacy reader
unchanged, since Pass 4 owns their fidelity. Links harvest into a footnote
list with inline `[^N]` markers and a `[N]:` block, launchable by `1`-`9`
or the `Tab` picker. Attachments open through the `@` picker. `U` runs
List-Unsubscribe when the header is present.

A contact affordance hangs off the sender line, reserved on `i`. The
contact model is Pass 7, so this section reserves the key and leaves the
popover behavior to that pass.

### 3.6 Triage, bulk, undo, and traversal

Single-key triage acts on the cursor row, or on the marked set when a
selection is live. The scope rule from section 2 holds: a collapsed thread
root covers the whole thread, an expanded message covers the one message.
`d`, `a`, `s`, and `.` delete, archive, star, and toggle read. `L`, `z`,
and `M` label, snooze, and mute. Each is optimistic through the cache and
queued through the outbox.

`L` opens the label picker, a multi-select toggle with a filter input.
Checking a label applies it and unchecking removes it across the selection.
Typing a name that does not exist offers a create row, so a new label is
created on the backend and applied without a separate step. `Enter` commits
every toggle through the `Label` mutator.

```
╭─ Labels ─────────────────────────╮
│  > gol▏                          │
│  ☑ golang                        │
│  ☐ priority                      │
│  ☐ ops                           │
│  + create "gol"                  │
│  ␣ toggle  ⏎ apply  Esc cancel   │
╰──────────────────────────────────╯
```

`z` opens the snooze picker: preset wake times plus a custom row that
parses a typed time. On a client-managed account, one whose backend does
not advertise server snooze, the picker states that the item returns at the
next sync after the wake time, the difference section 2.5 requires the UI
to surface.

```
╭─ Snooze until ───────────────────╮
│  ┃ Later today       4:00 PM     │
│    Tomorrow          8:00 AM     │
│    This weekend      Sat 8:00 AM │
│    Next week         Mon 8:00 AM │
│    Custom…                       │
│  returns at sync after wake      │   client-managed only
│  ⏎ snooze  Esc cancel            │
╰──────────────────────────────────╯
```

`M` mutes with no overlay. It fires the capability-tiered engine from
section 2.5 and drops a toast.

`E` empties the current Disposal folder, Junk or Trash. It opens the
confirm modal, since the operation runs `Destroy` and cannot be undone. On
confirm the folder's messages are permanently deleted and the toast omits
the undo hint.

`v` enters manual visual select, where `Space` marks rows. `V` opens the
select-by-criteria menu, the tag-pattern surface adapted to single keys.
Choosing a predicate marks the matching set across the full results, not
the visible page, and closes the menu. The next triage key then applies to
the whole marked set and clears the marks, the same marks-survive-then-
dispatch rule manual selection uses.

```
╭─ Select by ──────────────────────╮
│  u  all unread                   │
│  f  from Alice Johnson           │   sender of the cursor row
│  t  this thread                  │
│  s  current search results       │   enabled under an active search
│  Esc  cancel                     │
╰──────────────────────────────────╯
```

The undo window is the legacy chrome row above the status bar. A reversible
triage lands a toast with a `[u undo]` hint and a countdown, and `u` fires
the inverse op within the window. Label, snooze, and mute are reversible,
so each lands the undo hint; the inverse is unlabel, unsnooze, and unmute.
Permanent-delete operations suppress the hint, since the primitive has no
inverse.

```
───┴───── 󰒲 Snoozed until Mon 8:00 AM   [u undo · 6s] ────╯
───┴───── 󰓹 +2 labels: golang, ops      [u undo · 6s] ────╯
───┴───── 󰂛 Muted thread                 [u undo · 6s] ────╯
```

`Tab` and `Shift-Tab` advance to the next and previous unread. The
traversal follows the deterministic order section 2.6 fixed: the next
unread in the current folder, then the next folder holding unread in
classified order, then onward across accounts in configuration order in the
unified context.

### 3.7 Overlays

App owns the modal overlays and composites each over the current frame.
Only one overlay is active at a time, resolved through a fixed cascade. The
overlays are the help popover, the label picker, the snooze picker, the
select-by-criteria menu, the move picker, the confirm modal, the link
picker, the attachment picker, the outbox overlay, and the conflict
overlay.

An overlay composites over a dimmed underlay. The dim recedes the frame
beneath so attention lands on the overlay, while the underlay stays legible
as context, so the user keeps their place. In the terminal the dim blends
the underlay's foreground toward the background to lower its contrast,
rather than blanking it. The exact blend follows the styling map and the
build plan; the principle is a focus scrim, neither so faint that the
overlay fails to stand out nor so heavy that the context is lost. The
preview pane and the inline compose surface are not overlays and do not
dim.

### 3.8 Acceptance scenarios

1. In the account view, `j`/`k` move the message cursor and `J`/`K` move
   the sidebar cursor; triage and reply keys act with no focus change and
   no focus-cycling step.
2. Every bound action is a single bare key or opens a modal picker; no
   user-facing action requires a modifier or a key sequence, and there is
   no `:` command line.
3. `Tab` and `Shift-Tab` advance to the next and previous unread, crossing
   into the next folder with unread in classified order and then across
   accounts in configuration order, per section 2.6.
4. `[` and `]` switch the active account to the previous and next account
   and land the cursor on that account's Inbox; with the unified view on,
   the cycle includes the Unified Inbox.
5. `I` opens the Unified Inbox, and `D`/`S`/`A`/`X`/`T` open the active
   account's Drafts, Sent, Archive, Junk, and Trash.
6. With `[ui] unified-inbox = false`, the Unified Inbox row is absent and
   `I` opens the active account's Inbox; a single-account configuration
   also hides the account-header chrome.
7. The sidebar shows a pinned Unified Inbox, one collapsible section per
   account over its classified tree, and a Saved Searches group; `h`/`l`
   (and the `←`/`→` aliases) collapse and expand a section, and a collapsed
   account shows summed unread.
8. In the Unified Inbox and any cross-account saved search, each row
   carries its account's color marker; a single-account folder omits the
   marker column.
9. `t` toggles the current folder between threaded and flat; threaded
   collapses a conversation to one `[N]`-badged row, and flat lists every
   message in date order with a depth cue and no folding.
10. A message's labels render as chips in the list row and in full on the
    reader's Labels row; list chips overflow to `+N` and drop in the
    Spartan tier.
11. Past the widescreen breakpoint, `P` opens a follower preview pane;
    `j`/`k` still move the list, the pane re-renders the cursored message
    without scrolling, cursoring does not mark read, and `Enter` opens the
    full reader and marks read.
12. Below the widescreen breakpoint the preview pane is unavailable; the
    reader opens in place of the list, and `q` returns to the list.
13. `L` opens the label picker, which toggles labels across the selection
    and creates a typed name that does not yet exist, committing through
    the `Label` mutator and the outbox.
14. `V` opens the select-by-criteria menu; a chosen predicate (all unread,
    from the cursor's sender, this thread, or the current search) marks the
    full matching set, and the next triage key applies to the whole set and
    clears the marks.
15. `z` opens the snooze picker and, on a client-managed account, states
    that the item returns at the next sync after the wake time; `M` mutes
    with no overlay; both land an undo toast whose inverse op restores the
    prior state within the window.
16. A modal overlay composites over a dimmed underlay that stays legible as
    context, and the overlay cascade keeps one overlay active at a time;
    the preview pane and inline compose do not dim.
17. In threaded display, `Space` folds and unfolds the thread under the
    cursor and `F` folds or unfolds every thread; while a visual selection
    is active, `Space` toggles the cursor row's selection instead.
18. With a search active in the account view, `n` and `N` advance to the
    next and previous match; `g` and `G` jump the message list to its top
    and bottom.
19. `E` on a Disposal folder opens the confirm modal and, on confirm,
    permanently empties the folder through `Destroy` with no undo hint.

---

## 4. Message rendering and display

Owner: Pass 4. Settles charter §6 decision 6 and the rendering program in
charter §7. Fills the reader body region that section 3.5 framed but left to
this pass.

Section 3 fixed the reader surface, including the header, the Labels row, the
body region, and the link and attachment affordances. Section 4 owns what
fills the body. The default render is poplar's own cleaned markdown, derived
from whichever message part carries the most structure, rather than a verbatim
dump of a MIME part. Its pipeline is built to be improved by Claude against a
corpus, so this section specs the renderer with its contract and gold standard,
the fidelity targets, the developer-facing features, and the offline tool and
eval loop that drive the renderer toward solid. The field survey behind these decisions is
`docs/poplar/research/2026-05-29-mail-client-gap-analysis.md` §4, which found
poplar's HTML-to-markdown-to-blocks reduction already ahead of the
w3m-dump terminal field. Pass 4 raises fidelity and adds developer features on
that base.

### 4.1 The rendering pipeline

The pipeline runs in three stages. It parses the chosen message part into a
normalized block tree, cleans the tree and infers structure the source only
implied, then renders the tree to themed terminal lines. The block tree is the
single intermediate. Two outputs derive from the same tree. The themed terminal
lines are the runtime render the reader shows. A deterministic markdown
serialization of the tree is the audit artifact the contract and the eval tool
judge against, so what Claude inspects is exactly what the pipeline produced.

The reader body wraps to the reader's content width through the
`ansix.Measurer`, so the render is pane-relative. There is no fixed column cap.

### 4.2 Source selection and render modes

`[ui] body-render` takes `"markdown"`, `"html"`, or `"plain"` and defaults to
`"markdown"`. A reader-scoped key cycles the active message through the three
modes for a one-off look; its binding is reconciled against the locked section
3 keymap in the build phase.

- `"markdown"` runs the full cleaning and structural-inference pipeline and
  produces poplar's signature render. The pipeline selects the source part
  itself, preferring `text/html` when it carries real structure and falling
  back to `text/plain`.
- `"html"` forces the HTML part as the source and renders its structure with
  the inference layer relaxed, a render closer to the source with less
  normalization.
- `"plain"` shows the `text/plain` part with reflow only and no inference.

The raw source is always reachable for inspection regardless of mode.

### 4.3 The rendering contract and the gold standard

The contract is a versioned normative document,
`docs/poplar/rendering-contract.md`. It is the standard the eval tool judges
every render against, and the document a Claude session loads before a
rendering pass so two sessions judge the same render the same way. It carries
four load-bearing parts.

- **Principles.** The design philosophy the rules rest on, including density is
  signal, structure is inferred rather than copied, output is for a narrow
  terminal column, and consistency beats cleverness. When a case is ambiguous,
  principles decide and the conflicting rule is flagged for revision.
- **Structural-inference rules.** Heuristics that derive markdown structure
  from presentation when the source lacks semantic tags, such as a bold
  standalone short line becoming a heading, a hard-wrapped plain-text source
  reflowing to logical paragraphs, marker runs becoming a list, and a `-- `
  line opening a signature block.
- **Syntactic rules.** RFC 2119 MUST and SHOULD checks on the rendered output,
  covering entity decoding, whitespace hygiene, heading and blockquote and list
  and code formatting, and link handling. Each rule names an observable failure
  mode so a bad render points at a specific violation.
- **Density signals.** Smoke-detector metrics computed from the output,
  including lines per paragraph, characters per non-blank line, blank-line
  ratio, orphan rate, and heading density. A failing signal flags a render for
  inspection rather than failing it outright.

The eval loop's judge rationales are how this document grows. A new failure
class that the loop confirms becomes a new rule, so the contract sharpens with
each pass.

Alongside the rules, a gold standard fixes what good output looks like in the
concrete. It is a small, hand-curated set of exemplar renders, each pairing a
representative message shape (a newsletter, a threaded reply, a mailed patch, a
calendar invite) with the markdown render a human has blessed as excellent.
Excellent is a quality bar measured against the source. The render must carry
the original message's meaning, structure, and emphasis into clean markdown, so
a render that is valid markdown yet loses what the message conveyed still fails.
The contract rules generalize from these exemplars, and the judge calibrates
against them, so scoring measures a render against shown-good output rather than
abstract checks alone. When a message shape the gold standard does not yet cover appears,
a human blesses a new exemplar and the contract rules extend to match.

The tunable data the rules reference lives in one hand-edited Go file,
`rules.go`, holding the entity table, the tracking-parameter list, the heading
and density thresholds, the list-marker sets, and the patch and diff
signatures. Centralizing it gives a tuning fix one obvious home and a reviewable
diff. The structural logic stays hand-written Go. Nothing here is
code-generated; "compiled into poplar" means the ruleset is Go in the binary,
edited like any other source.

### 4.4 Fidelity targets

The gap analysis flagged two debts this pass closes alongside the contract
rules.

- **Quote folding ships.** A quoted block, identified by `>` depth and an
  `On … wrote:` attribution, collapses by default to a one-line stub with a
  depth cue, and a single key expands and re-collapses the block under the
  cursor. The terminal field folds quotes and poplar does not today.
- **Tables split by intent.** A table carrying tabular data renders as a GFM
  pipe table. A table used only as layout scaffolding flattens to sequential
  blocks. The structural-inference rule distinguishes the two.

Entity decoding, tracking-parameter stripping (`utm_*`, `fbclid`, `gclid`, and
the rest of the list in `rules.go`), and footnote link extraction into the
section 3.5 list are contract MUSTs the renderer always applies.

### 4.5 Remote content, inline images, and terminal graphics

Remote resources never load on render. That is the privacy floor and it holds
in every mode. A blocked remote image leaves an alt-text placeholder, and a
per-message action loads remote resources only when the user asks.

Inline image rendering is opt-in and capability-detected. At startup poplar
probes the terminal for a graphics protocol (kitty, iTerm2, or sixel), the same
shape as icon-mode resolution in section 1, and `[ui] inline-images` gates the
feature. When the feature is on and the terminal supports a protocol, the reader
renders embedded `multipart/related` (CID) images and image attachments inline,
and renders remote images only after an explicit load. When the feature is off
or the terminal lacks a protocol, every image is an alt-text placeholder and the
layout is unchanged.

### 4.6 Developer-facing features

The audience is coders, so the body renders three developer cases as
first-class output.

- **Syntax highlighting.** Fenced code blocks render through chroma, themed to
  the active palette. The language comes from the fence tag when the source
  declared one, with light autodetection otherwise.
- **Patch and diff rendering.** A body that is a unified diff or `git
  format-patch` output, or a `text/x-patch` or `text/x-diff` part, renders with
  add, remove, and hunk-header coloring and a file-header treatment. A mailed
  patch reads natively.
- **Richer links.** The footnote model from section 3.5 stays, and each
  harvested link gains a copy action alongside the launch already specced
  there.

### 4.7 The golden corpus

The corpus has two tiers. Public sets commit to the repo directly as golden
inputs, including Apache SpamAssassin and TREC for breadth, public-inbox and
lore for patches and threading, and Enron for scale. The real-world tier is the
user's own Fastmail mail, read by the eval tool's own JMAP pull rather than
through a running poplar backend. A message from that tier enters the repo as a
committed fixture only after a minimize step strips it to the smallest input
that still reproduces the case and a human reviews and approves the scrubbed
result. Raw captures stay outside the repo. The corpus and the tool are
dev-only and reach no runtime path.

Of these golden files, the small quality-defining subset blessed as the target
is the gold standard from 4.3. The rest serve as regression baselines, locking
the current accepted render of each corpus message so a change that alters them
is caught.

### 4.8 The rendering tool and the eval loop

The tool is a dev-build-tagged CLI in the poplar repo, built around the renderer
package and decoupled from the app. It talks to the renderer and the corpus on
disk, never to a live backend, the cache, or the UI tree. That isolation is
what makes Claude's inner loop on rendering fast and reproducible without
standing up the whole client. The tool renders any raw email file
deterministically, emits the themed output and the audit markdown side by side,
diffs a current render against a locked snapshot, scores a render against the
contract, and lists and addresses corpus entries so Claude sweeps many cases in
one pass.

The improve loop runs as a skill. It loads the contract, lists the corpus,
renders and judges each case against the contract, and clusters the failures by
root cause. It fixes the pipeline logic or the `rules.go` data, diffs the
affected renders to confirm no regression, runs the gate, then locks the golden
and scrubs the fixture. The judge's rationale for a confirmed failure class
feeds back into the contract document. Locked golden renders and the public
corpus run under `make check` as a permanent regression gate, so an unreviewed
rendering change fails the build.

The loop also runs unattended. A batch mode renders the whole corpus, judges
each render with Claude, clusters the failures, applies the ruleset and pipeline
fixes that clear the gate, and commits the result, so the renderer can be driven
toward the gold standard on a schedule rather than only by hand. The same tool
emits a human-readable evolution report, written with Claude, that traces how
the ruleset changed over time and names the failure classes that drove each
change. The report is the narrative record behind the rules, and the contract
document stays the normative source.

### 4.9 Deferred: runtime LLM HTML cleaning

A runtime feature that sends one message's HTML to an LLM for cleaning is named
here and deferred. It is an opt-in, never the default path, and its boundary is
cost, latency, privacy, and offline operation. Pass 4 does not build it. The
default rendering path is the offline pipeline in 4.1.

### 4.10 Acceptance scenarios

1. A message carrying both `text/plain` and `text/html` renders by default as
   poplar's cleaned markdown, derived from the structure-richest part, not as a
   verbatim MIME part.
2. `[ui] body-render = "html"` renders from the HTML part with inference
   relaxed, `"plain"` shows the `text/plain` part reflowed only, and the default
   `"markdown"` runs the full pipeline; the reader's render-mode key cycles the
   active message through the three modes.
3. Body text reflows to the reader's content width, so a widescreen reader wraps
   wider than a standard one, and no fixed column cap applies.
4. A quoted block collapses by default to a one-line stub with a depth cue, and
   a single key expands and re-collapses the block under the cursor.
5. A table carrying tabular data renders as a GFM pipe table, and a table used
   only for layout flattens to sequential blocks.
6. Rendered output carries no raw HTML entities and no tracking parameters in
   link hrefs, and it harvests links into the section 3.5 footnote list.
7. Remote images never load on render; each shows an alt-text placeholder, and a
   per-message action loads remote resources on request.
8. With `[ui] inline-images` on and a supporting terminal, embedded CID images
   and image attachments render inline; with the feature off or the terminal
   unsupported, every image is an alt-text placeholder and the layout is
   unchanged.
9. A fenced code block renders with chroma syntax highlighting themed to the
   palette, using the fence language tag when present and autodetection
   otherwise.
10. A body that is a unified diff or `git format-patch` output renders with add,
    remove, and hunk-header coloring and a file-header treatment.
11. Each harvested link offers a copy action alongside the launch action from
    section 3.5.
12. The rendering tool renders any raw email file deterministically and emits
    both the themed output and the audit markdown, and the same input always
    produces the same output.
13. A public-set fixture commits to the repo directly, and a fixture derived
    from real Fastmail mail enters the repo only after a minimize step and a
    human scrub gate.
14. Locked golden renders and the public corpus run under `make check` and fail
    the build on an unreviewed rendering change.
15. The judge scores a render for quality relative to its source, against the
    gold-standard exemplars together with the contract; a render that is valid
    markdown but loses the source's meaning fails, and a newly blessed exemplar
    extends the standard to a message shape it did not yet cover.
16. An unattended batch run drives the full corpus loop, applying and committing
    the ruleset and pipeline fixes that clear the gate.
17. A human-readable report traces how the ruleset evolved, naming the failure
    classes that drove each change.
18. The runtime LLM-clean-HTML path is absent in Pass 4, and the spec names it
    as a deferred opt-in with its boundary stated.

---

## 5. Compose and sending

Compose is the outbound surface. Catkin hosts the body, the surface gathers
the headers and attachments, and the outbox owns dispatch, the send delay,
and scheduling. The decisions here derive from the field and current
practice; legacy poplar is evidence, not the baseline. The audience sends to
mailing lists and patch queues as well as to people, so the surface treats
clean text/plain output as a first-class case rather than an afterthought.

### 5.1 The compose surface

Compose is a modeless full-pane surface. It opens in place of the list or
the reader with the sidebar still drawn, the placement the full reader uses.
It is not an overlay and does not dim an underlay (§3.7). `c` opens a blank
compose, and `r`, `R`, and `f` open it seeded as a reply, reply-all, or
forward.

The surface is a text-entry context, so it takes the whole keyboard and the
modifier-free account-view rule lifts here (§3.1). Body editing is modeless
in the Pico tradition. Every keystroke inserts, and editing runs through
chords rather than an editor mode. Vim modal editing inside catkin is named
and deferred; the rebuild ships the modeless editor. A persistent command
row along the foot lists the message-level chords.

```
 Compose ─ geoff@907.life ▾                              text+html
  To:    alice@example.com, Bob Lee <bob@example.com>▏
  Cc:
  Bcc:
  Subj:  Re: Q2 launch
  ── signature: Work ▾ ─────────────────────────────────────────
  Hey Alice,

  Thanks for the update. A few notes below.

  > Just wanted to follow up on the Q2 launch timeline.

  ▏

  § draft.diff (4 KB)   § notes.md (1 KB)
  ──────────────────────────────────────────────────────────────
  ^X send  ^O draft  ^A attach  ^J snippet  ^T tidy  Esc cancel
```

Poplar owns the message-level commands and catkin owns the body. `Ctrl-X`
sends, `Ctrl-O` saves a draft and closes, `Ctrl-A` attaches a file through
the attachment picker, `Ctrl-J` inserts a snippet, and `Esc` leaves the
surface, postponing a dirty draft to Drafts and offering discard for an empty
or unwanted one (§5.6). Tidy is catkin's own command on
`Ctrl-T` (§5.9). The surface claims its chords before delegating the rest to
catkin, and catkin leaves those keys unbound, so the boundary holds for the
standalone editor too. Its header carries From, To, Cc, Bcc, and Subject.
The From field shows the active identity with a picker, the top-right shows
the send mode (`text+html` or `text`), and a signature row divides the
headers from the body. Cc and Bcc collapse to a hint line until focused or
non-empty.

### 5.2 Address entry and autocomplete

To, Cc, and Bcc take comma-separated addresses. As the user types a trailing
fragment of two or more characters, a dropdown offers recency-decayed
suggestions from the cache's address history, ranked by recency and
frequency, a few at a time. `Tab` or `Enter` accepts the highlighted entry
and rewrites it as `Name <email>, `. The suggestion seam is the same one
Pass 7 contacts feed, so CardDAV entries join the dropdown without a second
code path. It renders only on the focused field and only while the trailing
fragment is open.

### 5.3 Reply, reply-all, and forward

`r` seeds a reply to the sender. `R` seeds reply-all with the original From
in To and the original To and Cc in Cc, dropping the user's own identities
and any duplicates. `f` seeds a forward and carries the parent's attachments
into the attachment row, each removable.

The parent body quotes with depth-preserving `>` markers, so a reply to a
reply deepens to `>>`. The cursor lands above the quote, the top-post
default, with the quote present and trimmable so list-style trimming and
interleaving stay natural. Threading headers follow RFC 5322. `In-Reply-To`
is the parent's Message-Id, and `References` extends the parent's References
with the parent's Message-Id. The reply subject takes a single `Re: ` prefix
and a forward takes `Fwd: `, neither doubled when already present. Identity
auto-selection runs per §1.5, matching the delivered-to address with
alias-pattern awareness, and seeds the chosen identity's default signature.

### 5.4 Identities and signatures

The From field picks the active identity. For a reply or forward the
candidate set is the owning account's identities, pre-selected by §1.5. For a
fresh unified compose it starts from the primary account's first identity and
allows an account switch from the same picker, so the merged view needs no
separate account modal. Each identity carries an ordered signature list, and
the first signature is the identity's default. A signature picker in the
surface switches among them, and each signature carries the RFC 3676 `-- `
separator. Switching identity re-seeds the new identity's default signature
unless the user has edited the signature text, which is preserved.

### 5.5 MIME assembly and the text/HTML contract

On send the draft assembles to MIME. The default is multipart/alternative
with a text/plain part and a text/html part. Its text/plain half is the
markdown source, lightly reflowed. Its text/html half is the goldmark render
of that same markdown. That render uses the Pass 4 reader's goldmark
configuration, so the structure a recipient sees in an HTML client matches
the structure poplar renders on receive, and a message that round-trips
through poplar reads the same on both ends. With attachments present the
alternative nests inside a multipart/mixed wrapper.

Text-only mode drops the HTML part and emits a single text/plain body. An
identity carries a `text-only` flag, and a per-message toggle in the
surface's top-right mode indicator overrides it either way. A list identity
sends clean text by default, and the per-message toggle handles a one-off
patch from an otherwise rich identity. The markdown source is the wire text
in this mode, so fenced code blocks and unified diffs pass through unmangled,
which is the reason a coder's client needs the toggle at all.

### 5.6 Drafts

The surface autosaves to the cache on a debounce and on leave. Leaving with
`Esc` postpones the draft to Drafts rather than discarding it, with an
explicit discard path for an empty or unwanted draft. A draft syncs to the
server Drafts folder so another client sees it. `D` opens Drafts (§3.1), and
selecting a draft re-opens the compose surface seeded from it, restoring
identity, recipients, subject, body, attachments, and send mode. On send the
outbox row carries the `draft_id` FK from the cache contract (§1.7), and the
drainer deletes the linked draft in the same transaction that records the
send, so a sent draft does not linger in the folder.

### 5.7 Outbox: send delay, send-later, and undo-send

Send never dispatches inline. `Ctrl-X` queues an outbox row and closes the
surface, optimistic through the cache. The row's `scheduled_for` defaults to
the current time plus the send-delay window (`[ui] send-delay`, a few seconds
by default, zero to disable). During the window the undo chrome row shows a
countdown and `u` cancels the send, returning the message to Drafts; the
drainer dispatches the row only once `scheduled_for` has passed and the
backend is reachable.

Send-later sets `scheduled_for` to a chosen future time through a time picker
that shares the snooze picker's time parsing. A scheduled message sits in the
outbox as a visible pending row in the outbox overlay (`Q`), where it can be
canceled back to a draft or rescheduled. Undo-send and send-later are one
mechanism. A future `scheduled_for` with a cancel-to-draft inverse covers
both, so neither is special-cased state. Dispatch failures route through the
§1.7 conflict matrix by typed sentinel, retrying on `ErrConnection` and
surfacing on `ErrAuth`, without losing the queued message.

### 5.8 Templates, snippets, and the attachment reminder

Snippets are named reusable bodies stored in config, each a `[[snippet]]`
block with a name and text. `Ctrl-J` opens a snippet picker filterable by
name, and choosing one inserts its text at the cursor. A single `{{cursor}}`
placeholder positions the cursor after insertion, and no other substitution
applies, which keeps the feature small per the charter. A template is just a
snippet used as a whole-message starting body, so templates and snippets
share one mechanism at different scales.

The attachment reminder scans the assembled body at send for
attachment-intent keywords, the default set covering "attached",
"attachment", and "enclosed". If the body signals an attachment and none is
attached, send pauses on a confirm modal offering attach-now or send-anyway.
The scan runs at send time, so it catches a forgotten attachment after the
body is final. Its keyword set is overridable for a non-English composing
language.

### 5.9 AI prose tidy

Tidy is a catkin feature, user-invoked, and never on the send path. `Ctrl-T`
runs it over the whole body or the current selection. The result shows as an
in-place diff with insertions and deletions marked inline, and the user
accepts or rejects the change as a whole or steps through it hunk by hunk.
Tidy never mutates the body without an explicit accept, and it never fires on
send or on save. It corrects grammar, clarity, and tone while preserving
meaning, and it leaves fenced code blocks and quoted text untouched. The
model and the prompt are catkin's concern, and poplar threads the configured
provider through. Tidy is offline-optional. With no provider configured the
key is inert and says so.

### 5.10 Acceptance scenarios

1. `c` opens a blank compose as a non-overlay full pane with the sidebar
   still drawn, and the surface does not dim an underlay.
2. Body editing is modeless, every keystroke inserts, and the message-level
   commands are chords shown in a persistent command row; no body edit
   requires a mode switch.
3. Typing a two-character trailing fragment in To, Cc, or Bcc shows
   recency-decayed address suggestions from the cache, and `Tab` accepts the
   highlighted entry rewritten as `Name <email>, `.
4. `r` seeds a reply to the sender, `R` seeds reply-all with the original
   recipients minus the user's own identities and with no duplicates, and `f`
   seeds a forward carrying the parent's attachments.
5. A reply sets `In-Reply-To` to the parent's Message-Id and extends
   `References` per RFC 5322, quotes the parent body with depth-preserving `>`
   markers, and lands the cursor above the quote.
6. A reply to a message delivered to an alias selects the alias-matching
   identity with alias-pattern awareness and seeds that identity's default
   signature, and both the identity and the signature are switchable in the
   header before send.
7. By default, send assembles multipart/alternative with a text/plain
   markdown source and a text/html goldmark render matching the Pass 4
   reader's render of the same markdown.
8. With attachments, the message is multipart/mixed wrapping the alternative,
   and each attachment appears in the surface's attachment row and is
   removable.
9. With the identity's `text-only` flag set or the per-message text-only
   toggle on, the message is a single text/plain part with the markdown
   source as the wire text and no HTML part, and fenced code and diffs pass
   through unaltered.
10. Leaving compose with `Esc` postpones the draft to Drafts and syncs it to
    the server Drafts folder, and an empty draft offers an explicit discard.
11. Reopening a draft from `D` restores identity, recipients, subject, body,
    attachments, and send mode, and sending it deletes the linked draft in
    the same transaction that records the send.
12. `Ctrl-X` queues the message to the outbox with `scheduled_for` at the
    current time plus `[ui] send-delay` and closes the surface, and the
    message is not dispatched during the window.
13. During the send-delay window the undo chrome row counts down and `u`
    cancels the send back to Drafts; with `send-delay = 0` the message
    dispatches as soon as the backend is reachable.
14. Send-later sets `scheduled_for` to a chosen future time, and the pending
    row shows in the outbox overlay where it can be canceled to a draft or
    rescheduled.
15. A dispatch failure routes through the §1.7 conflict matrix by sentinel,
    retrying on `ErrConnection` and surfacing on `ErrAuth`, without losing the
    queued message.
16. `Ctrl-J` inserts a named config snippet at the cursor, honors a single
    `{{cursor}}` placeholder, and applies no other substitution.
17. A body carrying an attachment-intent keyword with no attachment attached
    pauses send on a confirm modal offering attach-now or send-anyway, and the
    scan runs at send time.
18. `Ctrl-T` runs tidy over the body or the selection and shows an in-place
    accept-or-reject diff; tidy never alters the body without an explicit
    accept and never runs on send or save, and with no provider configured
    the key is inert and says so.
