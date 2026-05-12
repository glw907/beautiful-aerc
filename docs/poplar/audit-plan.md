# Poplar Audit Plan

The path from pre-beta to soak runs through four directed audits.
`STATUS.md` tracks which audit is next; this document defines what
each audit covers and how to know it returned empty.

The soak-entry rule: enter beta soak when a full audit cycle returns
no findings. Version number, calendar date, and "feels ready" are
not the rule.

Phase B splits into B.1 (Elm + bubbletea v2 conformance) and B.2
(general structural integrity). The Elm/bubbletea surface is big
enough that compressing it into one focus item underweights it
and risks fatigue-driven skimming. Other phases stay single.

## Why phased

Auditing everything continuously costs more attention than it
returns, and the same lens scanning unchanged code degrades into
noise. Auditing at phase boundaries with a per-phase focus list is
reproducible and bounded. Each phase's audit checks the risk that
phase introduced; the final audit is the comprehensive sweep.

The audits are themselves passes. They consume context, produce
findings or an empty result, and gate the next batch of work.
A blocking finding queues a remediation sub-pass (e.g. `26.1`)
before the next audit phase runs.

## Phase A — bug-fix completeness

**Trigger.** Passes 23–25 ship; `make check` green.

**Question.** Did the bug fixes leave sibling hazards behind, or
miss a callsite during mechanical folds?

**Focuses.**

- Mail-infrastructure regression sweep. Re-run the patterns that
  found #51, #52, #53: dead-handle reuse, sentinel-error gaps,
  silent-success returns, unbounded retry on a stale cached
  pointer. Walk every `mail.Backend` method against the IDLE-
  reconnect lens. JMAP `Connect` error paths, IMAP `OpenFolder`
  mid-IDLE switch races, cache drainer's conflict-matrix coverage
  of any new sentinels.
- Config-validator completeness. Walk every `AccountConfig` field
  and ask: what happens if it is empty, malformed, of the wrong
  type, or a typo of an adjacent field? The MockBackend hazard
  surfaced because empty `provider` was a valid input; others
  likely exist (`smtp.host` on non-preset, `oauth.client-id`
  when `oauth-store = "keyring"`, etc.).
- Defensive clamps. After Pass 25 drops the clamps in
  `backoff.Exponential`, grep `internal/` for sibling defensive
  checks between internal callers that the no-defensive-checks
  rule also forbids.

## Phase B.1 — Elm + bubbletea v2 conformance

**Trigger.** Passes 27–29 ship. Catkin is all-value; the
`Editor` wrapper is gone; `app.go` is decomposed.

**Question.** Does the UI tree conform to the project's Elm and
bubbletea-v2 contracts after the structural changes?

The canonical surface for both contracts is split across three
files: the `elm-conventions` skill (Elm architecture rules);
`docs/poplar/bubbletea-conventions.md` (idiomatic bubbletea —
size contract, View self-guard, wordwrap composition, planning
+ review checklists); `.claude/rules/ui-invariants.md` (the
component + UX binding facts). The audit pulls from those —
this file lists what to walk against them, not what the
contracts say.

**Focuses.**

- Receiver discipline. Every `Model` in `internal/ui/`,
  `internal/catkin/`, and `internal/ui/wizard/` has value
  receivers on `Init`/`Update`/`View`, plus either value
  receivers throughout or every state mutation gated behind a
  Msg handled in `Update`. Mixed receivers anywhere is the
  same straddle catkin had pre-27.
- Cmd-as-only-IO. Every I/O call site (cache reads, backend
  RPCs, file open, subprocess, timer arming) sits inside a
  `tea.Cmd` returned from `Update` or `Init`, not inline in
  `Update`'s body and not inside `View`.
- Msg vocabulary discipline. Children signal parents via
  exported `Msg` types in `<subpkg>/msgs.go`. No state mirror
  back-channels (a child `Msg` that copies the child's state
  back to the parent for the parent to re-render). Parents
  read children via accessor methods after delegation.
- Size contract. `View()` is self-guarded via `clipPane` /
  equivalent — never trusts the caller's width. `SetSize`
  propagates via `WindowSizeMsg` through every subtree.
  `bubbles/v2/list` consumers honor the list's own size
  contract (size set before items are populated).
- `JoinHorizontal` ban under SPUA-A. The ADR-0084 ban on
  `lipgloss.JoinHorizontal` for icon-bearing rows when
  `spuaCellWidth != 1` is still honored — grep for new
  `JoinHorizontal` calls introduced post-Pass 17 and verify
  each is row-by-row `strings.Join` of pre-padded children.
- Cursor hoisting. Every cursor-bearing leaf exposes its
  cursor via a `Cursor() *tea.Cursor` accessor; the parent
  composes them in `App.frameCursor()`. No call to
  `SetVirtualCursor(true)` survives — every textinput /
  textarea must call `SetVirtualCursor(false)` at
  construction.
- Paste routing. The paste contract in ADR-0189b: address
  fields atomic-emit chips; catkin splices and wraps URL
  tokens as markdown links. Verify no new paste handler
  short-circuits this.
- `bubbles/v2/<pkg>` analogue preference. Any new UI surface
  added in Batch 2 prefers a `bubbles/v2` analogue
  (`list`/`tree`/`textinput`/`textarea`/`filepicker`/`help`)
  unless a deviation ADR is current. New deviations without
  ADRs are findings.
- Deviation ADRs current. ADR-0200 (help), 0201 (helppopover
  border), 0194 (list styles), 0195 (filepicker), 0199 (list
  delegate) describe live deviations from upstream `bubbles/v2`.
  Walk each: does the deviation still match what the code does?
  Outdated deviation ADRs are Phase Final invariant-rot
  findings if they survive here.

## Phase B.2 — general structural integrity

**Trigger.** Phase B.1 returns empty.

**Question.** Does the post-refactor structure hold up on
non-Elm dimensions — file size, interface design, package
layering?

**Focuses.**

- `App.go` decomposition regression. The 874-line `Update` is
  now several `update<Screen>` methods. Dispatch is exhaustive
  (every key/msg still routed). No method exceeds roughly 150
  lines (otherwise the god object moved rather than dissolved).
  No method calls back into a sibling `update<Screen>`. Back-
  channel coupling between screen controllers is worse than
  the original god switch.
- File-size budget. After the `app.go` peel, no file in
  `internal/ui/` exceeds ~600 lines. Where one does, name the
  reason; it should be a `Model` whose state genuinely
  requires that surface (e.g. `messagelist`).
- Interface count. With `Editor` deleted, count interfaces in
  `internal/`. If the count rose, name the new ones and the
  seam each represents. New single-impl interfaces without a
  named test fake or DI seam are the same anti-pattern the
  codebase already documents.
- Package-boundary leaks. After the `app.go` split, no imports
  from `account` / `compose` / `reader` / `messagelist` /
  `sidebar` / `wizard` point back at `internal/ui` directly.
  Subpackages cannot import the parent.

## Phase C — feature surface

**Trigger.** Passes 32–35 ship, or whatever subset lands.

**Question.** Did feature work introduce behavioral hazards in
code paths whose failure modes the project has not yet seen?

**Focuses.**

- OAuth refresh against the #53 lens. Token refresh on auth
  failure mid-IDLE has to re-resolve and retry, not hammer a
  stale token. Same dead-handle audit, different cached
  resource.
- Mouse hit-test surface. `v.OnMouse` declares clickable
  regions per frame. Audit stale coordinate math after
  `WindowSizeMsg`, hit-test overlap across the overlay cascade
  (confirm > conflict > outbox > help > linkpicker > attachpicker >
  movepicker > form > popover), wheel-scroll routing to the
  wrong viewport when a modal is open.
- `v.ProgressBar` lifecycle. Set/unset is per-frame. A long-
  running op that completes mid-frame must reach the next
  frame's `ProgressBar = nil`. Orphaned terminal-title bars
  after an op finishes are an OSC-9;4-shaped #53.

## Phase D — database

**Trigger.** Phase C returns empty.

**Question.** The per-account SQLite cache is the only thing the
project promises not to corrupt across versions. Does the schema,
the migration ladder, the drainer's transactional discipline, and
the on-disk-shape contract hold under the failure modes the user
will hit but the test suite hasn't?

**Focuses.**

- Schema migration ladder. Walk every `migrateVN` step against
  its predecessor: column adds, drops, renames, FK changes,
  index churn. A migration that ran clean on a fresh dev cache
  may not on a real one with 80k messages, 12k attachments,
  and a non-empty outbox. The `messages_fts` rebuild step
  (schema v11) is the obvious one — others are mid-ladder.
- Transactional boundaries. Every drainer op (Flag / Move /
  Destroy / Append / Send / EmptyFolder / Send-Later) flips
  `ui_flags` / `ui_hide` and inserts the outbox row in one
  tx. Audit each for a partial-commit window: a path that
  could leave the optimistic flip without its outbox row, or
  vice versa. `OpDone` cleanup must also be transactional with
  any compensating-state delete (draft FK `SET NULL`, retry-
  counter reset).
- FTS5 consistency. `messages_fts` is rowid-linked to
  `messages.id`. Every upsert and storeBody path that touches
  body text must reach the FTS write helpers inside the same
  tx. Backfill (`Backfiller`) is the largest body writer —
  audit its tx shape against the search invariants. A drift
  here means stale or missing search hits with no surface
  symptom until the user notices.
- Schema version probe + refusal. Old binaries against newer
  caches must refuse to open rather than silently corrupt
  (e.g. by writing schema-vN columns that vN+1 has renamed).
  Walk every `cache.Open` precondition for the "future
  schema" branch.
- UIDVALIDITY re-key + IMAP scan-and-diff. The protocol
  semantics that motivated the cache (ADR-0118): a mid-flight
  UIDVALIDITY change has to invalidate the per-folder UID
  mapping atomically. Audit the re-key path against
  in-flight outbox rows scoped by the old UID.
- File-on-disk shape. Attachment storage is the cache's only
  out-of-DB persistence (chunked blob? path-keyed?). Audit
  cleanup on `Destroy` / EmptyFolder. Orphaned blobs are
  silent disk-leak; the inverse (DB row pointing at a missing
  blob) is a render-time error with no recovery path.
- Drainer conflict matrix recheck. Audit A covered the
  error-routing side (`errors.Is(err, mail.ErrConnection)`).
  This pass audits the *state* side: every conflict path
  reaches a terminal `OpConflict` row and never an undead
  retry-loop. Cross-reference against the BACKLOG conflict
  items still open after Audit A.

## Phase Final — comprehensive pre-soak

**Trigger.** Phase D returns empty.

**Question.** Across every dimension the project cares about, is
anything left to fix before stability becomes the priority?

**Focuses.** All Phase A/B/C/D lenses, plus three not covered
upstream:

- Test-infrastructure quality. Real coverage of the dangerous
  paths: drainer conflict matrix, IDLE reconnect, outbox cancel
  during in-flight, OAuth refresh-on-fail. Fakes that obscure
  rather than reveal — a `fakeBackend` with a silent-success
  `Send` is structurally the MockBackend hazard. Snapshot tests
  where the golden was updated without inspection.
- Security and credential handling. TLS-verification surface
  (`InsecureTLS` opt-in paths, self-signed defaults). Secret-
  in-memory lifetime: cached `password` strings, OAuth refresh
  tokens, zeroing on disconnect. The `password-cmd` subprocess
  interface and what it leaks to process listings.
- Voice and documentation rot. The `go-comment-voice` 32-tell
  catalogue applied to the whole codebase, not just recent
  diffs. ADR voice. Invariant drift: `docs/poplar/invariants.md`
  and `.claude/rules/` against the code as it stands. A
  silently outdated invariant is the highest-cost failure mode
  for future audits.

## Audit mechanics

1. Read the focuses for the active phase.
2. For each focus, run the named search or walk. Record what was
   looked at, not just what was found.
3. Categorize findings: bug → `BACKLOG.md` with priority;
   architectural flaw → `ROADMAP.md` as a project; small nit →
   folded inline into the next pass.
4. Blocking findings — silent data loss, credential leak,
   behavior the user cannot recover from without restart — queue
   a remediation sub-pass before the next audit phase.
5. Empty audit: mark the phase complete in `STATUS.md` with the
   date and a one-line summary.

The record matters more than the result. A future audit comparing
against an empty prior is weaker evidence than one comparing
against `"Phase A 2026-05-12 → 0 findings; Phase A 2026-06-03 → 2
findings (#54, #55), both fixed in Pass 25.1"`.

## Failure modes for the audit itself

- Same lens, same code, expecting different findings. Re-running
  Phase A over unchanged code is noise. Each audit's focuses are
  pegged to the phase that just changed.
- Findings logged but never scheduled. Blocking findings gate
  the next phase. Non-blocking findings still land within two
  passes of being logged, or the audit was theatre.
- Vibes-as-finding. "This feels off" is a hypothesis, not a
  finding. Convert to a specific check (walk every X for Y) and
  run it. If the check returns empty, the hypothesis is closed.
