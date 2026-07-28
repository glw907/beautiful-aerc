# Poplar technical design

**Date:** 2026-07-27
**Status:** Draft for the Phase 4 gate.
**Charter:** `2026-07-19-poplar-refounding-charter.md`
**Requirements:** `2026-07-27-poplar-requirements.md` (binding; its
constraints C1-C11 govern every decision here).
**Survey:** `docs/poplar/research/2026-07-27-phase4-library-survey.md`
(the C9 evidence base; versions cited there are not repeated here).
**ADRs:** `docs/superpowers/specs/adr/` holds one record per major
decision with the alternatives considered. This document carries the
connective architecture; where a decision needs its reasoning trail,
the ADR is the record.

The design describes the system Phase 5 builds. It is judged against
the knowable-horizon register (vision) and C11's two-part test: bet
on where the field is going, and carry nothing daily use does not
earn.

## 1. Shape of the system

Poplar is one process, one binary, three layers:

1. **The store** (`internal/store`): one SQLite database per account
   holding mail, calendar, contacts, outbox, sync state, and the
   full-text index. The store is the truth the UI reads (SY-1). One
   writer goroutine owns every write; read connections serve the UI.
2. **The engines** (`internal/sync`, `internal/outbox`,
   `internal/render`, `internal/calendar`, `internal/contacts`,
   `internal/search`): background workers that keep the store
   current and pure functions that transform its contents. Engines
   never touch the terminal.
3. **The UI** (`internal/ui`, `internal/theme`, `internal/catkin`):
   a bubbletea v2 program. The UI reads the store through commands,
   mutates only by enqueueing intents, and hears about the world
   through messages. It performs no network I/O and no store writes.

Data flows in one loop: server → sync engine → store → UI, and
user intent → store (optimistic local mutation) + outbox → dispatch
→ server → sync engine confirms → store. The UI never waits on the
network (C1); the four enumerated exceptions (on-demand body,
attachment fetch, explicit image load, server search) are explicit
async commands with visible progress states.

## 2. Package boundaries

```
cmd/poplar             wiring and main
internal/store         schema, migrations, reads, the writer, FTS
internal/backend       the seam: Backend interface + capabilities
internal/backend/jmap  Fastmail JMAP backend (go-jmap)
internal/backend/dav   CalDAV calendar transport (go-webdav)
internal/sync          delta orchestration, watermarks, push, backoff
internal/outbox        durable intent queue, dispatch, typed failures
internal/mail          MIME parse (enmime) and assembly (go-message)
internal/render        the rule engine, pipeline, fact check, traces
internal/calendar      event model, recurrence, iTIP, occurrence index
internal/contacts      ContactCard model, autocomplete ranking
internal/search        query grammar, FTS query build, merge
internal/when          the shared natural-language time parser (C6)
internal/uerr          the error seam (ER-1)
internal/config        the config struct, load/persist, migration
internal/keyring       token storage with the named fallback
internal/platform      opener, clipboard, notifications, instance lock
internal/theme         compiled tokens: colors, glyphs, spacing roles
internal/ui            root model, screen registry, screens
internal/catkin        the live-markdown editor; no poplar imports
```

Dependency rules, enforced by an import-boundary test in the gate:
`ui` imports `store` (read API), `theme`, `catkin`, `uerr`, and the
intent types; it never imports `backend`, `sync`, or `outbox`
directly. `catkin` imports nothing from poplar (the spinoff stays
free, horizon 5). `render`, `when`, `search`, `calendar` logic
packages are pure: no I/O, no store handles, fully testable in
isolation. `backend` implementations are the only packages that
speak a wire protocol.

## 3. Data model and store schema

One SQLite file per account at
`$XDG_DATA_HOME/poplar/<account-slug>/store.db`, WAL mode. The
engine, schema shape, and concurrency discipline are ADR-0001 and
ADR-0002; the account seam is ADR-0004. The load-bearing choices:

**Fat-table hybrid.** Every entity table carries a `data` JSON
column holding the full serialized model plus scalar columns for
exactly the fields queries touch (sort keys, filters, foreign keys).
New model fields land in JSON without migration; only new queryable
fields alter schema. This is Mailspring's proven shape and it keeps
the migration count low for the life of the product.

**Poplar mints every primary key.** Internal 64-bit integer keys on
every row; server identifiers (JMAP Email id, blobId, CalDAV href,
ETag) are indexed side columns, replaceable wholesale on resync.
Thunderbird's Panorama re-architecture is the cautionary record for
keying on server ids.

**Account identity on every scoped row.** Every account-scoped table
carries `account_id` (FK to `account`) from the first migration, and
every query predicate includes it, even while v1 ships one account.
UI state (cursor, scroll, search scope) is keyed by account in the
UI's state types. C4's schema review checks both; a test asserts no
account-scoped table lacks the column.

Core tables (scalar columns abbreviated):

```
account        id, slug, backend_kind, address, data
mailbox        id, account_id, server_id, role, name, sort_order,
               unread_count, total_count, data
message        id, account_id, server_id, blob_id, thread_key,
               received_at, subject, from_addr, flags, size,
               has_attachment, snippet, data
message_mailbox  message_id, mailbox_id          -- multi-membership
                                                 -- (FO-5 stays open)
body           message_id, kind (text|html|raw), content BLOB,
               fetched_at                        -- raw MIME retained
                                                 -- per RD-1
message_fts    FTS5, external content over subject + body text
thread         id, account_id, server_thread_id, data
calendar       id, account_id, href, name, color_slot, visible,
               is_default, data
event          id, account_id, calendar_id, uid, href, etag,
               raw_ics BLOB, summary, dtstart_utc, dtend_utc,
               is_recurring, transparency, sequence, data
occurrence     event_id, start_utc, end_utc, recurrence_id
               -- the expanded window index, rebuildable
contact_card   id, account_id, server_id, uid, full_name, data
contact_email  contact_card_id, address, rank_hint
sent_history   address, name, last_used_at, use_count
               -- CO-4 ranking, noreply-filtered on insert
outbox         id, account_id, kind, payload JSON, state,
               attempt_count, next_attempt_at, failure_class,
               failure_detail, created_at
draft_meta     message_id, local_rev, server_pushed_at, dirty
sync_state     account_id, object_kind, server_state_token,
               local_rev                          -- two watermarks
schema_version singleton
```

**Threading.** `thread_key` is the server's thread id where the
backend supplies one; for messages the server fails to thread, a
pure JWZ References walk (`internal/mail/thread.go`, no I/O)
assigns a local key. Subject-heuristic merging never runs (TH-1).
Thread views query by `thread_key` across all mailboxes, which
satisfies TH-2 (Sent and Archive members) by construction.

**FTS discipline.** `message_fts` is external-content FTS5 over the
message table; index maintenance statements execute inside the same
transaction as every message mutation, so SR-1's atomicity is
transactional, not eventual. The index is derived state: `poplar
--rebuild-index` (and SY-8 recovery) regenerates it from `body`
rows; index corruption is a repair, never data loss.

**Snooze/label seams (LT-7, FO-5).** `message.data` reserves
`hiddenUntil`; `message_mailbox` is many-to-many; keyword flags are
a bitfield plus JSON overflow. All three LATER items stay open at
zero carried cost.

## 4. The backend seam

`internal/backend` defines the seam C4 requires and the Gmail
horizon needs (ADR-0004):

```go
type Backend interface {
    Capabilities() Capabilities
    Mail() MailSource        // list/delta/fetch/mutate/submit
    Calendar() CalendarSource // may be nil: Capabilities says so
    Contacts() ContactSource
    Push(ctx) (PushStream, error) // may degrade to polling
}
```

The seam is operation-shaped, not protocol-shaped: `MailSource`
speaks in poplar's model types (changes-since-token, fetch-bodies,
apply-mutation, submit-with-lifecycle), never in JMAP method names.
`Capabilities` carries the facts the engines branch on: server
thread identity (yes for JMAP, no for a future IMAP-shaped source),
push transport, submission semantics, delta granularity. The
Fastmail v1 backend composes `backend/jmap` (mail, contacts) and
`backend/dav` (calendar); the composition point is the account
config, so a future Gmail backend (OAuth, different calendar
source) is a new implementation behind the same seam, and the JMAP
calendar upgrade (section 8) swaps `backend/dav` for a
`backend/jmap` calendar source without touching the calendar engine.

## 5. Concurrency model

The full model is ADR-0003. Fixed cast, no dynamic goroutine
spawning per folder or per request:

- **The UI loop**: bubbletea's event loop. `Update` never blocks:
  store reads run as `tea.Cmd` on the reader pool; mutations post
  intents and return optimistically.
- **The writer**: one goroutine in `internal/store` owning the
  single write connection. All writes arrive as functions over a
  channel; each executes in a transaction and answers on a reply
  channel. UI-originated optimistic mutations and their outbox rows
  commit together here (SY-4's shared transaction). No other
  package holds write access; the store's write API is unexported
  outside the writer's intake, and a vet-class check in the Phase 5
  gate keeps it that way (the QA-2 "test fails on a UI-thread
  write" mechanism).
- **The mail sync worker**: one goroutine per account. Runs the
  delta loop (section 6), listens to the push stream, owns backoff.
- **The groupware poll worker**: one goroutine polling calendar and
  contacts by collection state on the SY-2 cadence, plus
  focus-triggered refresh.
- **The outbox dispatcher**: one goroutine draining the outbox in
  order, classifying failures, scheduling retries.
- **The backfill worker**: one goroutine fetching older bodies,
  newest first, in bounded batches, pausing whenever interactive
  work is pending (SY-5's subordination is a priority check against
  the writer's queue depth and recent UI read activity).

Workers communicate with the UI only by posting messages through
`Program.Send` (store-changed notifications carry table + account
granularity; the UI re-queries what it shows). Workers never share
memory with the UI; the store is the only shared state, and the
writer serializes it. `testing/synctest` drives the whole cast in
tests with virtualized time.

## 6. Sync engine

ADR-0005. Per account, per object kind, two watermarks: the server
state token and a local revision counter (`sync_state`). The mail
loop:

1. On connect: `changes-since(token)` per object kind (Mailbox,
   Email, Thread), applied in batches through the writer;
   `queryChanges` maintains the live list views.
2. Push: the EventSource stream (go-jmap `core/push`) delivers
   StateChange; the worker coalesces bursts (200ms window) and runs
   step 1. Stream drop falls back to polling with exponential
   backoff + jitter, capped per SY-2's 30s p95 recovery, and
   re-establishes push automatically.
3. `cannotCalculateChanges` or token expiry: full resync, a normal
   path, never an error. Resync rebuilds server-derived state and
   preserves local-only state (bodies, outbox, drafts' local
   revisions) by matching on server ids where they survive and by
   re-anchoring on Message-ID where they do not.
4. Bodies: eager for recent mail (initial-sync window: newest N
   days or M messages, config default), backfill for the rest
   (SY-6). A message opened before its body arrives triggers an
   on-demand fetch (C1 exception) with the 100ms progress
   indicator and named timeout path.

Conflicts resolve by server state ordering: the engine applies
server changes as they arrive; a local optimistic mutation whose
outbox dispatch later fails against changed server state is
reconciled by re-reading the server object, local losing ties
(SY-3), with an ER-1 trace and, where the user saw the state, a
toast. Because JMAP cannot say what changed, only that something
did, poplar never field-merges (the mujmap lesson).

Calendar and contacts follow the simpler poll shape: ctag/
sync-token compare, fetch-changed-by-href with ETag, RFC 9610
`/changes` where Fastmail provides it for ContactCard. Same
watermark table, same resync-is-normal rule.

## 7. Outbox and mutation discipline

ADR-0006. Every mutation is an intent record: kind (flag, move,
archive, delete, send, rsvp, event-create/edit/delete, folder op),
payload, and the compensating intent for undo. Enqueue writes the
optimistic local mutation and the outbox row in one transaction.
The dispatcher executes intents in order per account, maps results,
and classifies failures into SY-4's typed reasons: `auth` routes to
ST-5 re-authentication with the queue preserved; `connection`
retries with backoff; `not-found` reconciles against server state
and reports; `server` surfaces with the typed detail. Undo (UX-9)
enqueues the compensating intent; if the original has not
dispatched, the two annihilate in the queue; if it has, the
compensation dispatches normally. Send (CO-7) is an outbox intent
with a 10-second hold state; cancel during the hold restores the
compose buffer; exit persists the hold and dispatches on next
launch. QA-6's kill harness asserts the invariant this section
exists for: no committed mutation without its intent row and no
intent row without its mutation, at every kill point.

**Drafts (CO-6, ADR-0007).** The local store is the edit buffer:
autosave writes `body` + `draft_meta` on the 1-second debounce,
local-only. Server push (close + 5-minute idle) uses the probe-
verified atomic pattern: one `Email/set` creating the replacement
draft and destroying the predecessor, because Fastmail silently
no-ops in-place body updates (survey, live probe). The push
records the new server id; send deletes draft and dispatches in
one intent.

## 8. Calendar subsystem

ADR-0009 (engine), ADR-0010 (libraries). The event model is
poplar's own, shaped toward JSCalendar's vocabulary (start,
duration, recurrence rules as structured overrides, participants
keyed by address) so the JMAP-calendars upgrade is a backend swap
plus a thinner mapping, not a remodel (C11). The raw ICS blob is
retained per event; poplar edits by round-tripping the parsed
component set and rewriting only the properties it models (CA-4's
modeled set), preserving unknown properties verbatim.

- **Transport**: `backend/dav` on go-webdav/caldav: discovery,
  sync-collection, conditional PUT. A 412 refetches, surfaces the
  CA-1 named conflict, and never overwrites the remote version.
- **Recurrence**: RRULE expansion via the rrule library (survey
  bake-off; the fork with DST fixes is the default candidate) with
  poplar-owned EXDATE/RDATE/RECURRENCE-ID override splicing into
  the `occurrence` window index. The index covers a sliding window
  (default: 13 months back, 18 forward), rebuilt incrementally on
  event change; agenda and grid views read it exclusively.
- **Timezones**: TZID resolution order: IANA name match → vendored
  CLDR Windows-zone table (version-pinned, regenerated by script)
  → float to local time with the CA-1 visible notice. `time/tzdata`
  is embedded.
- **iTIP/iMIP**: `internal/calendar/itip` is hand-rolled (no Go
  prior art; survey): parse REQUEST/REPLY/CANCEL from
  `text/calendar` parts into the event model; construct REPLY with
  correct PARTSTAT and REQUEST for organizer flows; SEQUENCE
  discipline per RFC 5546. Outbound iTIP mail is MIME-assembled by
  `internal/mail` and queued through the outbox like any send.
- **RSVP (CA-6)**: the reader's invite card renders from the
  parsed `text/calendar` part exclusively. The answer path has two
  mechanisms; the pending Fastmail probe (section 16 item 1) picks:
  if a CalDAV PARTSTAT write triggers Fastmail's server-side iTIP
  reply, poplar writes PARTSTAT only; otherwise poplar writes
  PARTSTAT and sends the iMIP reply itself. The design carries
  both; the probe result is recorded in ADR-0009 before the build,
  and the double-send fixture guards whichever branch loses.
- **Reminders (CA-7)**: VALARM data persists; an in-process ticker
  over the occurrence index raises the non-focus-stealing banner.
  No daemon exists or will (C3; ruled non-goal).

## 9. Rendering pipeline

ADR-0008. The Phase 1 architecture productized, deterministic end
to end (C2, QA-7):

```
raw MIME bytes                        (retained verbatim in store)
  → decode      internal/mail: enmime parse, charset repair,
                defect accumulation
  → plan        pick the best part chain (text/calendar first for
                invites, text/html, text/plain), record the choice
  → parse       x/net/html DOM for HTML parts
  → rules       the engine: ordered, named rules over the DOM and
                the intermediate doc; each rule = name, observable
                trigger, transform, provenance refs, tests; the
                trace records every firing
  → doc         poplar's intermediate document model (blocks,
                inlines, links, code, quotes, tables, attachments)
  → check       fact inventory: deterministic extractors pull
                links, amounts, dates, codes from the source;
                verify presence in doc; failure downgrades the
                render honestly (RD-2)
  → emit        markdown for the reader (glamour renders it), or
                filtered plain text (fallback), or raw source
```

The rule engine owns the email diseases the survey confirmed have
no library cure: layout-table linearization (scored on cell
density, link ratio, `role="presentation"`, nesting depth) and
hidden-content elision (a narrow inline-style visibility parser:
`display:none`, `visibility:hidden`, `mso-hide`, zero font sizes).
html-to-markdown/v2 serves as the baseline comparison in the
improve loop and as prior art, not as a runtime dependency of the
primary path; the degenerate fallback ("just render it") is the
plan stage choosing filtered plain text. Quote folding (RD-12)
and link numbering (RD-5, with the committed tracking-strip list)
are doc-model transforms with fixtures.

Determinism: the pipeline is a pure function of (raw bytes,
RenderContext{width, height, color profile, capability profile,
locale, tz}). Profiles are declared test inputs, never sniffed in
tests (QA-7). The fired-rule trace is queryable for any rendered
message (RD-1), and the raw source is always reachable (the
flag-loop seam, horizon 3, and RD-14's corpus seam ride on it).
The offline improve harness (RD-15) ships in-repo: specimen in,
graded diff out, new named rule with regression test as the
documented workflow, reproducing the Phase 1 corrective round.

## 10. Search

`internal/search` compiles the SR-2 grammar to FTS5 MATCH plus
scalar predicates over the message table (operator-filtered
queries join both). One parser serves mail and calendar (CA-10
maps applicable operators onto event columns). The fall-through
rule (unknown `key:value` widens to a bare term) and the
malformed-query degrade (bare-term search with a visible notice)
are parser behaviors with a grammar-corpus test. Search-as-you-type
issues a query per keystroke on the reader pool; each carries a
generation number, and a result only lands if its generation is
current (SR-3's race criterion). The 500-row cap and more-results
state are query-level. Server-side fallback during partial
coverage (SR-7) goes through the backend seam as an explicit
command, results merged and labeled by source.

## 11. TUI architecture

ADR-0011, ADR-0012; the visual and interaction vocabulary lives in
the design-language artifact (`2026-07-27-poplar-design-language.md`,
the UX-3 deliverable). The structural decisions:

- **bubbletea v2** with the declarative `tea.View`. One root model
  owns: the active surface, the screen stack, global chrome
  (footer, status line, toasts, reminder banner), and the account-
  keyed UI state. Screens are child models implementing a
  `Screen` interface and registering in the package-level registry
  at init; UX-1's reflection test walks `internal/ui/...` for
  unregistered screen types, and the grammar test iterates the
  registry. Keymaps are data (`theme`-independent), derived into
  the footer and the help overlay from the same registry entries,
  so neither can drift (UX-2, UX-5).
- **Kitty keyboard enhancements are declined.** Poplar binds
  modifier-free single keys (C8); enabling disambiguation would
  change nothing poplar uses and would fork input behavior across
  terminals. ADR-0012 records this as a deliberate capability
  refusal.
- **Components own their size contract** (the bubbles/glamour
  pattern): parents pass width/height down; children render to
  fit; no parent-side defensive clipping. Idiomatic-bubbletea is a
  standing project rule and the elm-conventions skill enforces the
  update/view discipline in the gate.
- **The reader** renders the pipeline's markdown through glamour
  v2, styles driven from `internal/theme` (including the chroma
  formatter for RD-7). **Catkin** is its own bubbletea model over
  a goldmark-AST incremental renderer (survey: glamour's whole-
  document render does not fit per-keystroke editing); it exposes
  the CO-12 contract (vim-idiom motions, buffer-scoped undo,
  bracketed paste byte-fidelity, fence-aware soft wrap) and knows
  nothing of mail.
- **Text entry** follows the UX-8 single model: printable keys are
  input; the leave-field verb exits to the context's command
  state; message-level verbs live in the command state. The model
  is specified once in the design language and implemented once as
  a shared focus-management helper.
- **The theme package** compiles all visual tokens as Go values
  (C5): semantic color roles as functions of `isDark` (lipgloss v2
  has no auto-detection; the root model requests the background
  color at startup and threads the answer), glyph tokens, spacing
  roles, and the ANSI-16 and NO_COLOR degrade tables UX-7
  requires. The UX-3 analyzer forbids styling literals outside it.

## 12. Onboarding, config, credentials

- **Config** (ST-3): one Go struct in `internal/config`, persisted
  as a poplar-written TOML file in `$XDG_CONFIG_HOME/poplar/`. The
  file is machine-owned: hand-editing survives (unknown keys warn
  by name), but no supported behavior requires it. The config
  surface renders from the struct via reflection metadata (field →
  entry, help string, persist round trip), and the settings
  reference generates from the same struct.
- **Tokens** (ST-1, ST-5): `internal/keyring` wraps zalando/
  go-keyring on Linux (pure Go, D-Bus Secret Service). On macOS
  the library path shells out, which C3 forbids, so macOS and
  keyring-absent Linux use the same named fallback: a 0600 file
  under `$XDG_STATE_HOME/poplar/`, with the visible notice ST-1
  requires. Auth failures route to the ST-5 re-auth flow; the
  outbox and drafts survive the swap by construction (they live in
  the store, keyed by account, not by credential).
- **First run** (ST-1): token form → live probe (session fetch +
  one Email/query) → keyring store → initial sync with progress
  while the UI is interactive. The flow is a screen like any
  other, registered and grammar-checked. A browser-redirect OAuth
  path later (horizon 2) is a new credential step in the same
  flow; the seam is the credential-acquisition function, per
  backend.

## 13. Errors, logging, observability

ADR-0013. `internal/uerr` is ER-1's one seam: presentation types
with unexported constructors; the exported constructor takes
(operation, ids, typed reason, wrapped error), writes the
structured log line, and returns the view value. A vet-class
analyzer (Phase 5 gate) fails on user-facing error construction
outside the package. Logging is `slog` to a size-rotated file in
`$XDG_STATE_HOME/poplar/` (the documented existing home), debug
level carrying the ER-2 action trace (every user action logs
intent + outcome; a scripted session's log reconstructs the
sequence). Redaction is structural: the log value types have no
body field at all; addresses and subjects are debug-only fields;
tokens never enter a log value. ER-3's honesty states (degraded
render, partial search coverage, stale sync, queued-not-sent) are
UI states with presentations, not log-only facts.

## 14. Testing strategy

ADR-0014. The test economy follows what each layer is:

- **Pure logic** (render rules, JWZ walk, query parser, when
  parser, iTIP, recurrence splicing): table-driven unit tests +
  fixture corpora. Every render rule carries at least one fixture
  (RD-1); the license-clean specimen corpus is built in Phase 5
  pass 3, with the private Phase 1 corpus as a local supplemental
  run (QA-9's regrade rides both).
- **The store**: transaction-level tests, the randomized mutation+
  search consistency script (SR-1), migration tests from N-1
  (SY-1, from v1.1 on), and the QA-6 kill harness (seeded 30-action
  script, SIGKILL at 200 pseudorandom points, three seeds, in CI)
  asserting integrity, outbox-mutation pairing, autosave loss
  bounds, and index-count equality.
- **Engines**: `testing/synctest` for the sync loop, backoff, push
  recovery, and outbox retry schedules under virtualized time;
  backend fakes implement the seam with scriptable state
  (including the state-reset and 412 paths). A fixture JMAP server
  (recorded shapes) backs ST-1's throttled first-sync test.
- **UI**: teatest + golden files per screen state (LT-1, TH-3,
  UX-7's marker goldens, capability-profile matrix per QA-7);
  registry-driven grammar, footer, and switch-table tests (UX-1,
  UX-2, UX-4); scripted keystroke tests for the LT-2 optimistic-
  update criterion (View after one Update shows the new state).
- **Performance**: the QA-1/2/3 harnesses land with the subsystems
  they measure (build-order step 1) and run in CI with scaled
  thresholds; the measurement spike's numbers (running now)
  replace the provisional gates.
- **Live-account checks**: a small, tagged, manually-run suite
  (EventSource auth, draft round-trip, CalDAV RSVP once the token
  exists) documents server behavior; CI never touches the live
  account.

## 15. Platform posture

C10 in practice: the gate platform is Linux. macOS builds and
passes tests; keyring and notifications degrade by name there
(survey: no compliant path), clipboard rides OSC 52 everywhere
(with the size-ceiling degrade named per ER-3), and the opener
uses the C3 carve-out (`xdg-open`/`open`). The instance lock
(SY-7) is gofrs/flock on the store directory; a second poplar
refuses to start with an actionable message naming the holder
(ADR-0015 records why refusal beats read-only attach: a read-only
mode would gate every mutation path for a rare case, and the
message costs the user one keypress). Inline terminal graphics
stay behind a capability probe as a SHOULD (RD-8), off the v1
critical path.

## 16. Horizon and C11 audit

Against the knowable-horizon register:

1. **Multi-account**: account table + `account_id` on every scoped
   row + account-keyed UI state + per-account worker cast. Nothing
   assumes account singularity except the v1 account picker (a
   list of one). Open.
2. **Gmail backend**: the operation-shaped seam + capability flags
   (server threading absent → JWZ walk covers; push absent → poll
   cadence). OAuth is a credential-step swap. Open.
3. **Flag-a-bad-render**: raw MIME retained per message, fired-rule
   traces queryable, specimen export exists for the improve
   harness. The one-key capture is UI sugar over shipped seams.
   Open.
4. **Capture-mailbox corpus**: the folder model tolerates a
   non-interactive collection mailbox (role-less folders already
   sync; nothing assumes every folder is user-visible). Open.
5. **Catkin spinoff**: zero inbound imports, enforced by test.
   Open.
6. **Encryption seams**: MIME assembly and parse are single
   pipelines in `internal/mail` with a defined part-tree
   transform point before submission and after decode; an
   interposed signing/encryption layer is a pipeline stage, not a
   restructure (CO-11's design-review check). Open.
7. **Contacts micro-highlight**: the contacts list model keys
   cursor state per letter group (CT-3). Open.

C11's two halves: the design bets forward (JMAP-native sync with
push, a JSCalendar-shaped event model ahead of the RFC queue,
bubbletea v2/lipgloss v2 at their current majors, OSC 52 and
declared capability profiles over legacy terminal sniffing) and
carries no parity freight (no mailcap, no format=flowed, no
plugin surface, no per-folder goroutine zoo, no second cache
store, no library dependency for what three hundred lines of
poplar-owned code does traceably). Each subsystem's "what poplar
builds itself" entry exists because the survey proved the
dependency market empty, not because building was fun.

## 17. Risks and open items

1. **The measurement spike** (running) prices the QA-1/2/3 gates
   on real hardware; its numbers land in the requirements'
   provisional slots and gate Phase 5 planning. Risk if it
   disappoints: the concurrency discipline (section 5) has two
   pre-planned relief valves, read-connection statement caching
   and a snippet/preview column to avoid body-table reads on the
   list path.
2. **CalDAV RSVP mechanism** (section 16 item 1): blocked on the
   calendar-scoped token; both branches are designed and the
   fixture guards the loser.
3. **EventSource auth**: one authenticated-stream check against
   the live account before the sync engine's build pass.
4. **iCalendar bake-off**: go-ical is the transport-forced parse
   layer; the open question is only whether its serialization
   round-trips the CA-4 property set on Fastmail fixtures, with
   golang-ical as the fallback for the iTIP construction layer.
5. **rrule fork verification**: DST fixture set decides upstream
   vs fork.
6. **OSC 52 ceiling**: the copy-out degrade state needs its
   design-language presentation (a named "too large for terminal
   clipboard" toast with the byte count).
