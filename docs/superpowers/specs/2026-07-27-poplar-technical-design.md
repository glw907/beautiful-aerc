# Poplar technical design

**Date:** 2026-07-27
**Status:** Revision 2, for the Phase 4 gate. Revision 1 was
adversarially reviewed by three independent lenses (requirements
traceability, architecture attack, horizon and C11 audit; ~85
findings, 8 blockers); this revision folds them. Findings that
changed a decision are reflected in the ADRs.
**Charter:** `2026-07-19-poplar-refounding-charter.md`
**Requirements:** `2026-07-27-poplar-requirements.md` (binding; its
constraints C1-C11 govern every decision here).
**Survey:** `docs/poplar/research/2026-07-27-phase4-library-survey.md`
(the C9 evidence base; versions cited there are not repeated here).
**ADRs:** `docs/superpowers/specs/adr/` holds one record per major
decision with alternatives considered.
**Companion artifacts:** the design language
(`2026-07-27-poplar-design-language.md`, the UX-3 deliverable) and
the outgoing-HTML allowlist
(`2026-07-27-poplar-html-allowlist.md`, the CO-5 artifact).

## 1. Shape of the system

Poplar is one process, one binary, three layers:

1. **The store** (`internal/store`): one SQLite database holding
   every account's mail, calendar, contacts, outbox, sync state,
   and the full-text index. The store is the truth the UI reads
   (SY-1). One writer goroutine owns every write; read connections
   serve the UI.
2. **The engines** (`internal/sync`, `internal/outbox`,
   `internal/render`, `internal/calendar`, `internal/contacts`,
   `internal/search`): background workers that keep the store
   current and pure functions that transform its contents. Engines
   never touch the terminal.
3. **The UI** (`internal/ui`, `internal/theme`, `internal/catkin`):
   a bubbletea v2 program. The UI reads the store through commands,
   mutates only by enqueueing intents, and hears about the world
   through messages. It performs no network I/O and no store
   writes.

Data flows in one loop: server → sync engine → store → UI, and
user intent → store (optimistic local mutation) + outbox → dispatch
→ server → sync engine confirms → store. The UI never waits on the
network (C1). The user-initiated network acts are the four C1
exceptions plus RD-11's confirmed one-shot unsubscribe POST, and
every one carries the same contract: a visible progress state, a
bounded timeout (10 s default; body fetch additionally shows its
100 ms progress indicator per SY-6), and a named offline behavior
(the action reports "offline, will not retry" for fetches and
image loads; server search degrades by name per SR-7; unsubscribe
refuses offline with a notice).

## 2. Package boundaries

```
cmd/poplar             wiring and main
internal/store         schema, migrations, reads, the writer, FTS
internal/backend       the seam: Backend interface + capabilities
internal/backend/jmap  Fastmail JMAP backend (go-jmap)
internal/backend/dav   CalDAV calendar transport (go-webdav)
internal/sync          delta orchestration, watermarks, push, backoff
internal/outbox        durable intent queue, dispatch, typed failures
internal/mail          MIME parse/assembly, threading, reply seeding,
                       identity matching, flowed decode
internal/render        the rule engine, pipeline, fact check, traces
internal/calendar      event model, recurrence, iTIP, occurrence index
internal/contacts      ContactCard model, autocomplete ranking
internal/search        query grammar, FTS query build, merge
internal/when          the shared natural-language time parser (C6)
internal/uerr          the error seam (ER-1)
internal/config        the config struct, load/persist, migration
internal/keyring       credential storage: per-account entries, the
                       named file fallback, the refresh seam
internal/platform      opener, clipboard, notifications, instance lock
internal/theme         compiled tokens: colors, glyphs, spacing roles
internal/ui            root model, screen registry, screens
internal/catkin        the live-markdown editor; no poplar imports
```

Dependency rules, enforced by an import-boundary test in the gate:
`ui` imports `store` (read API), `theme`, `catkin`, `uerr`, and the
intent types; it never imports `backend`, `sync`, or `outbox`
directly. `catkin` imports nothing from poplar and its dependency
ceiling is bubbletea v2, lipgloss v2, and goldmark (the spinoff's
go.mod is knowable in advance). `render`, `when`, `search`, and
`calendar` logic packages are pure: no I/O, no store handles.
`backend` implementations are the only packages that speak a wire
protocol.

## 3. Data model and store schema

One SQLite file for all accounts at
`$XDG_DATA_HOME/poplar/store.db`, WAL mode. One file, not one per
account: `account_id` on every scoped row is the isolation
mechanism, the single writer and single instance lock stay true at
N accounts, and cross-account views (a unified inbox, horizon 1)
stay reachable as an account-predicate change. The engine, schema
shape, and concurrency discipline are ADR-0001 and ADR-0002; the
account seam is ADR-0004.

**Fat-table hybrid.** Every entity table carries a `data` JSON
column holding the full serialized model plus scalar columns for
exactly the fields queries touch. The schema review enforces the
corollary: no query predicate reaches into JSON on a hot path.
Where a LATER feature's predicate would live in JSON, the scalar
column exists now (see the reserved columns below).

**Poplar mints every primary key.** Internal 64-bit integer keys;
server identifiers (JMAP Email id, blobId, CalDAV href, ETag) are
indexed, replaceable side columns. Resync reconciles by matching
`server_id` and never re-mints a key for a surviving row, so
bodies, membership, draft metadata, and FTS rowids survive resync
by construction.

**Account identity on every scoped row.** Every account-scoped
table carries `account_id`; the schema test asserts the rule as:
every table not reachable by foreign key from a scoped parent
carries the column (this covers `sent_history`, which scopes
autocomplete ranking per account).

**Origin and durability.** `message.origin` is `server` or
`local`. Resync and SY-8 rebuild preserve `origin='local'` rows
and their bodies; `server_id` is nullable for them. This is the
column that keeps ST-4 (import) open: an importer creates local
rows, and no routine resync deletes them. (Revision 1 foreclosed
this; the horizon review caught it.)

Core tables (scalar columns abbreviated):

```
account        id, slug, backend_kind, address, data
mailbox        id, account_id, server_id, role, name, sort_order,
               visible, unread_count, total_count, data
message        id, account_id, server_id, blob_id, thread_key,
               received_at, subject, from_addr, flags, size,
               has_attachment, origin, hidden_until, search_text,
               data
message_mailbox  message_id, mailbox_id, received_at
message_fts    FTS5 over message(subject, search_text)
body           message_id, content BLOB, fetched_at   -- raw MIME
thread         id, account_id, server_thread_id, muted, data
calendar       id, account_id, href, name, color_slot, visible,
               is_default, data
event          id, account_id, calendar_id, uid, href, etag,
               raw_ics BLOB, summary, start_local, tzid,
               duration_secs, is_all_day, is_floating,
               is_recurring, transparency, sequence, data
occurrence     event_id, start_utc, end_utc, start_local,
               local_date, recurrence_id
contact_card   id, account_id, server_id, uid, full_name, data
contact_email  contact_card_id, address, rank_hint
sent_history   account_id, address, name, last_used_at, use_count
outbox         id, account_id, kind, payload JSON, state,
               undo_group, chunk_seq, attempt_count,
               next_attempt_at, failure_class, failure_detail,
               created_at
draft_meta     message_id, local_rev, pushed_rev, anchor_msgid
sync_state     account_id, object_kind, server_state_token,
               local_rev
schema_version singleton
```

Reserved-for-LATER scalars, present from migration one so the
claims in section 16 are mechanisms rather than assertions:
`message.hidden_until` (snooze), `thread.muted` (thread mute,
also the `thread` table's consumer), `mailbox.visible` (the
capture mailbox, symmetric with `calendar.visible`).

**Raw retention, defined.** `body.content` is the raw MIME source
with one bounded elision: attachment part bodies over 256 KB are
replaced by a placeholder recording length and SHA-256. Inline
and small parts stay verbatim, so rendering never needs the
network; large attachments remain the RD-8 on-demand fetch (the
C1 exception), and QA-5's overhead ceiling is measured against
this retained form. Corpus specimens (horizon 3 and 4) are this
form, stated in their sidecars.

**Search text.** `message.search_text` is the extracted body text
(from the rendering pipeline's plan stage), denormalized onto the
message row and maintained in the same transaction as the message
write that produces it. `message_fts` is FTS5 over
`message(subject, search_text)` with a single content source, so
the external-content delete-before-update discipline has exactly
one table to agree with. One store-internal helper owns every FTS
maintenance statement and reads current row state inside the same
transaction before writing. A message indexed before its body
arrives is re-indexed in the backfill transaction that lands the
body; SR-1's atomicity is per-mutation (each transaction leaves
index and store agreeing about what is present), and coverage
growth is SR-7's visible state. `INSERT INTO
message_fts(message_fts) VALUES('integrity-check')` runs in the
QA-6 restart assertions and at the end of the SR-1 randomized
script, because row-count equality does not detect term rot. The
FTS table declares `prefix='2 3'` for search-as-you-type. The
index is derived state: `--rebuild-index` regenerates it.

**The index set** is a schema artifact in ADR-0002, not an
implementation detail. The hot ones: `message_mailbox(mailbox_id,
received_at DESC, message_id)` (covering; the list query),
`message(account_id, thread_key, received_at)` (thread views,
TH-2 across folders by construction), partial index on unread by
mailbox (LT-6, FO-2 counts), `outbox(state, next_attempt_at)`,
`occurrence(start_utc)` and `occurrence(local_date)`. Every hot
query carries an `EXPLAIN QUERY PLAN` golden test so a schema
change that breaks an index fails the gate, not QA-2.

**Connection discipline.** Every connection sets the same pragma
set (`foreign_keys=ON`, `busy_timeout`, `journal_mode=WAL`,
`synchronous=NORMAL`, `cache_size`); the DSN builder is the only
place it is spelled. Checkpointing is owned by the writer:
`wal_autocheckpoint` is disabled, the writer runs a PASSIVE
checkpoint between batches, `journal_size_limit` bounds the file,
and a TRUNCATE checkpoint runs at defined idle (writer queue
empty, no reads in flight). WAL size is a measured number in the
QA-5 harness.

**Threading.** `thread_key` is the server's thread id where the
backend supplies a References-derived one; for messages the
server fails to thread, a pure JWZ References walk
(`internal/mail/thread.go`) assigns a local key. Subject-heuristic
grouping never runs on References-derived backends (TH-1); a
future backend whose server threading is itself heuristic (Gmail)
is the `ServerHeuristic` capability case, section 4. Thread rows
are derived from message threadIds; there is no Thread/changes
sync round trip.

**Store recovery (SY-8).** Startup runs `PRAGMA quick_check` plus
the FTS integrity check behind the schema-version gate. Detected
corruption or a failed migration surfaces per ER-1 and offers
rebuild-from-server, which exports and re-imports the preserved
set (undispatched outbox rows, drafts and their local revisions,
`origin='local'` messages and their bodies) into a fresh store
before resyncing. Disk-full on any write surfaces the typed
failure and never partially commits (the transaction is the
unit). Forced-corruption, failed-migration, and full-disk tests
are named in ADR-0014.

**Folder classification (FO-1).** The backend maps
server-declared roles; `internal/store`'s classification helper
applies the tested name-heuristic fallback (a committed
name-to-role table) and resolves duplicate roles by first-created
with a log line, never a sync refusal.

## 4. The backend seam

`internal/backend` defines the seam C4 requires (ADR-0004). The
review sharpened it from vocabulary to typed shapes, because the
shapes are what keep JMAP batched and Gmail real:

```go
type Backend interface {
    Capabilities() Capabilities
    Mail() MailSource
    Calendar() CalendarSource   // nil when Capabilities says so
    Contacts() ContactSource
    Push(ctx) (PushStream, error)
    Credentials() Credentials
}

// MailSource, the load-bearing shapes:
Changes(ctx, kind, token, limit) (ChangeSet, error)
    // ChangeSet: Created/Updated as hydrated model objects,
    // Destroyed as server ids, NewToken, HasMore. A JMAP
    // implementation composes changes+get with back-references
    // in ONE request; a Gmail implementation composes
    // history.list plus batch get.
FetchBodies(ctx, refs) (iter, error)
ApplyBatch(ctx, batch) (BatchResult, error)
    // batch mutations with creation-id references, so
    // create-folder-and-move dispatches as one server request
Submit(ctx, msg, lifecycle) error
MailboxLifecycle: CreateMailbox / RenameMailbox / DeleteMailbox
ServerSearch(ctx, query) (Results, error)  // SR-7, optional
```

`Capabilities` carries what the engines branch on, at the
granularity the requirements actually take: thread identity as a
three-valued fact (`None` | `ReferencesDerived` |
`ServerHeuristic`; the engine trusts server ids for grouping in
the third case and TH-1's no-false-merge criterion applies to
`ReferencesDerived` backends, recorded as a per-backend carve-out
in ADR-0004), push transport, delta granularity, server-search
support, scheduled-send support, the RSVP mechanism (which side
sends the iTIP reply; per backend, probe-resolved), server limits
(maxObjectsInGet/Set, maxCallsInRequest, maxConcurrentRequests,
maxSizeUpload, from the live session), and per-capability account
ids (the probe found contacts may live on a different JMAP
account).

`Credentials` owns token lifecycle so OAuth (horizon 2) is a
backend property, not a worker restructure: `Token(ctx)` returns
a valid credential, owning expiry, single-flight refresh, and
persistence through `internal/keyring`; workers call it per
request and never see a 401-refresh-retry. The `auth` failure
class gains the `refresh-failed` sub-reason that routes to ST-5.
For v1's static Fastmail token, `Token` is a read.

The Fastmail v1 backend composes `backend/jmap` (mail, contacts)
and `backend/dav` (calendar); composition lives in account
config. The JMAP-calendars upgrade swaps the calendar source; a
Gmail backend is a new composition behind the same seam.

## 5. Concurrency model

The full model is ADR-0003. Fixed cast: the UI loop, the writer,
one mail sync worker per account, one groupware poll worker, one
outbox dispatcher, one backfill worker. Workers post messages to
the UI through `Program.Send`; the store is the only shared
state. The sync, backfill, and dispatch workers draw from one
shared request budget derived from `Capabilities` limits
(maxConcurrentRequests is an account-wide fact, not a per-worker
assumption).

**Writer admission policy** (the review's biggest structural
addition). The writer runs two lanes: interactive (UI-originated
mutations, autosave) and bulk (sync apply, backfill, bulk-triage
chunks). Any single transaction is bounded at roughly 50 ms of
work; bulk work is chunked to fit, and the interactive lane
preempts at chunk boundaries. Backfill subordination reads the
interactive lane's recent activity, not the queue depth (which is
empty exactly when the writer is busy). Consequences stated
honestly: CO-6's loss bound is the debounce window plus the
admission ceiling, and the 50-run kill test measures that sum.

**Optimistic paint, the named mechanism** (LT-2). The root model
owns a pending-intent overlay keyed by internal message id.
`Update` applies the intent to the overlay and paints in the same
frame (the LT-2 criterion); the enqueue command commits the
mutation and its outbox row through the interactive lane; the
writer's ack message clears the overlay entry, and a write
failure reverts the overlay with an ER-1 toast (the disk-full
path). Store-changed notifications post only after commit and
carry a monotonic store revision; every read result carries the
revision it saw, and the UI discards results older than the last
one it applied, so a re-query can never visually revert an
optimistic paint. The crash window is stated: an intent
acknowledged on screen but not yet committed is lost by SIGKILL
inside the admission ceiling; the bound is small, measured, and
honest (QA-6's invariant covers everything committed).

**Store-changed coalescing.** The UI re-queries a surface at most
once per 100 ms per surface; list reads are keyset-paginated
windows (viewport plus overscan) over scalar columns only (the
`data` JSON is never parsed on the list path). Read-pool size and
per-connection `cache_size` are stated store configuration; RSS
at the QA-5 envelope is harnessed from build step 1.

## 6. Sync engine

ADR-0005. Per account and object kind, two watermarks: the server
state token and a local revision counter. The mail loop:

1. On connect and on push: `Changes` batches through the writer's
   bulk lane (hydrated objects, one server request per page).
   The local store is the list view; there is no queryChanges
   maintenance (revision 1 carried both mechanisms; the review
   struck the redundant one).
2. Push: the EventSource stream delivers StateChange; the worker
   coalesces with a fixed 200 ms delay from the first event
   (bounded, never a resetting debounce) and runs step 1. Stream
   drop falls back to polling with jittered exponential backoff
   within SY-2's recovery bound, re-establishing push
   automatically.
3. Self-echo suppression: the dispatcher records the state token
   each dispatch produces; the sync worker skips applying changes
   it originated (matching recorded tokens/ids), so an autosave
   push does not round-trip into a draft re-hydration.
4. Token expiry or `cannotCalculateChanges`: full resync, a
   normal path. Resync reconciles by `server_id` (section 3),
   preserving local-only and `origin='local'` state.
5. Bodies: eager recent window, backfill newest-first in bulk-lane
   chunks, on-demand fetch for opened messages (C1 exception).

Conflicts resolve by server state ordering, local losing ties
(SY-3), never field-merged. Calendar and contacts poll by
collection state on the SY-2 cadence with focus refresh, same
watermark table, same resync-is-normal rule.

## 7. Outbox and mutation discipline

ADR-0006, revised for claim states and batches. Every mutation is
an intent row: kind, payload, undo group. Payloads reference
poplar's internal keys only; the dispatcher resolves internal
keys to server ids at dispatch time from the current row (a
folder created offline resolves through the batch's creation-id
back-reference; a referent that no longer exists fails the intent
into `not-found`, which reconciles and reports). Every intent
kind is idempotent under replay, tested.

**Claim discipline.** The dispatcher claims an intent by moving
`queued → dispatching` inside a writer transaction before any
I/O. Undo annihilation is legal only against `queued` rows and is
decided in the same writer transaction, so the race with an
in-flight dispatch cannot exist. Undoable intents hold in
`queued` for the 10-second UX-9 window before dispatch (QA-4
measures server-to-local convergence, which this does not touch),
so annihilation is the common undo path; post-dispatch undo
issues the compensating intent as a normal mutation.

**Batches.** A bulk action (LT-3's full matching set; TH-4's
collapsed-thread action) enqueues chunked sub-intents sized under
the backend's maxObjectsInSet, sharing an `undo_group` and
ordered by `chunk_seq`; the local mutation for the whole set
commits with the first chunk (one optimistic paint), and the
compensating group restores exact prior state per message
(prior-mailbox recorded per chunk, bounded per chunk rather than
one 200 KB row). A bulk action over a search result re-runs the
criteria query uncapped at action time; the confirmation names
the true count (SR-3's 500-row display cap is a display fact).
Partial dispatch retries only unfinished chunks.

**Failure classes.** SY-4's enum gains `throttled`: retry-after-
aware backoff surfaced as SY-5's warn state, never an error
toast. `auth` carries the `refresh-failed` sub-reason (section
4). Series-split edits (CA-5, when built) are one intent carrying
both CalDAV writes with defined partial-failure reconciliation.

**Send** (CO-7) is an intent with the hold state; cancel during
the hold restores compose; exit persists and dispatches next
launch.

**Drafts (CO-6, ADR-0007, revised).** The local store is the edit
buffer; autosave is local on the 1-second debounce. Poplar mints
a stable anchor Message-ID header at draft creation
(`draft_meta.anchor_msgid`), preserved byte-identically across
every server replacement, so resync re-anchors drafts even
though the server id rotates. Server push (close + 5 idle
minutes) uses the probe-verified atomic create-and-destroy;
success is judged by the returned created id, never absence of
error. `pushed_rev` records which local revision was pushed
(`dirty` is the computed comparison, and a push that raced typing
leaves later revisions correctly unpushed with no wall-clock
reasoning). A `notFound` on the destroy half is the
someone-else-replaced-it branch: reconcile against the server
draft carrying the anchor, never create a duplicate.

## 8. Calendar subsystem

ADR-0009 (engine), ADR-0010 (libraries). The event model is
poplar's own, JSCalendar-shaped in its queryable layer, not only
its prose (the review caught revision 1 claiming JSCalendar while
storing RFC 5545 UTC instants): `start_local` plus `tzid` plus
`duration_secs` plus `is_all_day` plus `is_floating` on the event
row; occurrences carry UTC instants for window sorting plus
`start_local` and `local_date` so the agenda groups by local
calendar date and all-day events render on their date in every
zone (CA-1's fixtures cover both). An unmappable TZID floats
(`is_floating`) with the visible notice. The raw ICS blob is
retained; edits round-trip the parsed component and rewrite only
the CA-4 modeled properties, preserving unknown properties
verbatim. The JMAP upgrade delta is named in ADR-0009's
consequences so "thinner mapping" is checkable.

- **Transport**: `backend/dav` on go-webdav/caldav; 412 refetches
  and surfaces the named conflict.
- **Recurrence**: RRULE expansion (fork bake-off per ADR-0010)
  with poplar-owned override splicing into the occurrence index.
  The window (13 months back, 18 forward) is a cache with a miss
  path: navigation or date-jump outside it triggers a bounded
  on-demand expansion for the target range with a visible
  progress state; the window boundary is a named UI state, never
  an empty day (C7). The slide runs in the background worker's
  bulk lane, never on startup. Out-of-window RDATEs are captured
  by the miss path; a tzdata update invalidates and re-expands.
- **Timezones**: IANA match → vendored CLDR Windows-zone table →
  float with notice; `time/tzdata` embedded.
- **iTIP/iMIP**: `internal/calendar/itip` hand-rolls
  REQUEST/REPLY/CANCEL with SEQUENCE discipline; outbound iTIP
  mail rides the outbox.
- **RSVP (CA-6)**: the invite card renders from the parsed
  `text/calendar` part only. The answer mechanism is a
  `Capabilities` fact (section 4): server-sends-reply or
  poplar-sends-iMIP, resolved by the pending calendar-scoped
  probe (requirements section 16 item 1), with the double-send
  fixture guarding the losing branch.
- **Reminders (CA-7)**: VALARM data persists; an in-process
  ticker over the occurrence index raises the banner; launch runs
  the missed-reminder sweep (trailing 12 hours surface once as a
  missed list; older drop with a log line). No daemon (C3).

## 9. Rendering pipeline

ADR-0008. Deterministic end to end (C2, QA-7):

```
raw MIME bytes (retained per section 3)
  → decode      enmime parse; charset repair (a recognized-but-
                unimplemented ianaindex charset falls to the
                documented default with the RD-4 visible notice);
                format=flowed and delsp un-flowing (un-stuff,
                rejoin soft-wrapped lines) before any downstream
                stage; defect accumulation
  → plan        pick the part chain, extract search_text, record
                the choice
  → parse       x/net/html DOM for HTML parts
  → rules       ordered named rules; every firing traced
  → doc         the intermediate document model
  → check       fact inventory; missing facts downgrade honestly
  → emit        markdown (glamour), filtered plain text, or raw
```

Inbound un-flowing is on the build-it-ourselves list: without it,
flowed mail from mutt/aerc/Thunderbird correspondents renders
with spurious breaks and space-stuffing corrupts quote depth,
which lands in QA-9's list-patch class (the review caught
revision 1 generalizing the outbound flowed drop to the inbound
obligation).

The pipeline is a pure function of (raw bytes, RenderContext,
resolved remote resources). RenderContext is {width, height,
color profile, capability profile, locale, tz}; resolved
resources are the explicitly loaded remote images (RD-6), fetched
outside the pure boundary (the injected-dialer test enforces
that) and passed in as content. Link targets are resolved at the
doc-model stage into RD-5's numbered references; glamour's OSC 8
hyperlink emission is disabled, so the reader buffer and RD-16's
copy-out carry no terminal escapes in yanked text. Syntax
highlighting uses the fence's explicit language token only;
chroma's content-based `Analyse` never runs (map-iteration
tie-breaks are a determinism hazard inside a QA-7 MUST). The
pinned render dependency set (enmime, goldmark, glamour,
lipgloss, chroma) is part of the QA-7 contract: a bump
regenerates goldens and triggers a QA-9 regrade.

Captured specimens (the improve loop, horizon 3 and 4) live
under `$XDG_STATE_HOME/poplar/corpus/`, raw bytes plus a sidecar
recording capture time and the rule-set version, because traces
are recomputed and a specimen's evidence must not drift silently.
The encryption seam (CO-11) is a boundary statement: decryption
produces the raw bytes the pure pipeline consumes (before stage
one, not a stage); `internal/mail`'s decode entry point takes
bytes and is callable on an inner MIME entity (re-entrant for
enveloped parts); whether an encrypted message's extracted text
enters `search_text` is the reserved CO-11 policy fork (cache
decrypted or not at all, with the FTS-coverage consequence
named); an encrypt-on-send compose suppresses server draft push.

## 10. Outgoing mail

(New in revision 2; the compliance review found the compose path
had no design home.) `internal/mail` owns three pure subsystems
plus assembly, recorded in ADR-0016:

**Reply seeding (CO-2).** A pure function from (source message,
mode, user identities) to a compose seed: recipient math with
dedup and own-identity removal, depth-preserving quote markers
over the rendered doc model, In-Reply-To/References chains,
non-doubling Re:/Fwd: prefixes, attachment carry-through on
forward with per-attachment drop. The CO-2 fixture matrix tests
the function directly.

**Identity matching (CO-3).** The live probe settled the shape:
Fastmail's Identity list is curated, so the matcher is
poplar-owned: exact match on delivered-to, then alias-pattern
match (user-defined wildcard patterns in account config), then
domain-suffix match, then the account default; per-recipient
last-identity memory (SHOULD) rides `sent_history`. Signature
materialization and the byte-identical swap rule live here;
materialization calls Catkin's buffer-mutation API (section 11)
and lands as one undo entry.

**Assembly (CO-5).** One markdown source through goldmark's AST
to: a text/plain part wrapped at 72 columns fixed (format=flowed
dropped, per the survey), with no-reflow protection for fenced
regions and for unfenced diff-shaped runs (unified-diff and hunk
headers, leading +/-/space lines), because patch mail is normally
unfenced (the review caught the fence-only rule destroying the
exact workflow the text-only toggle exists for); and a
conservative HTML part restricted to the committed allowlist
artifact (`2026-07-27-poplar-html-allowlist.md`), inline styles
only. The MIME tree golden, the allowlist validation, and the
plain-part byte goldens are CO-5's tests. The multipart builder
is go-message; a text-only toggle emits no HTML part.

## 11. Search

`internal/search` compiles the SR-2 grammar to FTS5 MATCH plus
scalar predicates. The parser retains its source string and
re-parses on load, so a stored query (SR-6) is a string, robust
across grammar evolution via the fall-through rule. A query with
no indexable positive term (pure `is:unread`, negation-only)
executes as a bounded scalar scan over the indexed columns with a
documented cap and a visible notice, never a silent truncation
(QA-3). Search-as-you-type: minimum two characters, one dedicated
search connection (never the shared read pool, so list reads
cannot queue behind FTS scans), superseded queries are cancelled
(context cancellation wired to SQLite's interrupt), and
generation numbers guard result application (SR-3 raced in
test). The 500-row cap is display-level; bulk-over-results
re-runs uncapped (section 7). Server-side fallback (SR-7) goes
through the seam's `ServerSearch`, merged and labeled by source.
`internal/when` (the shared time parser) is deliberately
hand-rolled: the survey did not cover NL date parsing (a C9 gap
the review flagged), and the maintained candidates (olebedev/
when, araddon/dateparse) trade determinism and en-only
predictability for breadth poplar does not need; the ruling and
alternatives are recorded in ADR-0016's appendix.

## 12. TUI architecture

ADR-0011, ADR-0012; the vocabulary lives in the design language.
Structural decisions, revised where the review found gaps:

- **bubbletea v2**, one root model, package-level screen
  registry; keymaps are registry data deriving footer, help,
  grammar test, and switch-table test.
- **The list is poplar's own windowed model.** bubbles/list
  holds all items in memory and filters in-process, which does
  not survive the QA-5 envelope; poplar's list model reads
  keyset-paginated windows through the read pool. bubbles
  components are used where their contracts fit (viewport,
  spinner, textinput as a base); the ruling is in ADR-0011.
- **huh is ruled out** for v1 forms: the registry-derived footer
  (UX-2) and the UX-8 leave-field model are load-bearing
  constraints a third-party form engine does not satisfy; forms
  are the design language's form component over shared
  focus-management. Recorded in ADR-0011.
- **Kitty keyboard enhancements are declined** (ADR-0012,
  reasoning corrected per review: the honest ground is one input
  regime everywhere and C11's lean half, not Esc handling; the
  pre-planned relief valve is enabling disambiguation-only if
  leave-field shows ambiguity defects on a gate terminal).
- **Capability profile resolver**: the runtime profile (truecolor
  / ANSI-16 / NO_COLOR) resolves from NO_COLOR, TERM, COLORTERM,
  and the background-color query, with an ST-3 config override,
  and the config surface reports what was detected. QA-7's tests
  take profiles as inputs; this resolver is runtime-only. The
  first frame does not wait on the terminal: a 100 ms bounded
  wait for `tea.BackgroundColorMsg`, then the default dark theme
  renders and a later answer repaints (the repaint is a golden
  input, not a flash bug).
- **Account-scoped UI state** uses a shared wrapper type
  (`AccountScoped[T]`) so the registry reflection test asserts
  the C4 UI half mechanically.
- **Catkin's contract** (revised; the review found the zero-
  import rule colliding with the UX-3 analyzer): catkin defines
  its own style-parameter struct; poplar injects values derived
  from `internal/theme`; `internal/catkin` is a recorded
  UX-3-analyzer exemption in the design language. Catkin exposes
  a buffer-mutation API (used by signature materialization now,
  AI tidy later) that lands each external mutation as one entry
  in its buffer-scoped undo. Reader rendering through glamour
  v2 with theme-driven styles; Catkin renders via its own
  goldmark-AST incremental renderer.
- **teatest** is adopted with its experimental status carried as
  a named risk (section 17): goldens are plain files, so a
  harness swap is mechanical.

## 13. Onboarding, config, credentials

- **Config** (ST-3): the struct splits a global section and an
  `[]Account` section (identities, signatures, default calendar,
  default reminders are per-account facts; the reflection test
  walks both). Persisted as a poplar-written TOML file; unknown
  keys warn by name; no supported behavior requires hand
  editing.
- **Credentials** (ST-1, ST-5): keyring entries are per-account
  (`poplar/<account-slug>`); the fallback is a per-account 0600
  file under the account's state directory with the visible
  notice. Credential acquisition is asynchronous and off the
  first-frame path: the store renders before credentials
  resolve; sync starts when the token arrives; a keyring that
  never answers surfaces the named ST-5-adjacent state (QA-1 is
  unaffected by a locked keyring).
- **First run** (ST-1): token form → live probe → store → initial
  sync with progress while interactive. OAuth later arrives as a
  backend `Credentials` implementation plus a browser-redirect
  step; the loopback listener is transient and user-initiated.

## 14. Errors, logging, observability

ADR-0013. `internal/uerr`: construction is the surfacing event;
the exported constructor takes (operation, ids, typed reason,
wrapped error), writes the log line, returns the view value.
Retry loops do not construct per attempt: a failure surfaces on
state transitions (first failure, class change, recovery), which
preserves both ER-1's one-line-per-outcome oracle and the log's
signal. The analyzer gates construction outside the package.
slog JSON to the state-dir file, hand-rolled size rotation,
structural redaction (no body field exists on log value types;
addresses and subjects debug-only; credentials never). ER-3's
honesty states are UI presentations backed by the same seam.

## 15. Testing strategy

ADR-0014, extended per review. As revision 1, plus: the three
SY-8 tests (forced corruption, failed migration, full disk); the
FTS integrity check inside QA-6 and the SR-1 script; EXPLAIN
QUERY PLAN goldens for the hot queries; the QA-10 artifacts
(conventions gates in CI from the first build pass; `internal/ui`
package documentation; README and architecture map at 1.0) named
as gate outputs; and idempotent-replay tests per intent kind.

## 16. Platform posture

The gate platform is Linux; macOS builds, tests, and degrades by
name (keyring, notifications). **Clipboard** (revised per
review): RD-16 names a system-clipboard path with OSC 52 for
remote sessions, and revision 1 collapsed both into OSC 52. The
design now carries both: golang.design/x/clipboard (cgo-free
since v0.8.0, in-process X11/Wayland, C9-verified in the survey)
as the local path, adopted if the named gate-box spike passes
(raw-mode coexistence, compositor coverage), with OSC 52 as the
remote and fallback path. OSC 52 is fire-and-forget, so its
posture is stated: copies report as attempted, the first OSC 52
copy per session emits a one-time hint naming the tmux
passthrough requirement, and the size ceiling degrades by name
with the byte count. **RD-6's image-load key** is capability-
gated: advertised only where the terminal renders graphics
(kitty protocol probe); elsewhere the placeholder names the
blocked count and open-in-browser is the path, recorded in the
design language's footer exceptions. **Opener temp files**:
0600 in `$XDG_RUNTIME_DIR` (fallback `os.TempDir()`), removed on
exit and by signal-handler cleanup, per RD-3. **Instance lock**:
gofrs/flock on a lock file beside the store; the holder writes
its pid into the file after acquiring (flock itself carries no
metadata); a second instance refuses with the actionable message
naming that pid (ADR-0015).

## 17. Horizon, LATER, and C11 audit

Both registers, per the review (revision 1 audited only the
vision's seven).

**Vision horizon:**

1. **Multi-account**: one store file with `account_id` isolation
   (section 3), per-account config section and keyring entries
   (section 13), `AccountScoped[T]` UI state with its reflection
   test (section 12), per-account worker cast. v1's UI is
   one-account-at-a-time; a unified inbox is not foreclosed (an
   account-predicate change on the same store). Open.
2. **Gmail backend**: operation-shaped seam with typed batch
   shapes, three-valued thread capability, `Credentials` refresh
   seam (section 4). Open.
3. **Flag-a-bad-render**: raw retention (defined form, section
   3), fired-rule traces, the corpus directory with rule-set-
   versioned sidecars (section 9). Open.
4. **Capture-mailbox corpus**: `mailbox.visible` exists from
   migration one. Open.
5. **Catkin spinoff**: zero inbound imports plus the style-
   injection and buffer-mutation contracts that keep the
   analyzer and CO-3 from forcing a coupling (section 12). Open.
6. **Encryption seams**: the boundary statement in section 9
   (decrypt-before-pipeline, re-entrant decode, the search_text
   policy fork, draft-push suppression). Open, with the CO-11
   fork named rather than hidden.
7. **Contacts micro-highlight**: per-letter cursor state in the
   contacts list model (CT-3). Open.

**LATER register:** ST-4 import — open via `message.origin`
(revision 1 foreclosed it; fixed). LT-7 snooze/mute — open via
the reserved scalar columns (the JSON reservation was unusable as
a hot-path predicate). SR-6 — open via source-string queries.
FO-5 labels — open via `message_mailbox` many-to-many. CO-2
forward-as-attachment — open via raw retention. CO-10 — open via
Catkin's buffer-mutation API. CO-11 — section 9. CT-4 contact
writes — open (ContactCard `data` carries the RFC 9553 shape; the
seam adds a mutate method later). CA-5 — SHOULD, served by the
occurrence design plus the single series-split intent. CA-12 —
open (`transparency`, attendees in `data`, raw REPORTs named).

**C11.** Forward half: JMAP-native sync with push, the
JSCalendar-shaped queryable model (now actually in the columns),
bubbletea v2/lipgloss v2, capability profiles with a named
runtime resolver. Lean half: no mailcap, no plugin surface, no
per-folder goroutines, no second cache store, no library where
three hundred traceable lines suffice, and the kitty-enhancement
decline (filed here, not as a forward bet: it declines machinery
no requirement needs, with its relief valve named). The `thread`
table earns its row as the mute flag's home; the snippet column
from revision 1 is gone (its job is done by `search_text` and
LT-1 has no preview line).

## 18. Risks and open items for the gate

1. **The measurement spike** (running) replaces the provisional
   QA-1/2/3 numbers; blocking input to Phase 5 planning. Relief
   valves if it disappoints: statement caching, and the
   `search_text` column already avoids body-table reads on hot
   paths.
2. **CalDAV RSVP and free/busy probes**: blocked on the
   calendar-scoped token (requirements section 16 items 1 and
   2). Both design branches exist; the capability flag makes the
   probe result a configuration, not a redesign. The gate is
   asked to accept this deferral with poplar-sends-iMIP as the
   default branch if the probe stays blocked into Phase 5.
3. **EventSource auth check**: probe dispatched this session
   (results land in the survey addendum); the 2021 401 report is
   the risk.
4. **iCalendar bake-off and rrule DST fixtures**: run before the
   calendar build pass (ADR-0010 names the fallback); the gate
   is asked to accept the deferral with the defaults named.
5. **Clipboard spike** on the gate box decides the in-process
   path (section 16); OSC 52 posture ships regardless.
6. **teatest is experimental**; goldens are plain files, harness
   swap is mechanical.
7. **The analyzer set** (import-boundary, write-call, styling,
   error-construction) is four checks poplar maintains; the
   first two are expressible as go-list/grep-class tests, only
   the styling and error-construction checks need AST passes.
   Phase 5's build machine owns them.
8. **CA-3 grid views**: the named gate taste call (agenda MUST
   is designed; the gate rules whether any grid view blocks v1).
9. **UX-3 analyzer scope**: the design language strengthens the
   rule to all non-ASCII rune/string literals outside `theme`
   (the requirement's block list missed four of the committed
   glyphs); the gate ratifies the stricter rule.
