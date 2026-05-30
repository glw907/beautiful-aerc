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
to canonical display names regardless of JMAP or IMAP naming. Junk and Trash
are the Disposal folders the empty action targets (section 3.6). Move and
triage semantics operate on the folder.

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
creates in the UI survives a restart as a config entry. They appear in the
sidebar's Saved Searches group next to the classified folder tree.

A stored query is a projection, never a stored result set. Opening one runs
the query against the local FTS5 index, so it resolves offline and stays
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
no standard primitive, so its engine is capability-tiered through
`SupportsNativeMute`. Gmail uses
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
13. On a backend with `SupportsServerSnooze`, a snoozed thread leaves the
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
| `C` | Contacts mode (section 7.3) |
| `/` | Search shelf |
| `u` | Undo last triage (within the undo window) |
| `Q` / `!` | Outbox overlay / conflict overlay |
| `?` / `q` | Help / quit |

The viewer keeps `j`/`k`/`Space`/`b`/`g`/`G` for scroll, `n`/`N` for the
next and previous visible message, `Tab` and `1`-`9` for links, `@` for
attachments, `U` for unsubscribe, `v` to cycle render mode, `V` to respond
to an invite, and `q` to close. Triage, reply, label, snooze, and mute stay
live in the viewer on the open message.

The cross-surface master key map, covering the reader, compose, the search
shelf, contacts mode, and the overlays, is section 8.1.

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
preview pane. Spartan is also the minimum supported width, around 60 columns,
and below it the
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
    󰈻  Starred
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
and starred.

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
column, so a per-account folder shows no marker column.

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
results mode: a flat list of matching messages, each tagged with its account
marker when the scope crosses accounts and an origin-folder prefix when the
scope crosses folders, with no folder-tree context. The thread markers stay,
so a matching thread still folds. Triage on a result dispatches to that row's
owning account, as section 2 requires.

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
attachment chip row, and unsubscribe are owned by Pass 4 (section 4) for
fidelity; this section fixes their placement. Links harvest into a footnote
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
parses a typed time. On a client-managed account, one whose backend reports
`SupportsServerSnooze = false`, the picker states that the item returns at the
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

The undo window uses the notification chrome row above the status bar. A reversible
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
unified context. When the traversal crosses into another folder or account,
the status bar names the jump target.

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
`"markdown"`. In the reader, `v` cycles the active message through the three
modes for a one-off look, leaving the `[ui] body-render` default unchanged
(section 8.1).

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
shape as the terminal-capability resolution poplar runs at startup (section
8.3), and `[ui] inline-images` gates the
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
12. The eval tool renders any raw email file deterministically and emits
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
of that same markdown, using the shared goldmark configuration (section 8.4),
so the structure a recipient sees in an HTML client matches
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
the current time plus the send-delay window (`[ui] send-delay`, 5 seconds
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
body is final. Its keyword set is `[compose] attachment-keywords`, overridable
for a non-English composing language.

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

---

## 6. Search

Owner: Pass 6. Fills the FTS index shape section 1.7 deferred to this pass,
binds the query grammar section 2.3's stored queries hold, and drives the
search shelf, match stepping, and results mode section 3 framed. Search is
local-index-first by section 1.7's performance-by-locality rule. Server search
is too slow and inconsistent to sit on the read path, so every search resolves
against a local FTS5 index in the per-account cache and stays responsive enough
to run as the user types. The field survey behind these decisions is
`docs/poplar/research/2026-05-29-mail-client-gap-analysis.md` §6. Decisions
derive from the field and current practice; legacy poplar's search layer is
evidence, not the baseline.

### 6.1 The local FTS5 index

Each account's cache holds one FTS5 virtual table, `messages_fts`, with its
rowid keyed to the message row. A write happens inside the same transaction
that mutates the message it mirrors, so the index never drifts from the cache.
FTS5 has no UPSERT, so an update is a DELETE then an INSERT. The table carries
its own content rather than running contentless, because contentless delete
semantics are awkward and the storage cost of a plain table is small.

The index splits full text from structured metadata. Its FTS5 columns hold the
matchable text, subject, from, to, cc, body, and attachment text. Predicates a
query filters on rather than matches against stay in the regular message and
attachment tables as SQL constraints, among them folder, account,
`SentAt`, size, flags, label, and attachment presence. A parsed query compiles
to one FTS5 `MATCH` expression and a set of SQL `WHERE` clauses over those
columns. This is the same full-text-versus-metadata split notmuch and the web
clients draw. Snooze and mute state for the `is:snoozed` and `is:muted`
operators reads from the cache's snooze-and-mute projection (section 8.4),
uniform across capability tiers.

The tokenizer is `unicode61` with diacritic folding and a prefix index, so a
prefix query for search-as-you-type resolves quickly. There is no stemming, so
an identifier or a code token matches exactly for the coder audience, and
recall comes from the explicit prefix wildcard instead.

What the index covers, and when each part populates, follows the cache's sync
model from section 1.7.

- **Envelope.** Subject and addresses index eagerly, in the same transaction
  as the metadata sync section 1.7 runs per folder, so envelope search is
  complete for every message the cache knows.
- **Body.** A body indexes when its raw MIME is cached, through the shared
  plain-text extraction, whether the body was fetched on demand or pulled by
  the backfiller below.
- **Attachments.** Filename and content-type index for `has:attachment` and
  `filename:`, alongside extracted text from `text/*` parts. Text extraction
  from binary formats such as PDF and DOCX is named here and deferred post-1.0.

Because bodies fetch on demand, the body column would otherwise cover only
opened messages. A throttled per-account body backfiller closes that gap. It
walks messages that carry an envelope but no cached body, fetches and caches
each one, which populates its FTS body column, and backs off under backend
pressure with an exponential delay. A `warn` substate surfaces in the status
bar while the backfiller sits in that backoff, so a user sees when index depth
is waiting on the server. Foreground reads stay local-first while the
backfiller runs behind them.

The whole FTS table is a rebuildable projection under section 1.7. A schema
change drops and rebuilds it from the message rows and the cached bodies with
no network refetch, so the index never falls under section 1.7's
do-not-discard line.

### 6.2 The query language

A pure `Parse` function turns the query string into a query value with no I/O.
The grammar is Gmail-compatible, the syntax the coder audience already carries.
A bare term matches across subject, addresses, body, and attachment text, and
bare terms combine by implicit AND. A quoted string is an exact phrase match.

The typed operators fall into text, structured, date, and size families.

- **Text.** `from:`, `to:`, `cc:`, `bcc:`, `subject:`, `body:`, and
  `filename:` each match the field they name.
- **Structured.** The structured operators are `in:` (alias `folder:`),
  `account:`, `label:`, `has:attachment`, and `is:` taking `read`, `unread`,
  `starred`, `replied`, `snoozed`, or `muted`.
- **Date.** `before:` and `after:` take a calendar date, and `newer_than:` and
  `older_than:` take a relative span such as `7d`, `2w`, or `3m`.
- **Size.** `larger:` and `smaller:` take a byte count with a `k` or `M`
  suffix.

Boolean structure wraps the terms and operators. Two parts combine by implicit
AND, `OR` unions two sides, parentheses group, and a `-` prefix negates the
term or operator that follows. `account:` is the operator the multi-account
model adds, since a cross-account search needs a way to name one account. An
unknown `key:value` falls through as a bare term, so an operator typo widens
the result set rather than silently shrinking it.

Results sort by `sent_at` descending with no relevance toggle, the default
Geary, K-9, aerc, Apple Mail, Outlook, and Fastmail all settle on.

### 6.3 Scope

A search runs at one of three scopes, a single folder, one account's whole
tree, or across every account. The three line up with the sidebar context from
section 3.3 and with the stored-query scope section 2.3 defined.

Scope defaults to where the sidebar cursor sits. A search from a folder
searches that folder, a search from an account header searches that account,
and a search from the Unified Inbox or a cross-account saved search searches
across accounts. The shelf's scope key (`\`, section 8.1) steps folder to
account to all, and the shelf badge names the active
scope. The `in:` and `account:` operators override the scope from inside the
query, so a folder-scoped shelf still reaches another folder by naming it.

Results render in the section 3.4 results mode, a flat list with an account
marker on each row when the scope crosses accounts and an origin-folder prefix
when it crosses folders.

### 6.4 Search-as-you-type

The shelf searches incrementally and only ever against the local index, so it
stays responsive with the network down. Each keystroke, after a short debounce,
re-runs the query and refreshes the list. The trailing term takes a prefix
wildcard against the prefix index from section 6.1, so a partial word matches
while a completed operator or a quoted phrase stays exact. A query still in
flight is superseded by the next keystroke rather than queued, and every query
carries a row limit, so a fast typist never stacks work.

Operator suggestion runs alongside the typing. A leading fragment such as `fr`
offers `from:`, and `in:`, `is:`, and `label:` offer their valid values, the
folder names, the flag names, and the existing labels. Tab accepts the
highlighted suggestion.

### 6.5 Saved searches

Section 2.3 fixed the stored-query shape (a name, a query, and a scope),
config-persisted and runtime-creatable. Section 6 binds the section 6.2 grammar
and the section 6.3 scope into that shape and gives it a run surface.

Saving the current shelf query is the shelf's `Ctrl-S` action (section 8.1).
It prompts for a name and writes a `[[saved-search]]` block through the same
`config.Render` round-trip section 1 relies on, so a search saved in the UI
survives a restart as config.

Saved searches sit in the sidebar Saved Searches group from section 3.3.
Selecting one runs its stored query at its stored scope against the local FTS
index and renders the matches in results mode. A saved search is a projection,
never a stored result set, so it re-runs on open and on the change events that
touch its scope, the rule section 2.3 set. One of these saved searches is the
label view, the query `label:<name>`, per section 2.2.

Cross-account saved searches scope across accounts and list matches from every
account in scope, each row carrying its account marker and dispatching triage
to its owning account. This is the section 2.3 path to unified surfaces
past the inbox, such as a flagged-across-accounts view. Editing a saved
search's query or scope and deleting one both run through the same config
round-trip.

### 6.6 Acceptance scenarios

1. Each account's cache holds one `messages_fts` FTS5 table; an envelope
   indexes in the same transaction as the message metadata sync, so envelope
   search is complete for every known message.
2. A parsed query compiles to one FTS5 `MATCH` over the text columns plus SQL
   `WHERE` constraints over folder, account, date, size, flags, label, and
   attachment presence.
3. A body indexes when its MIME is cached; the body backfiller fetches
   un-cached bodies in the background, deepens the index, and backs off under
   backend pressure with a `warn` substate shown in the status bar.
4. Attachment filename and content-type index for `has:attachment` and
   `filename:`, and `text/*` attachment parts contribute their extracted text;
   binary-format extraction is absent and named as deferred.
5. With the network down, a search resolves against the local index and stays
   responsive as the query grows character by character.
6. Text operators `from:`, `to:`, `cc:`, `bcc:`, `subject:`, `body:`, and
   `filename:` each match their named field.
7. Structured operators `in:`/`folder:`, `account:`, `label:`,
   `has:attachment`, and `is:` apply each as a SQL constraint.
8. The parser reads the `before:`, `after:`, `newer_than:`, and `older_than:`
   date operators and the `larger:` and `smaller:` size operators with
   byte-suffix values.
9. Implicit AND joins bare terms, `OR` unions, parentheses group, and a `-`
   prefix negates; a quoted string matches as an exact phrase.
10. An unknown `key:value` is treated as a bare term, so an operator typo does
    not silently shrink the result set.
11. Results sort by `sent_at` descending, with no relevance-sort toggle.
12. Search scope defaults to the sidebar cursor's context (folder, account, or
    cross-account); the scope key cycles folder to account to all, and the
    badge names the active scope.
13. An `in:` or `account:` operator overrides the active scope from inside the
    query.
14. Search-as-you-type re-runs on each debounced keystroke, wildcards the
    trailing term against the prefix index, supersedes an in-flight query
    rather than queuing it, and caps result rows.
15. Operator suggestion offers an operator for a leading fragment and offers
    valid values for `in:`, `is:`, and `label:`, accepted with Tab.
16. Saving the current search writes a `[[saved-search]]` config block through
    the `config.Render` round-trip and survives a restart as a sidebar saved
    search.
17. Selecting a saved search runs its stored query at its stored scope against
    the local index and renders the matches in results mode; it re-runs on open
    and on change events touching its scope.
18. A cross-account saved search lists matches from every in-scope account, each
    row carrying its account marker and dispatching triage to its owning
    account.
19. A label view resolves as the saved search `label:<name>`, using the same
    stored-query mechanism as a user-defined saved search.

---

## 7. Contacts, calendar, security

Owner: Pass 7. Settles charter §6 decisions 7 (RSVP and sender verification)
and 8 (the PGP and S/MIME scope call). Fills the reader's `i` contact hook
section 3.5 reserved and feeds the address suggestion seam section 5.2 named.
The pass keeps the contacts surface slim, since poplar is a mail client rather
than a contacts manager, while supporting CardDAV sync as a first-class source
because many users need it. The field survey behind these decisions is
`docs/poplar/research/2026-05-29-mail-client-gap-analysis.md` §7. Decisions
derive from the field and current practice; legacy poplar's contacts, calendar,
and unsubscribe code is evidence, not the baseline.

### 7.1 The contacts model and its sources

Contacts use one format end to end, vCard (RFC 6350). Three sources sit behind
a single query-and-suggest seam.

- **CardDAV books.** A synced, curated, browsable source, and the prominent
  path since many users want sync. Credentials follow section 1.6's
  `[account.contacts]` table and fall back to the parent account, and the
  synced book caches locally so browse, search, and suggest resolve offline.
- **Local vCard store.** A `~/.config/poplar/contacts.vcf` file or a
  `contacts.d/` directory, for a user who runs no CardDAV. It browses exactly
  like a book.
- **Auto-collected addresses.** A recency-decayed cache filled from sent mail,
  used only for suggestion. It never shows in the browse UI and never writes
  back to a book or the local store, so a curated source stays clean.

Poplar surfaces and edits a mail-client-minimal field set, a display name with
multiple emails and multiple phones, each carrying a type label and a primary
marker. Organization, postal addresses, birthdays, and photos never show or
edit. The raw vCard bytes stay the source of truth, so a CardDAV book carrying
richer fields round-trips untouched while poplar edits only the name, the
emails, and the phones. A multi-value read cascades `PREF=1`, then the legacy
`TYPE=pref`, then first-seen. In the list, the primary email and phone show
with a `+N more` count, and the card shows every value with its type and the
primary marked.

### 7.2 The suggest-and-lookup seam

One seam serves the compose autocomplete section 5.2 specified and the reader
contact card. The suggest call queries every source, ranks the curated sources
above auto-collect, and dedupes on email address. The lookup call resolves one
address to a contact for the card. CardDAV, the local store, and the
auto-collect cache all feed the seam through one path, the shape section 5.2
relies on.

### 7.3 Contacts mode and the reader card

Contacts mode stays slim. The `C` key (section 8.1) switches the account view
to a contacts list with an A-Z index in the sidebar. `Enter` opens
a detail card. The reader keeps the `i` hook from section 3.5. On a sender line
it opens a contact-card popover with the sender's name, emails, and phones, the
primary of each marked, and an add-to-contacts action when the sender is
unknown.

```
╭─ Alice Johnson ──────────────────╮
│  alice@example.com      work ★   │
│  alice.j@gmail.com      home     │
│  +1 555 0101            mobile ★ │
│  a add to contacts  e edit  Esc  │
╰──────────────────────────────────╯
```

Create and edit write name, emails, and phones to the local store or a chosen
CardDAV book. In v1 the write lands in one default destination, and the
multi-book destination picker is deferred post-1.0 per ADR-0176. The editor
manages the email and phone lists, so a value can be added, removed, retyped,
or set as the primary.

### 7.4 Calendar: ICS invites and RSVP

A `text/calendar` part or an `.ics` attachment renders inline as an invite
block showing the title, the time, the place, the organizer, and the user's
status. One-action RSVP offers accept, tentative, and decline.

```
 ┌ Invite ──────────────────────────────────┐
 │  Q2 Planning                              │
 │  Thu 10 Apr 2026 · 14:00-15:00            │
 │  Organizer: Bob Lee                       │
 │  Status: needs action                     │
 │  V respond                                │
 └───────────────────────────────────────────┘
```

Section 1 carries no calendar account, so RSVP sends an iMIP reply rather than
writing a calendar. The reply is a `text/calendar` part with `METHOD=REPLY` and
the user's `PARTSTAT` updated, emailed to the organizer through the Send and
outbox path section 5 built. That path needs no calendar backend. Writing the
event into a CalDAV calendar is named here and deferred, since v1 has no
calendar account model. The reader's `V` key opens the RSVP picker (section
8.1); accept, tentative, and decline send the iMIP `METHOD=REPLY`, and the
invite block reflects the new status once the reply queues.

### 7.5 Security: sender verification

Poplar reads the `Authentication-Results` header (RFC 8601) the trusted
receiving boundary stamps and shows a compact badge in the reader header for the
DKIM, SPF, and DMARC results. Trusting the delivery boundary is the standard
mail-client approach, since the client cannot reliably re-fetch DNS and the raw
signature on every message. Local re-verification, fetching the DNS records and
re-checking the DKIM signature over the raw MIME, is named here and deferred as
a later hardening. A DMARC failure or a From-domain mismatch shows a clear
warning rather than a quiet pass.

```
  From:  Alice Johnson <alice@example.com>
  ✓ dkim  ✓ spf  ✓ dmarc          signed · key CACA1234
```

### 7.6 Security: read-side PGP

v1 verifies PGP signatures and decrypts encrypted incoming mail, and it does not
sign or encrypt on send. It reads PGP/MIME (RFC 3156) and inline-PGP signatures,
and it decrypts a PGP/MIME or inline-PGP body. Keys come from the local GnuPG
keyring through gpg-agent, the source a coder's machine already carries. A
missing key shows an unverified state with a no-public-key reason rather than
failing silently, and a decrypted body renders through the section 4 pipeline
like any other. The reader shows a signature chip beside the section 7.5
verification badge, naming a good signature's key, a bad signature, or an
unknown key.

Signing and encrypting on compose, and S/MIME in either direction, are named
here and deferred to a post-1.0 encryption pass. Their deferred scope is
send-side key selection, recipient key lookup, and the trust UI, the surface
that makes outbound encryption a larger undertaking than inbound verification.

### 7.7 List-Unsubscribe

One-click List-Unsubscribe carries forward unchanged from section 3.5 and
ADR-0185. The reader harvests `List-Unsubscribe` and `List-Unsubscribe-Post` at
body fetch, `U` runs it, and it routes an https one-click POST ahead of a mailto
ahead of an http link, behind a confirm and a short banner. It sits in the
reader's affordance set beside the verification surfaces, so the
security-relevant reader actions read as one group.

### 7.8 Seam additions

Section 7 adds three seams the build plans fill. The contacts seam wraps a
CardDAV client, a vCard parser, the local-file source, and the auto-collect
cache behind one query-suggest-and-store interface. A calendar seam parses ICS
through the locked `arran4/golang-ical` and assembles the iMIP `METHOD=REPLY`
that rides the Send path. The security seam parses `Authentication-Results` and
verifies and decrypts PGP against the local keyring. Exact signatures are
build-plan work; this section fixes the sources, the surfaced fields, and the
capability boundaries.

### 7.9 Acceptance scenarios

1. Contacts read from three sources behind one seam, a synced CardDAV book, a
   local `contacts.vcf` or `contacts.d/`, and an auto-collected cache; the
   CardDAV book caches locally and browses offline.
2. A user with no CardDAV browses and searches the local vCard store
   identically to a book.
3. Auto-collected addresses feed suggestion only; they never appear in the
   browse UI and never write back to a book or the local store.
4. The surfaced and editable field set is a display name with multiple emails
   and multiple phones, each with a type and a primary marker, and other vCard
   fields round-trip untouched and never display.
5. A multi-value field resolves its primary by `PREF=1`, then `TYPE=pref`, then
   first-seen; the list shows the primary email and phone with a `+N more`
   count, and the card shows every value typed and primary-marked.
6. Compose autocomplete and the reader card draw from the same suggest seam,
   which ranks the curated sources above auto-collect and dedupes on email.
7. A mode key opens contacts mode with an A-Z sidebar index, and `Enter` opens a
   contact detail card.
8. `i` on a sender line opens the contact-card popover, and for an unknown
   sender it offers add-to-contacts.
9. Creating or editing a contact writes name, emails, and phones to the default
   destination; the editor adds, removes, retypes, and sets the primary of an
   email or phone, and the multi-book destination picker is absent and named as
   deferred.
10. A `text/calendar` part or an `.ics` attachment renders inline as an invite
    block with title, time, place, organizer, and the user's status.
11. Accept, tentative, or decline sends an iMIP `METHOD=REPLY` with the updated
    `PARTSTAT` to the organizer through the outbox, the invite block reflects
    the new status, and no calendar backend is required.
12. Writing the event to a CalDAV calendar is absent in v1 and named as
    deferred.
13. The reader header shows a sender-verification badge derived from the
    `Authentication-Results` header for DKIM, SPF, and DMARC, and a DMARC
    failure or a From-domain mismatch shows a warning.
14. Local DNS-and-raw-MIME re-verification is absent in v1 and named as
    deferred.
15. v1 verifies PGP/MIME and inline-PGP signatures and decrypts encrypted
    incoming mail using the local GnuPG keyring; a missing key shows an
    unverified, no-public-key state rather than a silent pass, and the decrypted
    body renders through the section 4 pipeline.
16. Signing and encrypting on compose and S/MIME are absent in v1 and named as
    deferred, with the deferred send-side scope stated.
17. One-click List-Unsubscribe carries forward from section 3.5, harvested at
    body fetch, run by `U`, and routing an https one-click POST ahead of mailto
    ahead of http behind a confirm.

---

## 8. Consolidation

Owner: Pass 8. This section folds the seven domain sections into one whole. It
adds no features. It assigns the keys earlier passes deferred to "the locked
section 3 keymap," names one source of truth for the binding tables, settles
command visibility across the responsive tiers, unifies the capability
vocabulary, maps the load-bearing seams and the contracts they share, and
gathers the deferral register, the acceptance-scenario index, the glossary, and
the config-key index. Where this section and a domain section once disagreed,
the domain section was edited to match and the canonical statement now lives
here.

### 8.1 The unified keyboard map

The keyboard model is the one section 3.1 fixed: modifier-free single keys, no
multi-key sequences, no `:` command mode, with text-entry surfaces (compose, the
search shelf, picker filters) as the exemption that takes the whole keyboard.
This section completes that model across every surface and names its source of
truth.

**Contexts.** A key's meaning is scoped to the surface that owns it. The
surfaces are the account view (sidebar plus message list, the keymap the
widescreen preview pane also drives), the reader, compose, the search shelf,
contacts mode, and the modal overlays. The account view and the reader share one
pane and have distinct keymaps, so a key may carry one meaning in the list and
another in the open message. Section 3.1 already relies on this split: `n`/`N`
step unread in the account view and step messages in the reader, and `g`/`G` and
`Space` differ the same way. Pass 8 extends the split to the keys it newly
assigns and states the rule once here so the reuse reads as design rather than
collision.

**The overlay-shadow rule.** While any overlay or picker is open, its local keys
take precedence and the global keymap is suspended. Only the overlay's own keys,
`Esc` to cancel, and the unadvertised `Ctrl-c` terminal-kill stay live. This is
why a picker may bind `a`, `t`, `d`, or `e` to local choices without colliding
with account-view triage: the global triage keys do not fire while the picker
owns the keyboard. The RSVP picker, the contact-card popover, and the
select-by-criteria menu all rely on this rule.

**Account view.** The canonical account-view map, extended from section 3.1 with
the contacts-mode key:

| Key | Action |
|-----|--------|
| `j` / `k` | Message cursor down / up |
| `J` / `K` | Sidebar cursor down / up |
| `h` / `l` | Collapse / expand the sidebar node (`←`/`→` alias) |
| `g` / `G` | Message list top / bottom |
| `Space` / `F` | Fold the thread under the cursor / fold all threads (toggle selection when a visual selection is active) |
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
| `C` | Contacts mode (toggle) |
| `/` | Search shelf |
| `u` | Undo last triage (within the undo window) |
| `Q` / `!` | Outbox overlay / conflict overlay |
| `?` / `q` | Help / quit |

**Reader.** The reader keeps the account view's triage, reply, label, snooze,
and mute keys live on the open message, and adds its own surface keys:

| Key | Action |
|-----|--------|
| `j` / `k` / `Space` / `b` / `g` / `G` | Scroll the body |
| `n` / `N` | Next / previous visible message |
| `Tab`, `1`-`9` | Open a footnote link (picker, or by number) |
| `@` | Attachment picker |
| `U` | Run List-Unsubscribe |
| `i` | Sender contact card |
| `v` | Cycle render mode (markdown, html, plain) for this message |
| `V` | Respond to invite (opens the RSVP picker when the message carries one) |
| `q` | Close the reader |

`v` in the reader cycles the open message through the three render modes for a
one-off look. It is per-message and does not change the `[ui] body-render`
default. `V` in the reader opens the RSVP picker only when the message carries an
ICS invite. The reader's `v` and `V` are context-distinct from the account
view's visual-select `v` and select-by-criteria `V`: a single open message has
nothing to multi-select, so the meanings never overlap in use.

**Compose.** Compose is a text-entry surface. Poplar owns the message-level
chords and catkin owns the body (section 5.1).

| Key | Action |
|-----|--------|
| `Ctrl-X` | Send (queues to the outbox) |
| `Ctrl-O` | Save draft and close |
| `Ctrl-A` | Attach a file |
| `Ctrl-J` | Insert a snippet |
| `Ctrl-T` | Tidy (catkin) |
| `Tab` / `Enter` | Accept the highlighted address suggestion |
| `Esc` | Leave the surface (postpone a dirty draft, offer discard for an empty one) |

**Search shelf.** The shelf is a text-entry surface. Typed characters build the
query; the reserved keys are shelf commands:

| Key | Action |
|-----|--------|
| `Tab` | Accept the highlighted operator or value suggestion |
| `Enter` | Run the query (results render in the section 3.4 results mode) |
| `\` | Cycle scope: folder, account, all |
| `Ctrl-S` | Save the current query as a saved search |
| `Esc` | Close the shelf |

The shelf reserves `\` from text entry. The query grammar (section 6.2) uses no
backslash, so reserving it costs nothing. `Ctrl-S` follows compose's precedent
that a text-entry surface may carry a chord.

**Contacts mode.** `C` switches the account view to contacts mode and back. The
sidebar becomes an A-Z index, `J`/`K` walk it, `Enter` opens a contact detail
card, and `i` on the reader's sender line still opens the contact-card popover
(section 7.3). `q` or `C` leaves the mode.

**Overlays and pickers.** Each opens over a dimmed underlay, one at a time,
through the section 3.7 cascade, and runs under the overlay-shadow rule. A
label picker filters by text with `Space` to toggle and `Enter` to apply.
Snooze and RSVP pickers navigate with `j`/`k` and commit with `Enter`.
Select-by-criteria, the contact-card popover, and the confirm modal use
single-letter local choices. `Esc` cancels every overlay.

**The single source of truth.** The build phase keeps one Go binding registry of
`key.Binding` values grouped by context, and every surface dispatches through
`key.Matches` against it (the elm-conventions and bubbles pattern). The help
popover renders from that registry, and a test asserts the popover lists exactly
the registry's bindings, so the help, the registry, and the code cannot drift.
The tables in this section are the spec-phase authority the registry must match.
No separate hand-maintained keybindings document is the source of truth; if one
ships for the website or the manual, it generates from the registry.

### 8.2 Command visibility across the responsive tiers

Bindings are constant across every tier at or above the Spartan floor. A key
bound at the full tier stays bound at Spartan. What changes by tier is what the
chrome advertises, the same data-driven cliff that drops the date and flag
columns, the label chips, and the sidebar detail in section 3.2.

| Tier | Width | Advertised affordances |
|------|-------|------------------------|
| Spartan | floor (around 60 columns) to the intermediate breakpoint | Keys all bound; on-screen hints minimal; the help popover (`?`) carries the full reference |
| Intermediate | up to the full breakpoint | Flags and a compact date return; common hints reappear in the chrome |
| Full | up to the widescreen breakpoint | Every chrome element on; command hints shown |
| Widescreen | around 130 columns and up | Full tier plus the offer of the preview pane (`P`) |

A smaller screen hides the hint, not the key. The help popover is the
always-complete reference at every tier, so a user who cannot see a hint reaches
the binding through `?`. Below the Spartan floor the layout is undefined.

### 8.3 Capability vocabulary

Poplar gates a feature where the backend, the transport, or the terminal cannot
support it, and says so rather than faking it. Three families of capability
exist, and they use different mechanisms on purpose.

**Backend capability flags.** Named `Supports*` booleans on the Backend
interface, each with a defined fallback so the feature still works when the flag
is false.

| Flag | Gates | Fallback when false |
|------|-------|---------------------|
| `SupportsLabels` | The label surface for the account | Label chrome absent; the UI states labels are unavailable |
| `SupportsServerRules` | Server-side rule (Sieve) management | The rule surface states server rules are unavailable |
| `SupportsServerSnooze` | The server snooze engine | Client-managed snooze (managed Snoozed folder, returns at the next sync after the wake time) |
| `SupportsNativeMute` | The native mute-label engine (Gmail) | A generated Sieve mute rule, or a cache mute list applied on sync |

Acceptance scenarios state these in the crisp `SupportsX` form. The mute spec
(section 2.5) and the snooze picker (section 3.6) were edited to name
`SupportsNativeMute` and `SupportsServerSnooze` at their feature sites rather
than describing the gate in loose prose.

**Transport capabilities.** Detected from the protocol session, not exposed as
poplar `Supports*` flags: IMAP MOVE versus COPY-fallback, SPECIAL-USE, IDLE
versus poll, CONDSTORE and QRESYNC (RFC 7162) for delta sync, the
`urn:ietf:params:jmap:sieve` and `urn:ietf:params:jmap:mail:snooze` JMAP
capabilities, and the ManageSieve `SIEVE` capability line with its
`sieveExtensions`. An IMAP backend runs two base connections, command and idle,
and opens a third for ManageSieve when the account advertises it; a JMAP backend
uses one HTTP client plus its event source. The two unratified snooze drafts are
confirmed against the live account capability at build time.

**Terminal capabilities.** Resolved once at startup, the same shape for each: the
graphics protocol probe (kitty, iTerm2, sixel) that gates `[ui] inline-images`,
and the Nerd Font and cell-width probe that resolves icon mode. These are
terminal facts, not backend or transport facts, so they are resolved in process
startup and threaded into the UI rather than carried on the Backend interface.

### 8.4 Cross-section seams and shared contracts

The charter named six load-bearing seams. Each domain section consumes them; this
subsection states each once and lists where it is shared.

- **Backend.** JMAP and IMAP behind one synchronous interface (section 1.3),
  consumed by the cache drainer, by triage and label mutators (sections 2, 3),
  by the outbox drainer's `Send` and `Append` (section 5), and by the RSVP iMIP
  reply that rides `Send` (section 7.4).
- **Per-account cache.** The source of truth the UI reads (section 1.7). It
  carries the outbox and its conflict matrix (consumed by sections 2.6, 3.6,
  5.6, 5.7), the FTS5 index `messages_fts` (section 6.1, consumed by the saved
  searches of sections 2.3 and 6.5), the draft store (section 5.6), the
  client-managed snooze wake table and mute list (section 2.5), the
  auto-collect address cache (section 7.1), and the snooze-and-mute projection
  the reader and the `is:snoozed` / `is:muted` operators read (sections 3.5,
  6.2).
- **UI tree.** The Elm root and its bubbles-shaped subpackages (section 3). The
  preview pane renders through the reader's own code (section 3.2), so the
  rendering contract covers both surfaces without a second path.
- **Catkin.** The poplar-agnostic markdown editor that hosts the compose body
  and the tidy command (sections 5.1, 5.9). Poplar claims the message-level
  chords and catkin leaves them unbound, so the boundary holds for the
  standalone editor.
- **Content renderer.** The block-tree pipeline that turns a MIME part into
  themed terminal lines (section 4). It renders the reader body (section 3.5),
  the preview pane (section 3.2), and a decrypted PGP body (section 7.6). It is
  poplar's own renderer, distinct from the goldmark markdown-to-HTML engine
  compose uses for the text/html part (section 5.5); the two share the same
  markdown source, not the same code.
- **Rendering eval harness.** The dev-tagged tool, corpus, and judge loop
  (sections 4.7, 4.8), fenced off from the live backend, the cache, and the UI
  tree. Referred to as the eval tool throughout.

Three contracts span sections and are stated once here so the build plans share
one definition:

- **The shared time parser.** The snooze custom row (section 3.6) and send-later
  (section 5.7) use one parser. It accepts the named presets, relative forms
  (`30m`, `2h`, `3d`, `tomorrow 9am`), and absolute forms (`2026-06-01`,
  `2026-06-01 14:00`). An unparseable entry surfaces an error and commits
  nothing.
- **Account accent color.** Each account renders with a stable accent color used
  by the cross-account row marker (section 3.4) and the reader account chip
  (section 3.5). It comes from `[account] color` when set, and otherwise from a
  fixed palette ring assigned by config order. The assignment is identical
  across restarts.
- **The suggest-and-lookup seam.** One path feeds compose autocomplete (section
  5.2) and the reader contact card (section 7.2). It ranks curated sources above
  auto-collect and dedupes on email.

### 8.5 Resolved drift and the canonical terms

The self-review fixed the following in the owning sections. The canonical term
is in bold; retired synonyms are noted.

- **Saved search** is the user-facing term and **stored query** is the
  underlying type (one type backs saved searches, label views, and the former
  "virtual folder"). "Virtual folder" is retired. The sidebar group is "Saved
  searches."
- **Star** and **starred** are the user-facing flag and state (the `s` key, the
  `is:starred` operator). The backend mutator is `Flag(uids, "starred", set)`;
  "flag" names the generic mechanism and the status-glyph column, not the user
  action. Section 3.4's column description and section 3.3's saved-search name
  were normalized to "starred."
- **Disposal folder** means Junk or Trash, the two folders `E` empties. The
  term is anchored in the folder model at section 1.2.
- **FTS5 index** is the consistent name; the table is **`messages_fts`**.
  Section 2.3's "local FTS index" was corrected to "local FTS5 index."
- **`SentAt`** is the Go metadata field and **`sent_at`** is the SQL and FTS
  column. They are the same value in two casings, noted in the glossary.
- Render pipeline and goldmark engine were disentangled (section 8.4);
  section 5.5 no longer attributes a "goldmark configuration" to the reader.
- Bottom-of-screen surfaces are named once: the **status bar** (bottom
  line), the **notification chrome row** above it (the undo and toast row), the
  **compose command row** (compose foot), and the **search shelf** (the `/`
  input). Section 3.6 was reworded off "legacy chrome row."
- Section 3's references to "the legacy list," "the legacy reader," and "the
  legacy chrome row" were reworded to describe the behavior directly, since the
  rebuild does not port the archived code.
- `is:snoozed` and `is:muted` (section 6.2) resolve against the cache's
  snooze-and-mute projection (section 8.4), so they return the same set on every
  capability tier. On an account with `SupportsLabels = false`, `label:` matches
  nothing, a label-view saved search scoped there is empty, and a cross-account
  label search omits that account.
- The undo within the window is a capability-matched inverse op (section 1.7):
  for server snooze it issues the inverse server operation, for client snooze it
  clears the cache wake entry, and for mute it removes the label, the rule, or
  the list entry. The window applies to every tier.

### 8.6 The deferral register

The features named and deferred across the sections, gathered for the build
plans. Each is out of v1 scope.

| Deferred | Section | Note |
|----------|---------|------|
| Unified Sent and unified Flagged views | 1.1 | Cross-account stored queries deliver these later |
| Lazy-connect knob | 1.4 | Connecting all accounts at startup is the v1 default |
| Gmail server-side rule management | 2.4 | Needs the Gmail REST API; tracked post-1.0 |
| Send-side PGP (signing and encrypting) | 7.6 | A post-1.0 encryption pass owns the send side |
| S/MIME in either direction | 7.6 | Post-1.0 |
| CalDAV calendar write | 7.4 | No calendar account model in v1 |
| Binary attachment text extraction (PDF, DOCX) | 6.1 | `text/*` extraction ships; binary is post-1.0 |
| Multi-book contacts destination picker | 7.3 | One default destination in v1 (ADR-0176) |
| Runtime LLM HTML cleaning | 4.9 | A deferred opt-in, never the default path |
| Local DNS and raw-MIME re-verification of DKIM | 7.5 | The delivery-boundary header is trusted in v1 |
| Vim modal editing in catkin | 5.1 | The rebuild ships the modeless editor |
| A shipped verified OAuth client | 1.6 | Bring-your-own client for v1; a verified preset is a post-1.0 milestone |

### 8.7 The acceptance-scenario index

The domain sections carry 122 numbered scenarios. Pass 8 adds 10 for the
decisions this section settled. The build plans turn the full set into the
done-contract.

| Section | Scenarios |
|---------|-----------|
| 1.8 Accounts, protocols, sync | 15 |
| 2.8 Organization, threading, automation | 16 |
| 3.8 Reading, triage, navigation | 19 |
| 4.10 Message rendering and display | 18 |
| 5.10 Compose and sending | 18 |
| 6.6 Search | 19 |
| 7.9 Contacts, calendar, security | 17 |
| 8.7 Consolidation | 10 |
| **Total** | **132** |

Pass 8 acceptance scenarios:

1. With any overlay or picker open, the global keymap is suspended; only the
   overlay's local keys, `Esc`, and `Ctrl-c` act, so `a` in an open invite
   picker accepts the invite and never archives.
2. In the full reader, `v` cycles the open message through markdown, html, and
   plain render modes; the change is per-message and leaves `[ui] body-render`
   unchanged.
3. `C` switches the account view to contacts mode and back.
4. The reader's `V` opens the RSVP picker for a message carrying an invite,
   and accept, tentative, or decline sends the iMIP reply through the outbox.
5. The search shelf binds `\` to cycle the scope folder to account to all and
   `Ctrl-S` to save the current query as a saved search.
6. Every key bound at the full tier stays bound at the Spartan tier; narrowing
   the terminal recedes hints and chrome, never a binding, and the help popover
   lists the complete set at every tier.
7. The help popover lists exactly the bindings in the build-phase key registry,
   and a test fails if the two diverge.
8. Each account renders with a stable accent color taken from `[account] color`
   when set and otherwise assigned from a fixed palette ring by config order,
   identical across restarts.
9. The snooze custom row and send-later share one time parser that accepts the
   documented preset, relative, and absolute forms, and an unparseable entry
   commits nothing.
10. `is:snoozed` and `is:muted` resolve against poplar's local snooze-and-mute
    projection and return the same set on every backend tier.

---

## Appendix A: Glossary

Canonical terms, with retired synonyms noted.

- **Account.** One mail identity domain: one backend, its credentials, one or
  more sending identities, and its own SQLite cache.
- **Identity.** A From address, an optional display name, and ordered
  signatures. An **alias-pattern identity** matches an address pattern so a
  reply under a wildcard domain sends from the right address.
- **Backend.** JMAP or IMAP behind one synchronous interface.
- **Capability flag.** A `Supports*` boolean on the Backend (section 8.3),
  distinct from transport and terminal capabilities.
- **Folder.** A message's single location, classified per account (Inbox, Sent,
  Trash, Archive, Drafts, Junk, Custom). **Disposal folder** means Junk or
  Trash.
- **Label.** A multi-membership server-backed tag. A **label view** is the saved
  search `label:<name>`.
- **Saved search.** A user-facing named query. **Stored query** is its
  underlying type. "Virtual folder" is the retired synonym.
- **Unified inbox.** The read-side cross-account merge of every account's Inbox.
- **Thread / conversation.** A first-class organizational unit; triage, snooze,
  and mute target it.
- **Snooze / mute.** Capability-tiered features with one UX each (sections 2.5,
  8.3).
- **Outbox.** The durable optimistic operation queue. The **drainer** dispatches
  rows; the **conflict matrix** routes failures by sentinel; the **inverse op**
  is the undo.
- **Cache.** A per-account SQLite store, the source of truth the UI reads,
  built on **performance-by-locality**. Its **FTS5 index** is the
  `messages_fts` table.
- **`SentAt` / `sent_at`.** The Go metadata field and the SQL and FTS column for
  the same sent timestamp.
- **Star / starred.** The user-facing flag and state, mutated through
  `Flag(uids, "starred", set)`. A **flag column** is the status-glyph column.
- **Account view / reader / preview pane.** A sidebar-plus-list surface, the
  open message, and the widescreen follower pane.
- **Search shelf / results mode / notification chrome row / compose command row
  / status bar.** The named UI surfaces (section 8.5).
- **Render pipeline.** Poplar's block-tree renderer (section 4), distinct from
  the **goldmark** markdown-to-HTML engine compose uses (section 5.5). The
  **rendering contract**, the **gold standard**, the **eval tool**, and the
  **golden corpus** are the rendering program's artifacts.
- **Catkin.** The poplar-agnostic markdown editor. **Tidy** is its user-invoked
  AI prose command.
- **Suggest-and-lookup seam.** The one path feeding compose autocomplete and the
  reader contact card.

## Appendix B: Config-key index

The config keys the spec introduces. The build plans own the full schema and
defaults; this index is the surface the sections name.

- `[ui] unified-inbox` (bool, default true).
- `[ui] body-render` (`"markdown"` default, `"html"`, `"plain"`).
- `[ui] inline-images` (bool, default false; gated by terminal graphics
  support).
- `[ui] send-delay` (duration, default 5s; 0 disables the undo-send window).
- `[ui] undo-window` (duration, default 5s; the triage-undo countdown).
- `[ui.folders.<name>] threading` (per-folder threaded or flat default).
- `[compose] attachment-keywords` (list; overrides the default English
  attachment-intent set for a non-English composing language).
- `[account] color` (accent color; optional, else assigned from the palette
  ring).
- `[account] primary` (bool; the default account for a fresh unified-view
  compose).
- `[[snippet]]` blocks (`name`, `body` with one `{{cursor}}` placeholder).
- `[[saved-search]]` blocks (`name`, `query`, `scope`).
- `[account.identity]` and `[account.identity.signature]` (section 1.5).
- `[account.oauth]`, `[account.contacts]` (sections 1.6, 7.1).
