# Poplar requirements

**Date:** 2026-07-27
**Status:** Approved at the Phase 3 gate (Geoff, 2026-07-27).
Revision 2 folded a three-lens adversarial review (completeness,
testability, scope); revision 3 adds the two gate directives (C9's
prior-art obligation, C11). Revision 4 lands at the Phase 5 build
boundary and carries the numbers the Phase 4 and Phase 5 gates
ratified, which until now lived only in the STATUS record while
this document still printed the superseded ones: QA-2's 25 ms p95
gate, QA-5's storage criterion as a ratio against retained body
bytes, CO-6's writer-admission term, the strengthened UX-3
analyzer rule, and UX-6's raise from SHOULD to MUST with
ADR-0017 as its design. Revision 5 (2026-08-19, the pass 1c gate)
re-ratifies QA-5's storage bound at 1.7x after the honest
full-envelope measurement. Each amended clause carries its own
revision note.
**Charter:** `2026-07-19-poplar-refounding-charter.md`
**Vision:** `2026-07-27-poplar-vision.md` (binding). Directives from
Geoff (2026-07-27) amend it: calendar is a first-class v1 surface
with full requirements; the local store exists from the first build
with no direct-JMAP interim; a unified design language precedes any
screen; mail, calendar, contacts, and config are deeply unified; the
product stance is forward-looking and lean (C11); Phase 4 opens with
an exhaustive prior-art and library survey (C9).
**Evidence:** `docs/poplar/research/2026-07-27-phase3-grounding-survey.md`
and the Phase 1 rendering verdict
(`docs/poplar/research/2026-07-19-rendering-bet-verdict.md`).

This document makes the vision testable. Phase 4 designs against it,
and the requirement priorities set the Phase 5 build order.

## How to read this document

Each requirement carries an ID, a priority, and acceptance criteria.

- **MUST**: v1 does not ship without it. A MUST earns its tier one
  of two ways: it is switch-bar (its absence forces routine fallback
  to the Fastmail web UI), or it is a named vision differentiator
  (the requirement cites which). No other path to MUST exists.
- **SHOULD**: scheduled after the switch, before 1.0 polish ends.
  Dropping a SHOULD from v1 is the default outcome of schedule
  pressure, not an exception needing a gate decision.
- **LATER**: post-v1. LATER items bind the Phase 4 design (nothing
  may foreclose them), not the build.

Acceptance criteria are written to be checked by a test, a script,
or a five-minute manual pass, and they name their oracle. Criteria
that depend on the Phase 4 measurement spike are marked
*provisional*; until the spike lands, provisional numbers are
recorded per build pass as regression baselines and gate nothing,
and the spike's measured numbers become gates from that point on.
The spike is a blocking input to Phase 5 planning.

Every user action also satisfies the standing invariants in C7
(testable, logged) even where a criterion does not restate them.

## Constraints

These bind every requirement and every Phase 4 decision.

- **C1. Local-first from the first build.** All interactive reads
  (list, thread, message, search, calendar, contacts) hit the local
  store only. JMAP and CalDAV are background sync layers. No build
  phase works directly against the network on the interactive path;
  the legacy client proved that path unusably slow. Four enumerated
  exceptions exist, each user-initiated or on-demand, each with a
  visible progress state, a bounded timeout, and a defined offline
  behavior: on-demand body fetch (SY-6), attachment fetch (RD-8),
  explicit remote-image load (RD-6), and explicit server-side search
  (SR-7).
- **C2. Deterministic rendering.** No LLM at render time. Rules are
  named, declarative, provenance-carrying, and traceable (Phase 1
  verdict, settled).
- **C3. Self-contained.** One binary, static per platform
  (`CGO_ENABLED=0`). No external editor, indexer, delivery agent, or
  companion daemon, and no mailcap: poplar never consults a
  handler-configuration file. One carve-out: a user-initiated "open
  with the system handler" hands a file to the platform opener
  (RD-3, RD-8); the OS owns the choice, poplar owns the temp file's
  lifecycle.
- **C4. One account, seams open.** v1 syncs one Fastmail account.
  The backend sits behind a seam that admits a second account and a
  Gmail backend later (horizon items 1 and 2). The seam is not only
  the protocol layer: the store schema carries an account identity
  on every account-scoped row from v1, and UI state (cursor, scroll,
  search scope) is keyed by account. Acceptance: a Phase 4 schema
  and design review checks both.
- **C5. Design language first.** A unified design language (theme
  tokens compiled as Go values, a component vocabulary, an
  interaction grammar) is settled before any screen is built, and
  every screen derives from it. Text wireframes precede any screen
  build. Behaviors this spec defers to "the design language
  document" land in that artifact, which UX-3 pins.
- **C6. Deep unification.** Mail, calendar, contacts, and config are
  one product: the same key means the same verb on every surface,
  the same components render lists, pickers, modals, and hints
  everywhere, and one search grammar (SR-2), one undo model (UX-9),
  and one natural-language time parser serve all surfaces.
- **C7. Observable by construction.** Every user-visible error also
  reaches the log through one seam. Every user action produces a
  deterministic test and a log trace. Silent no-ops are bugs.
- **C8. Keys are modifier-free single keys.** No chords, no
  sequences, no Ctrl/Alt. Ambiguous design calls default to the vim
  idiom bent to that constraint. Text-entry contexts follow the
  UX-8 model: printable keys are input there, and command verbs are
  reached through the leave-field verb, never through modifiers.
- **C9. Current stack, prior art first.** Phase 4 pins the latest
  stable Go, current Charm releases, and actively maintained mail,
  JMAP, iCalendar, and CalDAV libraries. No dependency inherits its
  legacy pin. Claims about releases and idioms are verified live at
  the point of use. Phase 4 opens with an exhaustive survey of
  existing libraries, tools, and prior art per subsystem; poplar
  builds nothing a maintained dependency already does well (Geoff,
  2026-07-27).
- **C10. Platforms.** v1's gate platform is Linux (the development
  workstation: ThinkPad X1 Carbon, Linux Mint, kitty; the perf
  harness records the exact spec). macOS builds and passes tests but
  does not block the v1 gate. Every platform-specific integration
  (keyring, notifications, clipboard, opener, terminal graphics)
  degrades by name on platforms that lack it.
- **C11. Forward-looking and lean.** Poplar anticipates where mail,
  calendar, and contacts are heading and lands there: it bets on
  the protocols and formats the field is moving to (JMAP-native
  sync, the JSCalendar upgrade seam, modern terminal capabilities)
  rather than re-implementing the field's past. Its feature
  philosophy is all the features you actually need and none you
  don't: a requirement earns its place by daily usefulness, never
  by field parity, and Phase 4 design choices are graded against
  both halves (Geoff, 2026-07-27).

## 1. Shell and unification (UX)

- **UX-1 (MUST, switch-bar and C6). Unified interaction grammar.**
  One documented grammar defines the global verbs (navigate, open,
  back, search, select, act, undo, help, quit, surface-switch,
  leave-field), including in-reader message stepping and body
  paging. Every surface binds them to the same keys. Acceptance:
  screens register in a package-level registry at init, and a
  reflection test fails on any screen type in `internal/ui/...`
  that is unregistered; the grammar check iterates the registry and
  fails on a binding that contradicts the grammar; list-navigation
  keys behave identically in mail list, thread list, calendar
  agenda, contact list, and config, asserted by one shared test.
- **UX-2 (MUST, vision differentiator 5). Mode-scoped key-hint
  footer.** Every screen shows a footer of currently legal keys,
  alpine-style: no advertised key is a no-op, no legal key goes
  unadvertised except a listed exception set, and the footer never
  goes stale. In a text-entry context the footer shows the
  leave-field verb and the context's own verbs. Acceptance: a
  registry-driven test proves footer set equals advertised keymap
  per screen; the exception set is capped at five entries, each
  with a committed reason in the design-language document, and the
  test fails on an undocumented exception.
- **UX-3 (MUST, C5). Design language artifact.** The design language
  (charter adjectives, tokens, component vocabulary, interaction
  grammar, and the default values this spec assigns to it) exists as
  a committed document plus a `theme` package of compiled Go values
  before the first screen lands. Acceptance: outside
  `internal/theme` and `internal/catkin`, in non-test files, an
  analyzer forbids lipgloss constructor calls, ANSI escape
  literals, and any rune or string literal containing a non-ASCII
  code point. An inline `//poplar:allow-unicode <reason>` escape
  covers the legitimate non-theme cases (entity handling in
  `internal/render`, tokens in `internal/when`, corpus fixtures);
  the gate counts escapes and the pass-end reviewer reads new
  ones. *Revision 4 strengthens the literal rule from five named
  Unicode blocks to all non-ASCII code points, since the block
  list enumerated what to catch and so missed everything outside
  it, and adds the catkin exemption the spinoff needs. The
  separate numeric-spacing clause is withdrawn: once lipgloss
  constructors are banned, spacing is reachable only through the
  theme's spacing-role API, so it needs no check of its own.*
- **UX-4 (MUST, C6). Surface switching.** Mail, calendar, contacts,
  and config are reachable from anywhere through one switching
  idiom. Acceptance: the switch table is committed and covers every
  registered screen outside text-entry contexts; text-entry
  contexts are listed exceptions reached via the leave-field verb
  first, and a test asserts the exception list equals the set of
  screens accepting printable input; surface state (cursor, scroll,
  pending input) survives a round trip.
- **UX-5 (MUST, switch-bar). Help.** A help overlay lists the active
  screen's full keymap and the global grammar. Acceptance: the help
  key opens it on every registered screen; content is derived from
  the keymap registry so it cannot drift, asserted by test.
- **UX-6 (MUST, Geoff 2026-07-27). Pointer support.** The pointer
  is an accelerator over a keyboard-complete grammar, never the
  only path to anything. Its scope is ADR-0017's eleven-row
  vocabulary: click to move the cursor, double-click to open,
  click a sidebar entry to goto, click a pane to focus it, click a
  status-line surface digit or footer hint to run that verb under
  the same state rules the keys obey, click a banner dismiss,
  wheel to scroll, click inside a focused text-entry field to move
  the in-field cursor, and click a modal answer. Drag-select in
  the reader stays SHOULD inside this MUST. Nothing requires the
  mouse, and enabling mouse reporting never removes a copy path
  (RD-16 covers copy). Acceptance: every mouse-reachable action
  has a keymap entry, asserted through the registry test; pointer
  behavior is tested by injecting typed mouse messages at the
  Update and golden layers, never at the terminal. *Revision 4
  raises this from SHOULD on Geoff's day-one-mouse directive,
  ratified at the Phase 5 machine gate. ADR-0017 is the design.*
- **UX-7 (MUST, switch-bar). Accessible defaults.** The shipped
  theme's foreground/background pairs compute to at least 4.5:1 for
  text and 3:1 for state indicators, asserted by a test over the
  theme values. Under `NO_COLOR` and under an ANSI-16 profile, each
  of unread, selected, focused, and error is carried by a distinct
  non-color channel (glyph, position, or reverse video), asserted
  by a golden render in which no two states share a marker.
- **UX-8 (MUST, C8). Text-entry command model.** A single model
  governs every text-entry context (compose body, compose headers,
  search bar, forms, pickers): printable keys are input; one
  leave-field verb exits to the context's command state; every
  message-level verb (send, postpone, attach, identity switch,
  scope toggle) is reachable from the command state as a
  modifier-free single key. Acceptance: send, postpone, and attach
  are each reachable from compose without a chord or sequence,
  asserted by a scripted keystroke test; the model is defined once
  in the design-language document and referenced by every
  text-entry screen.
- **UX-9 (MUST, C6). One undo model.** Reversible actions across
  surfaces (mail triage single and bulk; calendar event delete and
  edit) share one undo presentation: a toast naming the action, a
  10-second window with visible countdown, single-level depth, and
  a single undo key. Undo after the outbox has dispatched issues a
  compensating mutation through SY-4; undo of a message the server
  has since changed resolves by SY-3's conflict rule with a logged
  trace and a toast. The window does not survive quit, and the
  toast says so. Permanent deletions (LT-4, FO-4) never offer undo.
  Acceptance: single, bulk, post-dispatch, and conflicting undo
  paths are tested per covered surface; RSVP answers are excluded
  by design (re-answering is the correction) and section 14 rules
  it.

## 2. Startup, onboarding, and config (ST)

- **ST-1 (MUST, switch-bar). First run to reading mail in one
  sitting.** First launch walks the user through the Fastmail
  token, probes the connection before saving, stores the token in
  the OS keyring, and starts the initial sync with visible progress
  while the UI stays responsive. The flow admits a
  browser-redirect credential path later (horizon 2) without
  restructuring. Acceptance: against a fixture server throttled to
  100 messages/second, the list shows its first full screen within
  3 seconds of token acceptance and stays responsive per QA-2
  throughout; an invalid token or unreachable server produces a
  named, actionable error and returns to the form, with a test per
  failure; keyring integration is pure Go with no subprocess and no
  cgo (`CGO_ENABLED=0` build plus a test asserting no
  `exec.Command`); absent a keyring service, the token falls back
  to a 0600 file with a visible, named notice, never silently.
- **ST-2 (MUST, switch-bar). Startup never blocks on the network.**
  Acceptance: with the network down, a warm start reaches an
  interactive, populated list within the QA-1 p95 ceiling.
- **ST-3 (MUST, C6). In-app config surface.** Configuration is a
  surface inside poplar (identities, signatures, default calendar,
  default reminders, trash retention, image-loading posture,
  notification posture), edited in-app with inline help,
  alpine-style. No hand-authored dotfile is required for any
  supported behavior. Acceptance: the config schema is one Go
  struct; a reflection test asserts every field has a
  config-surface entry, a help string, and a persist round trip;
  the settings reference is generated from the struct so the two
  cannot diverge.
- **ST-4 (LATER). Import.** No mbox/maildir/eml import in v1; the
  server is the source of record. The store design must not
  foreclose a later importer.
- **ST-5 (MUST, switch-bar). Credential lifecycle.** An auth
  failure (expired or revoked token) surfaces a named
  re-authentication flow reachable from any surface, re-probes
  before saving, and preserves the outbox, drafts, and store across
  the swap. Acceptance: revoked-token and keyring-absent recovery
  paths are both tested; queued outbox actions dispatch after
  re-authentication without loss.

## 3. Mail list and triage (LT)

- **LT-1 (MUST, switch-bar). The list.** One opinionated list
  layout: flags, sender, subject, date, attachment marker, thread
  count; unread and selection state distinguishable per UX-7. One
  sort order: date descending. No user column or sort
  configuration. Acceptance: golden render tests cover unread,
  flagged, selected, threaded, and overflow rows at narrow and wide
  widths.
- **LT-2 (MUST, switch-bar). Single-key triage everywhere mail
  shows.** Archive, delete (to Trash), flag, mark read/unread,
  move, and junk/not-junk act on the cursor row, the active
  selection, or the open message in the reader, optimistically
  against the local store, queued through the outbox. After a
  reader-context action, the reader advances to a defined target
  (next message in the list order; configurable nothing else).
  Acceptance: each verb is asserted in a bubbletea unit test where
  the `View()` after the single `Update` handling the keypress
  already shows the new state, with no intervening command round
  trip; each verb round-trips store and outbox in tests and
  converges after reconnect; verbs behave identically from list,
  thread view, and reader.
- **LT-3 (MUST, switch-bar). Selection and bulk.** A mark toggle
  plus select-by-criteria (all unread, from this sender, this
  thread, current search result) build a working set; one action
  applies to the full matching set, never only the visible page.
  Acceptance: correctness at a small fixture set where the affected
  count equals the criteria query's count; a 5k-message bulk
  archive completes within 3 seconds with QA-2 held throughout
  (the scripted keystroke session runs concurrently), and the UI
  reflects the change immediately while server convergence reports
  through SY-5.
- **LT-4 (MUST, switch-bar). Undo per UX-9.** Every reversible
  triage action (single or bulk) offers undo; permanent deletion
  and emptying Trash/Junk are confirmed per FO-4 and never offer
  undo. Acceptance: undo restores exact prior state (folders,
  flags) for single and bulk in tests; the toast names the action.
- **LT-5 (MUST, switch-bar). Move picker.** A folder picker with
  type-ahead moves messages and can create a missing folder inline.
  Acceptance: picker filters as you type; create-and-move in one
  flow round-trips to the server.
- **LT-6 (MUST, switch-bar). Next-unread.** A single key advances
  to the next unread message in the current folder; wrap-around
  reports exhaustion in the status line. Cross-folder traversal
  (Inbox, then non-role folders in sidebar order, skipping Trash,
  Junk, and Drafts) is SHOULD. Acceptance: in-folder traversal and
  wrap tested; the cross-folder order, when built, matches the
  stated order.
- **LT-7 (LATER). Snooze and thread mute.** The target account
  shows no snooze usage (live probe, 2026-07-27); neither is
  switch-bar. The store schema must leave room for both (a
  hidden-until timestamp; a per-thread rule).

## 4. Threading (TH)

- **TH-1 (MUST, switch-bar). Thread model.** Threads come from
  server-provided thread identity where the backend supplies one; a
  References walk over cached headers covers messages the server
  fails to thread; a message without references is a thread of one.
  Subject-heuristic grouping is never used. Acceptance: fixture
  corpus covers broken References, cross-posted, and orphan cases;
  no false merges on same-subject unrelated mail.
- **TH-2 (MUST, switch-bar). Cross-folder threads.** A thread view
  includes the user's own replies from Sent and members in Archive
  (naive folder-scoping loses Sent replies; the archived spec left
  this undefined). Acceptance: a fixture thread spanning Inbox,
  Sent, and Archive renders complete from any member folder.
- **TH-3 (MUST, switch-bar). Thread presentation.** Threaded is the
  default; collapsed rows summarize (latest date, unread-if-any,
  count); expanded rows indent; fold-one and fold-all are single
  keys; a flat mode exists as a toggle. Acceptance: golden renders
  for collapsed, expanded, and flat; cursor position survives fold
  and unfold.
- **TH-4 (MUST, switch-bar). Thread-scoped actions.** A collapsed
  thread row's triage verb covers every member; an expanded
  member's verb covers only itself. Acceptance: archive on a
  collapsed row moves all members in one undoable action; the same
  key on an expanded member moves one message.

## 5. Reading and rendering (RD)

- **RD-1 (MUST, vision differentiator 1). The pipeline.** Rendering
  is the Phase 1 architecture productized: deterministic Go, a rule
  engine whose rules carry name, observable trigger and transform,
  motivating corpus references, and tests; the renderer reports
  which rules fired per message; the raw MIME source of every
  rendered message is retained and reachable (the flag-loop and
  corpus seams depend on it). Acceptance: the fired-rule trace is
  available for any rendered message; every rule has at least one
  fixture test; golden render tests run in CI against a committed,
  license-clean specimen set covering the Phase 1 classes and known
  failure shapes (built during Phase 5 pass 3 from public sources
  and scrubbed self-generated mail), with the private Phase 1
  corpus as a local-only supplemental run.
- **RD-2 (MUST, vision differentiator 1). The fact check.** A
  fact-inventory self-check derives actionable facts from the
  source (links, amounts, dates, codes) and verifies the render
  covers them; a failed check downgrades the render honestly.
  Acceptance: against a labeled ground-truth corpus (the Phase 1
  specimens plus at least 40 real messages with hand-annotated
  facts, committed in license-clean form): fact recall at or above
  0.95; downgrade rate at or below 5% on the Phase 1 pass
  specimens; zero silent losses (a missed fact must trigger a
  downgrade); extraction is deterministic across 100 runs.
- **RD-3 (MUST, switch-bar). Fallback stack.** A single key cycles
  rendered markdown, filtered plain text, and the raw source;
  open-in-browser exports the HTML part through the C3 carve-out.
  Degraded renders say so. Acceptance: every fixture message
  reaches every mode, plus a property test over generated MIME
  trees; the exported temp file is 0600 in the user runtime
  directory and removed on exit; the export keeps content images
  and remote references (opening in a browser is an explicit act;
  the confirmation names it).
- **RD-4 (MUST, switch-bar). Charset and MIME correctness.**
  RFC 2047 headers, quoted-printable and base64, and legacy
  charsets decode correctly before rendering. Acceptance: each
  fixture's decoded body is byte-identical to its committed UTF-8
  expectation across ISO-8859-1, Windows-1252, UTF-8, ISO-2022-JP,
  and malformed inputs; a charset label that lies is detected by
  validity check and decoded correctly; unknown or absent charset
  falls back to a documented default with a visible notice;
  undecodable bytes become U+FFFD, never silent substitution.
- **RD-5 (MUST, switch-bar). Links.** Links render as numbered,
  followable references with duplicate collapse; a link mode
  selects and opens or copies them. Tracking parameters strip on
  open and copy. Acceptance: extraction and dedup tested against
  the specimen classes; the strip list is committed as data with a
  test per entry; stripping touches only listed parameter names,
  never path or fragment, and never a List-Unsubscribe target; a
  negative fixture set of ten signed or tokenized URLs asserts
  byte-identical pass-through.
- **RD-6 (MUST, switch-bar). Remote content.** Remote images never
  load automatically; a placeholder shows what was blocked; a
  per-message key loads them. Acceptance: the render path runs
  under an injected dialer that fails the test on any outbound
  connection, and a process-wide assertion covers a render-only
  scripted session.
- **RD-7 (MUST, vision differentiator 4). Coder polish.** Fenced
  code renders with syntax highlighting; unified diffs and patch
  mail render with diff coloring; commit and issue URLs stay
  intact. Acceptance: golden renders for a code-fence newsletter, a
  git patch, and a GitHub notification.
- **RD-8 (MUST, switch-bar). Attachments.** An attachment list
  shows name, type, and size; save-to-directory and
  open-with-system-handler (C3 carve-out; temp-file lifecycle as
  RD-3) are single keys. Inline image rendering behind a terminal
  capability probe is SHOULD. Saving a full message as .eml is
  SHOULD. Acceptance: save and open paths tested; an
  attachment-only message renders the attachment list plus the
  committed empty-body notice as a golden.
- **RD-9 (MUST, switch-bar). Reader header and navigation.** From,
  To, date, subject, folder, and flags, with verification per
  RD-10 when present; next and previous message and body paging
  work from the reader per UX-1's grammar. Acceptance: golden
  render; long-header truncation defined; reader stepping tested.
- **RD-10 (SHOULD). Sender verification.** Parse
  `Authentication-Results`; quiet pass, loud DMARC failure or
  From-domain mismatch. Acceptance: fixtures for pass, fail, and
  absent; at least five real ARC-signed mailing-list samples assert
  no false alarm.
- **RD-11 (SHOULD). List-Unsubscribe.** One key offers unsubscribe
  via RFC 8058 one-click POST, else mailto, behind a confirmation
  naming the target. Acceptance: each method tested against
  fixtures.
- **RD-12 (MUST, vision differentiator 1). Quote folding.** Quoted
  history collapses to a stub, expandable under the cursor; the
  heuristic fails open (shows the text). Threaded reading with
  history unfolded everywhere is a degraded render by the
  product's own standard. Acceptance: fixture set covers top-post,
  bottom-post, inline, and attribution-line variants; fail-open
  asserted on an unparseable quote shape.
- **RD-16 (MUST, switch-bar). Copy out.** A copy mode yanks the
  message body, a selected region, a header value, an address, or
  a link to the system clipboard, with an OSC 52 path for remote
  sessions. Acceptance: each yank target tested; clipboard
  integration degrades by name per C10; mouse reporting (UX-6)
  never removes a copy path.
- **RD-13 (LATER). Flag-a-bad-render** (backlog #63). The one-key
  capture loop is post-v1. Its seams (raw-source retention, the
  fired-rule trace) are RD-1 requirements and ship in v1.
- **RD-14 (LATER). Capture-mailbox corpus** (horizon 4). A
  dedicated collection mailbox feeding the offline loop is post-v1;
  the folder and account model must admit a non-interactive
  collection mailbox, and RD-1's retention seam is its input.
- **RD-15 (MUST, vision differentiator 1). Rule-improvement
  harness.** The offline loop the verdict demonstrated is a
  shipped, repeatable path: from a stored failing specimen to a new
  named rule with a regression test, runnable from the repo,
  including the corpus-grading harness. The in-app flag key is
  RD-13; the harness is not. Acceptance: a documented run
  reproduces the Phase 1 corrective round's mechanics on a sample
  specimen end to end.

## 6. Search (SR)

- **SR-1 (MUST, switch-bar). One local index.** Search runs against
  a local full-text index over headers and bodies, fully offline,
  for mail on every surface. Index updates are atomic with message
  mutations: no sequence of updates or deletes leaves the index
  inconsistent with the store. Acceptance: with the network down,
  body search over the synced mailbox returns within QA-3;
  index-store consistency is asserted after a randomized
  mutation-and-search script.
- **SR-2 (MUST core, switch-bar). One grammar.** A typed query
  grammar serves mail and calendar (CA-10). MUST core: bare terms,
  quoted phrases, `from:`, `to:`, `cc:`, `subject:`, `body:`,
  `in:`, `is:` (read, unread, flagged, answered), `has:attachment`,
  `before:`/`after:` with the shared date parser, and the
  fall-through rule (an unknown `key:value` becomes a bare term,
  widening rather than silently narrowing). SHOULD:
  `larger:`/`smaller:`, negation, OR, parentheses, and
  `header:Name=value` (SR-4 merged here). Acceptance: a grammar
  corpus covers every shipped operator and the fall-through rule;
  a malformed query (unbalanced parens, dangling OR, empty
  operator) degrades to a bare-term search with a visible notice,
  never an error or a silent empty result.
- **SR-3 (MUST, switch-bar). Search as you type, scoped.** Results
  update as the query changes; in-flight queries are superseded,
  never queued; every query carries a 500-row cap with a visible
  more-results state; results order by date descending. One key
  (from the search bar, via the UX-8 command state) toggles scope
  between the current folder and the whole account. Acceptance:
  keystroke-to-result within QA-3; a stale result never overwrites
  a newer query's results (raced in test); the scope state is
  visible in the search bar.
- **SR-5 (SHOULD). Match highlighting.** Result rows and the opened
  reader highlight matched terms, respecting UX-7. Acceptance:
  golden renders.
- **SR-6 (LATER). Saved searches.** The grammar and sidebar design
  must admit stored queries later.
- **SR-7 (MUST, switch-bar). Server-side fallback during partial
  coverage.** While local body coverage is partial (SY-6), an
  explicit key reruns the query as a server search; results merge,
  labeled by source; offline, the path degrades by name. This is a
  C1 enumerated exception. Acceptance: during a simulated backfill
  at 50% coverage, the fallback finds a fixture message whose body
  is not yet local; the search bar indicates partial coverage until
  backfill completes.

## 7. Folders (FO)

- **FO-1 (MUST, switch-bar). Role-based classification.** Special
  folders map by server-declared role where the protocol defines
  one, with a tested name-heuristic fallback; archive, trash, junk,
  drafts, sent, and scheduled behave by role everywhere. A
  duplicate-role account degrades by the name heuristic with a log
  line, never a refusal to sync. Acceptance: fixture accounts with
  unusual names and a duplicate role both classify predictably.
- **FO-2 (MUST, switch-bar). Folder navigation.** A sidebar or
  switcher lists folders with unread counts; single keys jump to
  Inbox, Drafts, Sent, Archive, Junk, Trash; a type-ahead switcher
  reaches any folder. Acceptance: counts match the store; jump keys
  follow the grammar; switcher filters as you type.
- **FO-3 (MUST, switch-bar). Folder lifecycle.** Create, rename,
  and delete folders in-app; delete requires a confirmation naming
  the message count and refuses on role folders. Acceptance:
  lifecycle round-trips to the server; renaming a folder with
  queued offline actions does not strand them (test).
- **FO-4 (MUST, switch-bar). Trash and junk semantics.** Delete
  moves to Trash; a distinct permanent-delete exists for Trash and
  Junk contents behind a y/n confirm with a count; optional
  retention (default off) sweeps Trash at startup, logged.
  Acceptance: soft and permanent paths tested; the sweep runs only
  when enabled and reports what it removed.
- **FO-5 (LATER). Labels.** The target account runs folders-mode;
  arbitrary user labels are not v1. JMAP keywords still back flags
  (LT-2), and the store schema keeps multi-mailbox membership and
  keyword queries open.

## 8. Compose, drafts, and send (CO)

- **CO-1 (MUST, vision differentiator 3). Catkin compose.** Compose
  is a full-pane, modeless surface: structured headers (To, Cc,
  Bcc, Subject, From) above the Catkin live-markdown body (visible
  delimiters, iA Writer shape). Poplar owns message-level verbs
  through the UX-8 command state; Catkin owns the body. No external
  editor, ever. Acceptance: header and body focus follow UX-8;
  compose opens within QA-2 from any surface.
- **CO-12 (MUST, switch-bar). Compose editor contract.** Catkin is
  an in-tree package with no inbound poplar coupling (the spinoff
  stays possible). v1 requires: vim-idiom motions within the body,
  word and line operations, in-editor undo (scoped to the buffer,
  independent of UX-9, and the design-language document states the
  boundary), bracketed paste preserving pasted content byte-exact,
  and soft wrap that never rewraps fenced code blocks. Acceptance:
  a scripted editing session covers each verb; a pasted code fence
  survives byte-identical through edit, save, and send.
- **CO-2 (MUST, switch-bar). Reply, reply-all, forward.** Seeding
  dedups recipients, drops the user's own identities from To/Cc,
  quotes with depth-preserving markers, sets In-Reply-To and
  References, applies non-doubling Re:/Fwd: prefixes, and carries
  attachments through on forward with a per-attachment drop
  control. Reply-to-list honoring List-Post is SHOULD.
  Forward-as-attachment (message/rfc822) is LATER. Acceptance: a
  fixture matrix covers recipient math, prefix idempotence,
  reference chains a client can thread, and forwarded attachments
  arriving intact.
- **CO-3 (MUST, switch-bar). Identity selection.** The From
  identity auto-selects by delivered-to match, wildcard alias
  patterns included, and is switchable pre-send; the identity's
  signature materializes into the buffer at open with the RFC 3676
  `-- ` delimiter. Switching identity replaces the signature block
  only when the current block is byte-identical to the outgoing
  identity's signature; an edited block is preserved and the swap
  offered, not applied. Per-recipient last-identity memory as a
  tiebreak is SHOULD. Acceptance: delivered-to, alias-pattern,
  edited-signature, and swap cases tested.
- **CO-4 (MUST, switch-bar). Contacts autocomplete.** From two
  typed characters, the address fields suggest from the local
  contact store and sent-history cache, ranked by recency and
  frequency, rendered as `Name <email>`; noreply-pattern addresses
  are excluded from history suggestions. Acceptance: ranking is
  deterministic under fixture history; acceptance rewrites the
  field correctly; suggestion latency within QA-2.
- **CO-5 (MUST, switch-bar). MIME assembly.** One markdown source
  yields multipart/alternative (clean text/plain plus conservative
  text/html); a text-only toggle serves list and patch mail; fenced
  code and diffs pass through verbatim in the plain part
  (format=flowed never reflows them). Acceptance: the composed
  MIME tree matches a committed structural golden (part order,
  charset, transfer encoding); the HTML part validates against a
  committed tag and attribute allowlist fixed in Phase 4; the
  text/plain part is byte-identical to its golden with no reflow
  inside fenced regions; a one-time visual confirmation in
  Fastmail web and Gmail web is recorded as a Phase 5 artifact,
  not run as a gate.
- **CO-6 (MUST, switch-bar). Drafts.** Local autosave on a 1-second
  debounce; server push on close and every 5 idle minutes; the
  leave-field verb exits the body, and the command state's postpone
  verb saves and closes (discard is a separate, confirmed verb);
  reopening restores full state; send deletes the draft in the same
  transaction. Server-canonical, last-write-wins. Acceptance:
  `kill -9` at a random point during a scripted compose loses at
  most the debounce window plus the writer's admission ceiling
  (~50 ms), asserted over 50 seeded runs; a draft round-trips
  through Fastmail web preserving body and headers; the send
  transaction leaves no lingering draft. *Revision 4 adds the
  admission term: a keystroke accepted inside the debounce has
  still to reach the single writer goroutine, and a bound that
  ignores that queue is one no implementation can hold.*
- **CO-7 (MUST, switch-bar). Outbox and undo send.** Send queues
  through the durable outbox with a 10-second send-delay window
  (single-key cancel returning to compose intact); failures surface
  with a typed reason and a retry path; nothing sends silently,
  nothing is lost offline. Exit during the delay window persists
  the queued send and dispatches on next launch. Acceptance:
  cancel, offline-queue, exit-then-cancel, and each failure class
  render and log per C7.
- **CO-8 (MUST, switch-bar). Attachments.** Attach via a path
  picker with completion. A send-time missing-attachment keyword
  scan is SHOULD (committed keyword list; at most one false warn
  per 50 attachment-free fixture messages). Acceptance: attach and
  remove tested; a server rejection for size surfaces legibly.
- **CO-9 (SHOULD). Send later.** Schedule into the account's
  scheduled-send role folder, sharing the time parser. The target
  account's scheduled folder is empty today (live probe,
  2026-07-27), so this is not switch-bar. Acceptance: scheduled
  mail appears in Scheduled with its time; cancel-before-send
  works.
- **CO-10 (LATER). Snippets and AI tidy.** Neither is switch-bar.
  Catkin stays poplar-agnostic; tidy binds to an explicit key,
  never the send path.
- **CO-11 (LATER, horizon 6). Encryption seams.** The MIME assembly
  and parse paths accept an interposed signing or encryption layer
  without restructuring. Acceptance: a Phase 4 design-review check.

## 9. Contacts (CT)

- **CT-1 (MUST, switch-bar). Local contact store.** Contacts sync
  read-only from Fastmail over JMAP contacts into the local store,
  feeding autocomplete (CO-4) and the contact card. Acceptance:
  sync converges within one sync cycle of a remote edit (cadence:
  with push where the server offers it, else the SY-2 poll
  cadence); the store serves autocomplete offline.
- **CT-2 (MUST, switch-bar). Contact card.** From the reader or any
  address, one key opens a card: name, addresses, recent
  correspondence, shared calendar events when calendar data holds
  any. Acceptance: card opens from reader sender line and compose
  field; correspondence list matches the store.
- **CT-3 (SHOULD). Contacts surface.** A browsable contacts list
  (search, A-Z jump) on the shared components and grammar. The
  list model retains per-letter cursor position within a group
  (horizon 7 stays reachable). Acceptance: navigation and search
  follow UX-1; opening a contact starts a compose or shows the
  card.
- **CT-4 (LATER). Contact editing and groups.** Editing, creation
  (including add-sender-to-contacts), and group expansion stay in
  Fastmail web for v1; the store schema keeps write paths and group
  membership open.

## 10. Calendar (CA)

Calendar is v1 scope by Geoff's 2026-07-27 directive ("mail just
isn't that useful without it"), with full requirements. The backend
contract today is CalDAV; Fastmail exposes no JMAP calendar API
until the specification finalizes, and the design treats JMAP
calendars as a drop-in upgrade seam. All views and verbs follow the
unified grammar (C6). The gate decides one open taste call: which
grid views ship in v1 (CA-3's tier).

- **CA-1 (MUST, switch-bar). Local calendar store.** Calendars and
  events sync two-way over CalDAV into the local store; all views
  read locally (C1). Recurrence expands locally for the visible
  window, honoring EXDATE, RDATE, and overrides; timezones resolve
  from VTIMEZONE to IANA zones via vendored, version-pinned CLDR
  Windows-zone mappings, and an unmappable TZID floats to local
  time with a visible notice. An ETag precondition failure on an
  event write refetches, surfaces a named conflict, and never
  silently discards the remote version. Acceptance: a fixture
  corpus of recurring, overridden, all-day, and cross-timezone
  events renders correct local times; the conflict path is tested;
  sync converges after edits on either side.
- **CA-2 (MUST, switch-bar). Agenda view.** The default calendar
  view is a scrollable agenda: date-grouped rows (time range,
  title, location, calendar color), today highlighted, empty days
  collapsed, list movement per the grammar, single-key jump to
  today, and the natural-language date jump ("next wed", "aug 14").
  Acceptance: golden renders; movement, today, and date jump
  tested.
- **CA-3 (SHOULD). Grid views.** Week, day, and month views with
  single-key switching and per-view unit stepping. Not switch-bar
  by the strict test (the agenda plus date jump covers routine
  schedule reading), but a named gate call: the owner rules whether
  any grid view is v1-blocking. Acceptance (when built): golden
  renders per view; switching and stepping follow the grammar.
- **CA-4 (MUST, switch-bar). Event detail, create, and edit.**
  Enter opens full event detail. Create and edit capture title,
  start/end or all-day, location, description, calendar, recurrence
  presets (daily, weekly, monthly, yearly), attendees, free/busy,
  and reminders, in a form following the design language. Adding
  attendees dispatches iMIP invitations (METHOD:REQUEST) through
  the outbox (SY-4); editing a single (non-recurring) event and
  deleting an event are MUST, and organizer edits prompt to notify
  attendees, recording the choice. Acceptance: the event fetched
  back over CalDAV parses equal to the sent event over the modeled
  property set (DTSTART/DTEND with TZID, RRULE, EXDATE, RDATE,
  VALARM, TRANSP, LOCATION, DESCRIPTION, ATTENDEE, SEQUENCE); an
  invitation message exists in Sent after an attendee add;
  validation errors are named.
- **CA-5 (SHOULD). Recurring-series editing.** Editing or deleting
  a recurring event prompts the full three-way scope (this event,
  this and future, all), producing correct RECURRENCE-ID and
  override structure. Until then, recurring series are edited in
  Fastmail web; creating recurring events (CA-4 presets) and
  answering recurring invites (CA-6) are unaffected. Acceptance
  (when built): each scope verified by CalDAV round trip and
  re-parse.
- **CA-6 (MUST, switch-bar and vision). The RSVP bridge.** An
  invite in the mail reader renders as an inline event card from
  the text/calendar part, never scraped from HTML: title, time in
  local zone, location, organizer, recurrence summary, response
  state. Three single keys answer accept, tentative, decline; one
  action records the answer on the server calendar and ensures the
  organizer receives exactly one valid iTIP reply per answer,
  whether the server generates it or poplar sends the iMIP reply
  through the outbox (the Phase 4 probe fixes the mechanism; the
  double-send case is a named fixture). The card reflects the
  recorded answer, and the same event opens in the calendar
  surface. Acceptance: poplar's own act is the observable: the
  CalDAV write succeeds with the probe result on record, or an
  iMIP METHOD:REPLY with correct PARTSTAT reaches Sent. Negative
  criteria: an unparseable text/calendar part renders the message
  normally with a named notice and no RSVP keys; a user absent
  from ATTENDEE gets a read-only card; a SEQUENCE regression is
  ignored with a log line; METHOD:CANCEL marks the local event
  cancelled and removes the answer keys; an offline answer queues
  per SY-3 and the card shows pending until dispatch; a changed
  answer re-sends; an update supersedes the card's stale copy.
- **CA-7 (MUST, switch-bar). Reminder data; in-app surfacing.**
  Default reminders configure separately for timed and all-day
  events (ST-3) and persist on created events (the data-correctness
  half is MUST). In-app surfacing: a due reminder renders as a
  non-focus-stealing banner; no keypress is consumed by it while a
  text-entry context is focused (asserted by typing through a
  firing reminder); on launch, reminders due in the trailing 12
  hours surface once as a missed list, older ones drop with a log
  line. Reminders while poplar is not running are a ruled non-goal
  (section 14): the account's own mobile and web notifications
  remain their home.
- **CA-8 (MUST, switch-bar). Multiple calendars.** All visible
  calendars sync; each carries a theme-assigned color; visibility
  toggles per calendar; a default calendar for new events is
  configurable. Acceptance: toggles persist; colors come from the
  theme package; events land in the chosen default.
- **CA-9 (SHOULD). ICS import and export.** Import an .ics file
  into a chosen calendar; export an event or calendar. Acceptance:
  round trip preserves the modeled property set; rejected
  components are named.
- **CA-10 (SHOULD). Event search.** The SR-2 grammar covers events
  (title, location, description, attendee) from the calendar
  surface. Acceptance: applicable operators work; results open the
  event.
- **CA-11 (SHOULD). System notifications.** Where the platform
  offers a notifier, due reminders also emit a desktop
  notification. No notifier is a capability fact, not a failure:
  reported once in the config surface and logged at startup.
- **CA-13 (SHOULD). Organizer response tracking.** Inbound iTIP
  replies to the user's own events update per-attendee
  participation status, visible in event detail. Acceptance:
  fixture replies (accept, decline, tentative) update the detail
  view.
- **CA-12 (LATER). Scheduling aids.** Attendee free/busy lookup,
  availability views, subscription management, and sharing
  management are post-v1; the store keeps attendee and transparency
  data so none is foreclosed.

## 11. Sync and offline (SY)

- **SY-1 (MUST, switch-bar). The store is the truth the UI reads.**
  One local store (mail, calendar, contacts, outbox, queues) backs
  every surface; it is rebuildable from the server except cached
  bodies, attachments, and undispatched outbox rows; a schema
  version gates migrations. Acceptance: delete-and-resync
  reproduces equivalent state over a named field set (flags,
  folders, threads, event properties per CA-4's modeled set);
  migration from version N-1 is tested from v1.1 on.
- **SY-2 (MUST, switch-bar). Delta sync with push.** Mail syncs by
  persisted server state (delta changes from a watermark) with
  server push keeping the inbox live; push falls back to polling
  with backoff when the stream drops and recovers automatically
  (within 30 seconds p95 across a severed connection, 20 trials).
  Calendar and contacts poll by collection state token at most
  every 5 minutes focused, 15 minutes idle, plus a refresh on
  surface focus. A server-side state reset triggers a clean resync,
  never corruption. Non-normative: at Fastmail this is JMAP
  `Email/changes`, `Mailbox/changes`, and `Thread/changes` with
  EventSource push, and CalDAV ctag/sync-token polling; the
  requirement is written to the seam (C4).
- **SY-3 (MUST, switch-bar). Offline read, triage, and compose.**
  With no network: reading, search (local coverage), triage,
  folder navigation, compose, and calendar viewing work fully;
  RSVP answers, sends, and event edits queue durably and dispatch
  in order on reconnect. Conflicts resolve by server state
  ordering, never local wall clock, local losing ties, with a
  logged trace; nothing is silently dropped. Acceptance: an
  offline session performing the committed action matrix (triage,
  move, flag, send, RSVP, event create/edit/delete, folder
  lifecycle) converges correctly on reconnect.
- **SY-4 (MUST, switch-bar). Outbox discipline.** Every mutation
  flows through one durable outbox: the mutation and its queue row
  share a transaction; failures classify into typed reasons (auth,
  not-found, connection, server) with defined retry or surface
  behavior; auth failures route to ST-5. Acceptance: a crash
  between mutation and dispatch loses nothing; each failure class
  has a rendered, logged outcome.
- **SY-5 (MUST, switch-bar). Sync transparency.** A status-line
  state shows synced, syncing, offline, or backing off; initial
  sync and body backfill report progress; backfill throttles
  (bounded batches, newest first) and stays subordinate to
  interactive work. Acceptance: states render distinctly; a
  rate-limited backfill shows its warn state rather than stalling
  silently; QA-2 holds during backfill (QA-2's concurrent
  criterion).
- **SY-6 (MUST, switch-bar). Body strategy.** Headers and recent
  bodies sync eagerly; older bodies backfill newest first; a
  message opened before its body arrives fetches on demand (C1
  exception): a progress indicator within 100 ms, the body within
  3 seconds on a 10 Mbps link, and a named timeout path. Search
  coverage grows with backfill and says so (SR-7). Acceptance:
  open-before-backfill and timeout paths tested.
- **SY-7 (MUST, switch-bar). Single writer.** A second poplar
  process against the same store is detected at startup and either
  attaches read-only with a named banner or refuses with an
  actionable message (Phase 4 picks which). Acceptance: a second
  launch against a live store is tested and never corrupts the
  outbox.
- **SY-8 (MUST, switch-bar). Store recovery.** Corruption or a
  failed migration is detected at startup, surfaced per ER-1, and
  offers a rebuild-from-server path preserving the outbox and
  drafts; disk-full during any write degrades visibly and never
  corrupts. Acceptance: forced-corruption, failed-migration, and
  full-disk tests.
- **SY-10 (SHOULD). New-mail presentation.** Arrival updates counts
  and the list without moving the cursor; a presence-gated desktop
  notification mirrors CA-11's posture, configurable in ST-3.

## 12. Errors, logging, and observability (ER)

- **ER-1 (MUST, C7). One error seam.** Error-presentation types
  live in one package with unexported constructors; the only
  exported constructor is the seam, which writes the log before
  returning the view value. Acceptance: a vet-class check fails on
  construction of a user-facing error value outside the package; a
  test asserts each SY-4 failure class produces exactly one log
  line with operation, ids, and typed reason.
- **ER-2 (MUST, C7). Action trace.** Every user action logs at
  debug level with its outcome. Acceptance: a scripted session's
  log reconstructs the action sequence.
- **ER-3 (MUST, C7). Failure honesty.** No silent fallback: a
  degraded render, partial search coverage, a stale sync, and a
  queued-not-sent message each say so in the UI. Acceptance: each
  named state has a visible presentation and a test.
- **ER-4 (MUST, C7). Log lifecycle.** Bounded size with rotation, a
  documented path (the existing `~/.local/state/poplar/` home), and
  a stated redaction policy: message bodies never log; addresses
  and subjects log at debug only; tokens never log. Acceptance:
  rotation and redaction tested.

## 13. Quality attributes (QA)

Reference environment: the C10 gate platform, exact spec recorded in
the perf harness. CI runs the same harnesses with separately
recorded, scaled thresholds. *Provisional* follows the rule in "How
to read this document": regression baseline now, gate after the
Phase 4 spike.

- **QA-1 (MUST, vision differentiator 2). Startup.** Measured from
  exec to the first frame containing the first screenful of real
  rows (in-process timestamps behind a `--startup-trace` flag),
  median and p95 over 20 runs, against a store at the QA-5
  envelope, page cache warmed by one prior run. Gate: p95 under
  200 ms; design target 100 ms. Cold start (caches dropped): p95
  under 500 ms. *Provisional.*
- **QA-2 (MUST, vision differentiator 2). Interaction latency.**
  Two budgets. App-side: key-message receipt to render-buffer
  flush, instrumented in-process, p95 under 25 ms and p99 under
  40 ms over a scripted 500-keystroke session (60% list movement,
  15% folder switch, 15% reader open of cached bodies, 10% search
  keystrokes) at the QA-5 envelope; this number gates, and 20 ms
  is the design target the implementation aims at. End-to-end:
  keypress to pixel, measured once per platform by high-framerate
  capture on the reference terminal, reported, not gated. The
  app-side budget also holds while initial sync or a 20k-body
  backfill runs concurrently, and the store's concurrency design
  must make that true (ADR-0003 names the mechanism; a test fails
  on a UI-thread write). *Measured (revision 4): the Phase 4 spike
  read 22-25 ms p95 with zero SQLITE_BUSY under concurrent write,
  which set the gate at 25 ms.*
- **QA-3 (MUST, vision differentiator 2). Search latency.** A
  committed benchmark set of at least 20 queries in four classes
  against a 100k-message index. Per-class p95: single term and
  phrase, under 100 ms to a full viewport (first 50 rows);
  operator-filtered, under 200 ms; boolean and negation, under
  500 ms with a visible in-progress state. A query with no
  indexable positive term answers from a bounded scan with a
  documented cap, never a silent truncation. *Provisional.*
- **QA-4 (MUST, switch-bar). Sync convergence.** A scripted
  server-side mutation from a second client reflects in the local
  store and paints within 2 s median, 5 s p95, over 50 trials
  (timestamped at the mutating call and at the frame containing
  the change) under push. Calendar and contacts converge within
  one poll cadence plus one poll duration (cadence per SY-2).
  *Provisional (the seconds, not the mechanism).*
- **QA-5 (MUST, switch-bar). Scale envelope.** All targets hold at
  100k messages and 5k events (roughly three times the target
  account). Steady-state RSS under 250 MB after scrolling the full
  list, opening 50 threads, and visiting all four surfaces; RSS
  growth under 5% across a 30-minute scripted soak. Store size at
  or under 1.7x retained body bytes, counting the index and all
  metadata. *Revision 4 restates the storage criterion, which read
  "overhead under 15%" through revision 3. The Phase 4 spike
  measured 53% overhead on a real 35,837-message archive amplified
  to 100k, so the old number was unmeetable by any schema and the
  ratio against retained bodies is the honest form. Revision 5
  (pass 1c gate, Geoff, 2026-08-19) re-ratifies the bound at 1.7x:
  the corrected full-envelope measurement read 1.63x, of which the
  prefix='2 3 4' index is the marginal 0.06, a deliberate
  storage-for-latency trade that took QA-2's prefix-search p99
  from 256 ms to ~1.1 ms. The metric and denominator stand; the
  ruling records the trade rather than redefining the measure.*
- **QA-6 (MUST, switch-bar). Data safety.** A seeded kill harness
  runs a fixed 30-action script (triage, bulk, compose, send,
  RSVP, event edit, folder rename) and SIGKILLs at 200
  pseudorandom points, three seeds, in CI. After each kill,
  restart asserts: store integrity check passes; no outbox row
  without its committed mutation and none the reverse; the draft
  matches the last autosave; index count equals message count.
  Corruption is any assertion failure.
- **QA-7 (MUST, vision differentiator 6). Render determinism.** For
  a fixed tuple (width, height, color profile, capability profile,
  locale, TZ), rendered output is byte-identical across runs and
  across the CI matrix. Capability-dependent output is
  golden-tested once per declared profile; the profile is a test
  input, never sniffed in tests.
- **QA-8 (SHOULD). Idle posture.** Idle CPU mean under 0.5% over
  five minutes with push connected, sampled from the process
  stats, and no timer wakeups attributable to polling while the
  stream is up.
- **QA-9 (MUST, vision differentiator 1). Render quality.** The v1
  pipeline holds at or above the Phase 1 measured usable-or-better
  rates on the coder core (github-ci 100%, personal 88%,
  list-patch 84%, transactional 76%; 87% aggregate), regraded
  before the 1.0 gate on the committed license-clean corpus plus
  the local corpus, using the Phase 1 grading protocol. Per-class
  regression beyond the verdict's ±5-point noise band is a gate
  failure.
- **QA-10 (MUST, vision differentiator 6). Showcase quality.** The
  conventions gates (go-conventions, elm-conventions, comment
  voice) run in CI on every commit from the first build pass;
  `internal/ui` carries package-level architecture documentation;
  a public README and architecture map ship with 1.0.

## 14. Explicit scope rulings

Decisions the survey or review flagged as silently undefined, ruled:

| Item | Ruling | Basis |
|---|---|---|
| Labels (multi-membership views) | LATER (FO-5) | Target account is folders-mode; schema stays open |
| Saved searches | LATER (SR-6) | Not switch-bar; grammar admits them |
| Snooze / thread mute | LATER (LT-7) | No usage on target account; not switch-bar |
| Sieve/server filter management | Non-goal v1 | Settings-level task; Fastmail web remains its home |
| Sender allow/block lists | Non-goal v1 | Junk verb (LT-2) covers the daily need |
| Mail import (mbox/maildir) | LATER (ST-4) | Server is source of record |
| Contact add/edit | LATER (CT-4) | Not routine mail work; store keeps write paths |
| Vacation auto-reply | Non-goal v1 | Rare, settings-level; Fastmail web |
| Export message as .eml | SHOULD (RD-8) | Cheap beside raw-source retention |
| Sort options beyond date | Non-goal v1 | One opinionated order, stated in LT-1 |
| AI tidy / snippets | LATER (CO-10) | Explicit-key seam preserved |
| Forward-as-attachment | LATER (CO-2) | Inline forward with attachments covers the need |
| RSVP undo | Excluded from UX-9 | Re-answering is the correction; CA-6 re-send tested |
| Reminders while poplar is closed | Non-goal | C3 forbids a daemon; phone and web notifications remain their home (CA-7) |
| Grid calendar views in v1 | Gate call (CA-3) | Agenda covers the strict switch bar; owner rules |
| Booking pages, free/busy aids | LATER (CA-12) | Different product surface |
| Year view | Non-goal | Neither incumbent ships one |
| PGP/S-MIME | LATER seams (CO-11), feature non-goal v1 | Vision horizon 6: seams must not weld it out |
| Multi-account | LATER via C4 seams | Vision horizon 1: schema and UI state account-keyed from v1 |
| Plugins, remaps, theme files, Gmail/IMAP, non-terminal UI | Non-goals | Vision, unchanged |

## 15. Build-order derivation

Phase 5 derives its pass order from this spine. Every MUST maps to
exactly one step; the QA harnesses run from the step that creates
their subject, never as a terminal pass (the legacy client died of
deferred performance).

1. **Foundation.** Store schema (mail, calendar, contacts, outbox;
   account-keyed per C4), sync engine, outbox, search index, role
   classification, single-writer and recovery (SY-1 through SY-8
   core), and the perf harness (QA-1/2/3 instrumentation) with
   baselines recorded from the first list render. The calendar
   store schema and CalDAV engine shape are designed here even
   though calendar views land in step 5.
2. **Design language and shell.** UX-1 through UX-5, UX-7 through
   UX-9; ST-1 through ST-3, ST-5. Onboarding lands here because
   every later pass needs a configured, authenticated client.
3. **Mail read path.** LT, TH, RD (pipeline, fact check, fallbacks,
   copy-out), SR-1 through SR-3, SR-7, FO. The license-clean
   specimen corpus (RD-1) is built here. QA-9's grading harness
   runs from this step.
4. **Compose path.** CO, CT-1, CT-2.
5. **Calendar.** CA-1, CA-2, CA-4, CA-6 through CA-8, wired to the
   step 1 schema; the RSVP bridge closes the loop through the
   reader (step 3) and outbox (step 1).
6. **Hardening and polish.** SY-3 full offline matrix, QA-6 kill
   harness at full scope, the SHOULD set as schedule allows, and
   the 1.0 gates (QA-9 regrade, QA-10 artifacts).

## 16. Open verification items

Carried to Phase 4, none blocking requirement approval:

1. CalDAV participation-status write at Fastmail: does the server
   send the iTIP reply, and how does it behave on recurring events?
   (Needs a calendar-scoped credential; CA-6's mechanism branch and
   the double-send fixture resolve here.)
2. Free/busy query support over CalDAV at Fastmail (informs CA-12's
   seams only).
3. JMAP draft body-update semantics and Fastmail's observed web
   autosave behavior (CO-6's server-push details).
4. JMAP `Identity` alias behavior, verified directly (CO-3).
5. The Phase 4 measurement spike: a real archive at the QA-5
   envelope measured through the QA-1/2/3 harnesses; its numbers
   replace every *provisional* target and are a blocking input to
   Phase 5 planning.
6. The CO-5 HTML allowlist, fixed from Fastmail and Gmail stripping
   behavior during Phase 4.
