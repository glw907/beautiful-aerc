# Poplar Re-founding: STATUS

**Track:** The re-founding. The sole active track. Charter:
`docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md`. It
supersedes the 2026-05-29 rebuild spec track; the rebuild's functional
spec and gap analysis are research inputs, and the dogfood client
stays archived at tag `poplar-legacy` and branch `legacy`.

This file owns the phase cursor and the next starter prompt. Read it
first on "continue".

## Phase-end ritual

Runs at every phase close, whether or not the starter prompt restates
it. Updating this STATUS is step one and is never optional.

1. Append a `Phase N outcomes` block to Current state below,
   recording the settled decisions and artifact paths.
2. Run `deadcode ./...` and record the count and every remaining
   entry's justification in the outcomes block. A package this phase
   composed into `cmd/poplar` (`jmap`, `sync`, `outbox`, and so on)
   still showing an unreached entry in its own composed path is a
   wiring gap, not a justification.
3. Replace the `## Next` section with the next phase's starter prompt.
4. Mark the phase `[done]` and the next `[next]` on the roadmap line.
5. Commit the artifacts and this STATUS, then push.
6. Refresh the `project_poplar_refounding_initiative` memory only when
   a load-bearing decision changed. The cursor lives here, not in
   memory.

## Current state (2026-07-28)

Phase 5 is in progress. Pass 1 (foundation) is built and at its
gate; pass 1b (integration and hardening) is the next action,
below.

Pass 1 outcomes (2026-07-28), commits 28e2905 through a8fafb2:
- 50 commits, 145 files, 20,699 insertions. `make check` verified
  green by the orchestrator at the close, all ten steps executing.
  Fourteen planned tasks plus three promoted mid-pass: 7b (moving
  the scriptable fake out of the production package before task 9
  imported it), 1b (schema and seam corrections from the ADR audit
  and the store survey), and 11b (the startup sweep for orphaned
  intents, found by the kill harness).
- What exists: the store (schema v1, migrations, single writer with
  two lanes, read pool with a compile-enforced read-only handle,
  FTS5, role classification), `internal/uerr`, the backend seam and
  its scriptable fake, a JMAP mail source verified against Geoff's
  live Fastmail account, the sync engine, the durable outbox, the
  instance lock and recovery path, `cmd/poplar`, the QA-1/2/3 perf
  harness, and the QA-6 kill harness at smoke scope.
- **The engines are not wired.** `cmd/poplar` opens the store, runs
  migrations, sweeps orphaned intents, and idles. It never starts a
  JMAP session, the sync worker, or the dispatcher. `deadcode`
  reports 203 unreachable functions, 143 of them in outbox, jmap,
  and sync, because nothing calls them. So the pass's claim of "a
  sync engine that converges against the live server" is true in
  tests and false of the program. Nobody decided this; no task was
  ever assigned the entry-point wiring. It is pass 1b's first
  acceptance criterion.
- **QA-2's number is not comparable to its baseline.** Measured on a
  quiet machine: QA-2 p95 457.9us quiescent and 657.2us under write
  against a 25ms gate, with p99 at 13.5ms against 40ms. The spike
  baseline was 22-25ms p95. The gap is the corpus: the seeder gives
  each message a 20-word body of a few hundred bytes where real mail
  averages ~9.2KB. QA-3 IS comparable (0.9-2.3ms against the spike's
  0.9-4.5ms) because the seeder draws realistic word distributions,
  so the FTS index has roughly the right term structure.
- Defect classes this pass produced, worth carrying as priors: six
  variants of a claimed outbox intent becoming unreachable, each
  through a different mechanism; and three pieces of dead surface
  that looked live (FO-1's role classifier with no caller,
  `uerr.LogHealth` with no caller, and the write-call analyzer,
  which matched no symbol `internal/store` exports and therefore
  guarded nothing for the entire pass while its own tests passed
  against invented names).
- Review shape that worked: a four-lens pass-end fan-out found two
  Criticals the per-task reviews missed, and every verification step
  that was asked to prove revert-sensitivity by experiment found
  something the reading missed. Three agents were lost to
  StructuredOutput schema retry caps, each silently, including the
  silent-failure lens on its first run.
- Owner rulings during the pass: typed models at the backend seam
  over `map[string]any`; adopt the JMAP protocol code as poplar's
  own package named `jmap`; Stalwart as the second validation
  server; right way before easy way; nothing the archived client
  picked is assumed correct; always research alternatives.
- Research committed under `docs/poplar/research/`: the TUI
  design-iteration survey, the JMAP library decision, the JMAP
  adoption assessment, the JMAP test inventory (46 numbered items,
  12 divergence tests), and the SQLite driver audit.

Phase 5 boundary outcomes (2026-07-27), commits 041eb32 through
7f997ef:
- Branch `legacy` exists at last. It was created and pushed from
  the final pre-boundary master commit (649cd0d) before anything
  was deleted, which makes the charter's, CLAUDE.md's, the
  routing rule's, and this STATUS's standing citations true for
  the first time and preserves the four spike tools the tag
  predates. Salvage is copy-with-rewrite from it.
- Master cleared: 1168 tracked files to 331. The three
  PostToolUse hooks left with their settings.json registrations
  in the same commit. Beyond the design's list, the aerc-era
  stow package, the archived client's README and screenshot
  assets, ROADMAP.md, and accounts.toml.example went too; none
  had a live consumer.
- The machine is up and `make check` is green on the empty
  module with every step executing: one local tier, the four
  analyzers as a multichecker binary with analysistest fixtures,
  tools pinned through an isolated tools/go.mod, CI carrying
  race plus the nightly jobs with the two unbuilt harnesses
  labelled as placeholders. The audit's two silent-skip guards
  are inverted and verified failing.
- Requirements are at revision 4, carrying the five ratified
  amendments that until now lived only in this STATUS while the
  spec printed superseded numbers. ADR-0001, ADR-0011, and
  ADR-0012 have revision blocks. Two extra defects were caught
  and fixed in passing: ADR-0001's Decision still asserted
  file-per-account and FTS5 external-content tables, and
  technical design section 18 still listed the EventSource auth
  risk as open while ADR-0005 recorded the probe closing it.
- Both workstation fixes are committed to dotfiles (60c0aed).
  `~/.claude/docs/go-comment-voice.md` turned out to be a
  dangling symlink, the root of the audit's finding 1, as was
  `prose-voice.md`; both are gone. The go-conventions Makefile
  example also lost its skip-quietly lint guard, the pattern
  that let poplar's linter sit broken and green for months.
- The pass 1 plan is authored and committed
  (`docs/superpowers/plans/2026-07-27-pass-1-foundation.md`),
  thirteen tasks. It settles five questions the specs left open;
  the load-bearing one is that pass 1 ships a real JMAP read
  path rather than validating the sync engine against the fake
  alone. Flag for the pass gate if that reading is wrong.

Phase 5 machine-gate outcomes (2026-07-27):
- Two Geoff directives arrived at phase open, both ratified at
  the gate: solid mouse support from day one, and a keybinding
  machinery ruling. Mouse is ADR-0017 (pointer as accelerator
  over the keyboard-complete grammar, eleven-row vocabulary,
  immediate single-click, two-grain hit-testing, message-level
  testing); it amends UX-6 from SHOULD to MUST in requirements
  revision 4. Keybindings settle on bubbles/key with
  registry-derived help.KeyMap (crush's production pattern).
- Approved artifacts, all committed: the build machine design
  (`2026-07-27-poplar-build-machine.md`, revision 2), ADR-0017,
  and the tooling audit
  (`docs/poplar/research/2026-07-27-phase5-tooling-audit.md`,
  revision 2). Evidence base: five research dispatches (in-repo
  inventory, Go toolchain, Charm ecosystem incl. a crush
  deep-dive, keybinding machinery, mouse) under the standing
  live-research directive.
- Audit verdict ratified: nothing carries forward by default.
  Broken today: the dangling voice catalogue (six citing
  artifacts), golangci-lint v1 config vs v2 binary with the
  failure swallowed, dead agent model pins, a hookify rule
  quoting deleted doctrine, and the phantom `legacy` branch
  (cited everywhere, exists nowhere; the tag predates the spike
  tools). The boundary ritual creates and pushes `legacy` from
  the last pre-boundary master commit before deleting anything.
- Gate composition ratified: one-tier local gate (build incl.
  darwin cross-compile, golangci-lint v2 explicit enables, four
  design analyzers via a tools-module multichecker, Vale,
  skipcheck, tests, non-race perf step); CI runs the gate
  verbatim plus race, darwin goldens (QA-7), and nightly
  govulncheck/deadcode/soak/idle; kill harness scales with
  subjects, full scope at pass 6. Mutation testing dropped;
  voice consolidates on Vale; gosec suppressions carry rule id
  plus reason and are reviewer-read.
- Spec amendments ratified for the boundary: requirements
  revision 4 (the Phase 4 ratified numbers plus the UX-6
  amendment) and revision blocks on ADR-0001 (one store file),
  ADR-0011 (LayoutMode pane rectangles; registry binds pointer
  targets), ADR-0012 (basic key disambiguation is v2's default;
  decline covers only opt-in enhancements).
- Two workstation fixes owed outside the repo:
  bubbletea-conventions.md relocates to `~/.claude/docs/` with
  the elm-conventions skill repointed; go-conventions' example
  golangci config updates to the v2 schema.
- Review shape that worked again: two Opus adversarial lenses
  (spec-consistency, evidence-and-feasibility; ~40 findings, 9
  blockers, all folded) over revision 1 of the three artifacts.
- Watch item for wireframe passes: lazygit retracted digit
  panel-switching under user pushback; poplar's `1`-`4` stands
  but digit semantics stay under the eye.

Phase 4 outcomes:
- Design approved at the gate (Geoff, 2026-07-27), with the
  presented recommendations ratified as defaults: CA-3 grid views
  stay SHOULD (agenda covers the switch bar); QA-2 gates at the
  measured-informed 25 ms p95 with 20 ms as the design target;
  QA-5's overhead criterion restates as store size at or under
  1.6x retained body bytes; CO-6's loss bound amends to debounce
  plus the ~50 ms writer admission ceiling; the UX-3 analyzer rule
  strengthens to all non-ASCII literals repo-wide with
  `internal/catkin` exempt; the deferred probes are accepted with
  their named defaults.
- Artifacts committed: technical design revision 2
  (`2026-07-27-poplar-technical-design.md`), sixteen ADRs
  (`specs/adr/`), the design language revision 2
  (`2026-07-27-poplar-design-language.md`), the CO-5 HTML
  allowlist (`2026-07-27-poplar-html-allowlist.md`), the C9
  library survey and the measurement-spike report (under
  `docs/poplar/research/`), and the `cmd/perfspike` tool.
- Gate directive (Geoff, 2026-07-27), refined same day: a clear
  responsive-design plan, fully functional at traditional sizes
  and taking real advantage of large windows, starting from
  research into actual terminal-size usage. Encoded as the
  design language's section 9: the ladder principle (a complete
  strictly-necessary core plus additive capability rungs; a rung
  only where capability changes; one change per boundary), three
  rungs plus a floor state (spartan 60-99 complete at 80×24,
  standard 100-139 adds the sidebar, wide 140+ adds the split
  with capped centered measures), short-height first-class
  treatment, and the coverage-cliff formulas inside rungs.
  Evidence committed:
  `docs/poplar/research/2026-07-27-terminal-size-survey.md`
  (no telemetry exists anywhere; defaults, shipped TUI
  breakpoints, and split math are the base; the 2-3% ultrawide
  tail ruled out a fourth rung).
- Review shape that worked: three adversarial Opus lenses (~85
  findings, 8 blockers, all folded) plus a fresh-context
  verification pass; the blockers were real (unservable list
  query, missing optimistic-paint mechanism, unanchored draft
  identity, unconstructible FTS shape, homeless compose path, a
  foreclosed ST-4).
- Load-bearing live findings: Fastmail silently no-ops draft
  body updates (autosave must use atomic create-and-destroy,
  probe-verified); Identity/get is a curated list (reply
  identity needs poplar's matcher); EventSource Bearer auth
  works (the 2021 401 report is dead); contacts are RFC 9610
  ContactCard only.
- Spike numbers (35,837 real messages amplified to 100k, this
  machine): startup ~5 ms once quick_check is event-driven
  (14.5 s synchronous at 924 MB, moved off the launch path);
  QA-2 22-25 ms p95 with zero SQLITE_BUSY under concurrent
  write; search 0.9-4.5 ms p95 (20x headroom); storage overhead
  53% of body bytes (drove the QA-5 restatement).
- Phase 5 obligations carried: CalDAV RSVP and free/busy probes
  when the calendar-scoped token lands (still owed; default
  branch: poplar sends iMIP); the go-ical/golang-ical bake-off
  and rrule DST fixtures before the calendar pass; the clipboard
  spike on the gate box; per-class wireframes before each
  screen's pass; column formulas re-derived from the spike
  harvest.

Phase 3 outcomes:
- Requirements spec approved and committed:
  `docs/superpowers/specs/2026-07-27-poplar-requirements.md`
  (revision 3). It binds Phases 4 and 5: ten constraints plus C11,
  ~95 prioritized requirements with acceptance criteria, measured
  quality attributes, explicit scope rulings, and the Phase 5
  build-order spine. Revision 2 folded a three-lens adversarial
  review (~90 findings).
- Grounding survey committed:
  `docs/poplar/research/2026-07-27-phase3-grounding-survey.md`.
  Seven live research dispatches plus a live JMAP probe of the
  target account.
- Directives received during the phase, all encoded as spec
  constraints: calendar is a first-class v1 surface with full
  requirements (C4/section 10; "mail just isn't that useful without
  it"); local-first from the first build, JMAP never on the
  interactive path (C1; the legacy direct-JMAP path was painfully
  slow); a unified design language before any screen, cairn-cms as
  the exemplar (C5); mail/calendar/contacts/config deeply unified
  in keys, look, and concepts, config as an in-app surface (C6);
  forward-looking lean stance, exhaustive prior-art survey opening
  Phase 4 (C11, C9).
- Load-bearing backend fact: Fastmail exposes calendars over CalDAV
  only (their dev docs commit to JMAP calendars only after the spec
  finalizes; draft-ietf-jmap-calendars sits approved but held in
  the RFC Editor queue). v1 calendar rides CalDAV with a designed
  JMAP upgrade seam.
- Account facts (live probe): folders-mode, 14 flat mailboxes,
  ~36k messages, JMAP contacts capability present, no snooze usage,
  scheduled folder empty. QA envelope set at 100k messages.
- Gate calls ratified with approval: agenda view MUST with grid
  views SHOULD (CA-3, still listed as an owner call if taste says
  otherwise), recurring-series editing SHOULD, reminders while
  poplar is closed a ruled non-goal, labels/snooze/saved-searches
  LATER.
- Phase 4 needs from Geoff: a calendar-scoped Fastmail API token
  for the CalDAV RSVP and free/busy probes (spec section 16).

Phase 2 outcomes:
- Vision doc approved and committed:
  `docs/superpowers/specs/2026-07-27-poplar-vision.md`. It binds
  Phases 3 through 5.
- Switch bar: full mail replacement for one Fastmail/JMAP account.
  Triage, threading, search, folder management, Catkin compose,
  contacts autocomplete, multiple identities/aliases, ICS-first
  calendar RSVP from the reader, offline read and triage.
- Named product goals: near-instant speed on every operation
  (local-first store, JMAP as sync layer; Phase 3 sets the numbers);
  self-containment (one binary, no external editor, indexer, or
  delivery agent, no companion daemon); attractive, research-grounded
  UI (wireframes before any screen build); current-stack posture
  (Phase 4 freshness audit; no dependency inherits its legacy pin).
- Non-goals for v1: user configurability, PGP/S-MIME, plugin or
  scripting systems, external-editor integration, multi-account,
  Gmail or generic IMAP, any non-terminal UI.
- The nvim companion plugin is dropped entirely (Geoff, 2026-07-27),
  off the horizon register; its workstation memory is deleted.
- Knowable-horizon register, seven items, recorded in the doc:
  multi-account (early post-v1, the named first priority), Gmail
  OAuth backend, flag-a-bad-render loop (#63), capture-mailbox
  corpus, Catkin spinoff, encryption seams, contacts micro-highlight.
- Phase 3 directive (Geoff): open with a grounding survey of
  Protonmail and actively used TUI clients covering UX patterns as
  well as features, so requirements are complete in advance and
  their priorities set the implementation order.
- Phase 5 directive (Geoff, 2026-07-27): audit the Go and bubbletea
  Claude infrastructure (CLAUDE.md, the go-conventions and
  elm-conventions skills, linters, the `make check` gate, the
  implementer and reviewer agents) against current best practice for
  high-quality, idiomatic Go before the build machine is finalized.
  Pairs with the vision's current-stack posture: the tooling gets the
  same freshness audit as the dependencies.
- Standing directive, all remaining phases (Geoff, 2026-07-27): the
  Go, Charm, and mail-library realm evolves fast. Verify releases,
  idioms, and best practices by live research at the point of use;
  model priors alone are stale by assumption. This binds the Phase 3
  survey, the Phase 4 freshness audit and design decisions, and the
  Phase 5 tooling audit.

Phase 1 outcomes:
- Verdict: viable inside a measured boundary. Deterministic rules
  reach 87% usable-or-better on the coder-relevant core (github-ci
  100%, personal 88%, list-patch 84%, transactional 76%), 41% on the
  commercial-and-calendar tail; calendar is an artifact (the product
  renders invites from the text/calendar part, not HTML). Verdict doc:
  `docs/poplar/research/2026-07-19-rendering-bet-verdict.md`.
- Settled constraints: no LLM in the render path (runtime is
  deterministic Go; LLM works offline in the improve loop);
  rules are declarative, named, provenance-carrying, and traceable
  so people and LLMs can improve them; both bind Phase 4.
- Committed artifacts: readability principles doc (grade definitions
  included), rendering-bet verdict, public-corpus survey (no modern
  public corpus exists; capture mailbox is the path), spike code
  (corpusharvest, renderspike, llm-render, spikerender). Local-only:
  corpus, ideals, renders, grades.
- Backlog #63 captures the flag-a-bad-render loop (one-key submit,
  double opt-in, hosted collection) as the long-term corpus strategy.
- Method finding worth keeping: blinded grading diagnosed failures to
  named rules, and one corrective round halved fails without
  regressions. The offline improve loop works end to end.

Phase 0 outcomes:
- Charter committed:
  `docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md`.
- This STATUS created; rebuild STATUS and spec-hardening STATUS carry
  superseded banners.
- CLAUDE.md and `.claude/rules/poplar-development.md` rewritten to
  route "continue" here and to mark all legacy docs
  (invariants, ADRs, subsystem rules) as non-binding reference.
- Workstation memory refreshed: re-founding initiative memory replaces
  the rebuild one; obsolete legacy-era entries deleted.
- Agents (`poplar-implementer`, `poplar-reviewer`,
  `poplar-go-reviewer`) and the in-repo `simplify` skill kept as-is;
  none is actively wrong, and Phase 5 redesigns the build machine
  against the settled architecture.

## Next: Phase 5 pass 1b (integration and hardening)

Pass 1 is at its gate. Pass 1b exists because pass 1's scope burst
and because closing it on undemonstrable claims would be dishonest:
its own stated outcome, a sync engine that converges against the
live server, is proven by tests and not by the program. Pass 1b
makes the foundation real, then hardens it. It discharges no new
MUST from the spine; it discharges the ones pass 1 claimed.

Its gate is a demonstration, not a checklist: poplar runs against
the live Fastmail account and Geoff watches the store fill.

**The plan is authored** (2026-07-28, a Fable planning sitting):
`docs/superpowers/plans/2026-07-28-pass-1b-integration-hardening.md`,
twelve tasks. Execution runs in a fresh Opus 5 session per the
plan-approval model boundary; the starter prompt below is the
handoff. The scope list that follows remains the binding source
the plan was authored from.

**Execution in progress (2026-07-30). Tasks 1 through 7b are done,
reviewed, and pushed** (master at `fd722bf`, clean). **Tasks 8, 9, 10,
11 and 12 remain.** The live progress ledger is
`.superpowers/sdd/2026-07-28-pass-1b-integration-hardening/progress.md`
and it is the recovery map: it records every task's outcome, every
parked and deferred finding, the routing rulings, and the defect
patterns worth carrying. **A resuming session reads that ledger
first**, then resumes at task 8.

Task 6a landed the cutover to `internal/backend/jmapsource` on
`poplar/jmap` with go-jmap and oauth2 out of `go.mod`. **Task 6b gave
the running program push**: live inbox latency against the Fastmail
account is 340ms end to end, from a server-side mailbox create to the
push-triggered `/changes` pull landing the row, where it was
`PollInterval`'s 60 seconds. Under an induced five-drop outage the
transport made six connections on its own jittered schedule and the
sync engine made one `Listen` call, so exactly one layer backs off.

**Task 7 was split by the same reasoning that split task 6.** 7a
replaced the seam's `map[string]any` field maps with typed models, a
sealed-interface payload rather than per-kind methods, since
`ObjectKind` also keys the sync-state table, the watermark, the echo
tracker, the push loop and resync. 7b closed the create-replay window
that duplicated a mailbox, by reconciling on the server's own
`alreadyExists` refusal: no schema change, no new outbox state, and no
marker distinguishing a replay from a first attempt, which is the
obstacle that made the fix impossible in pass 1.

**Two rulings from 7b bind later work.** The `landed` flag is gone, so
**task 8's disposition enum has three members, not four**, and its
brief carries the amendment plus the replacement acceptance criterion.
And `internal/store`'s `TestIncrementalVacuumReclaimsFreelist` and
`TestInteractivePreemption` fail under `-race` at master while passing
3/3 without it, so **CI's race job is red and has been since task 6a**,
independent of this pass; both are routed to task 11.

- **Task 1, the headless runner: complete.** `cmd/poplar` starts the
  JMAP session, sync worker, and dispatcher; clean shutdown ordering
  is pinned; poplar now starts and stays up with no network.
  `deadcode` fell from 203 unreachable functions to 61, each
  remaining one enumerated and justified. Five fix rounds.
- **Tasks 2, 3 and 4: the `jmap` library is complete** at
  `poplar/jmap`. It carries the data model, transport, and
  EventSource push client, is standard-library-only, imports no
  poplar package, and has a `make jmap-boundary` gate enforcing that
  mechanically. ADR-0018
  (push stream resumption) is committed. The adopt case held: the
  seven go-jmap defect fixes came to ~23 lines against 1,592 adopted,
  1.4% against the plan's 30% abort threshold, with no signature
  changes.
- **Task 5, Stalwart conformance: complete.** The suite runs behind
  its own build tag, DV-01 through DV-12 green against a freshly
  provisioned Stalwart v0.16.15, and `make conformance` sits outside
  `make check` so the default gate still needs no container runtime.
  Three fix rounds and five review passes. The first review found a
  Tier-1 item skipped on an impossibility claim that a probe
  disproved, the trim ledger never reaching the repo, and the first
  build-tagged files in `jmap/` sitting outside every gate.
- **Two product bugs surfaced by the second server, both fixed.**
  Stalwart advertises a push `interval` of 30000 while pinging every
  30 seconds, sending milliseconds where RFC 8620 section 7.3 says
  seconds; unclamped, poplar's stall window became 16h40m and a dead
  push connection would never be noticed. And poplar refused to move
  mail into any mailbox whose id is all digits, which Stalwart
  generates, because a numeric pointer token was read as an array
  index against an object. RFC 6901 section 4 reads a token as an
  index only when the referenced value is an array.

**Both owner-directed artifacts (2026-07-29) have landed:**
- **The trim ledger** is `jmap/TRIMMED.md`, referenced from
  `jmap/doc.go`: one row per method or package deliberately omitted
  from `poplar/jmap`, with why it is out and what would bring it
  back. The trim boundary leaked three times before it was tracked.
- **The RFC obligations map** is
  `docs/poplar/research/2026-07-28-jmap-rfc-obligations-map.md`,
  client-binding normative obligations across RFC 8620, 8621, 9219,
  6901 and the WHATWG SSE section, each mapped to a proving test, a
  `GAP`, or an `N/A`. **Ruling: it is a standing input, not a work
  item.** Each pass closes the MUST gaps for the surface that pass
  ships. Pass 2 takes the read path, pass 3 the eleven `Email/set`
  create constraints, and pass 5 the calendar rows. There is no
  gap-closing task and no gap-closing pass. It also carries the
  server-set-property survey that pass 3 needs when it designs
  compose, and the interop note that naming a capability a server
  lacks gets the whole request rejected rather than the one call.

**Task 6 is split (orchestrator ruling, 2026-07-29).** It had grown
from five deliverables to nine, and the four this pass routed to it
are push-semantics work that contradicts its own written boundary,
"no behavior changes to sync or outbox in this task." **6a is the
mechanical cutover** and keeps that boundary, since its whole safety
argument is that nothing changed and the existing adapter suite plus
a live run is a sound net for exactly that claim. **6b is the
push-semantics change**: the reconnect-ownership collapse,
flush-on-connect, JT-21's trigger half, the stall-detector disable,
and the ping-clamp log seam. Briefs for both are written in the sdd
directory. JT-13/14/15's fixture assertions route to task 11, whose
`/changes` paging consumer in `internal/sync` is what they protect.

**Environment correction for anyone running the conformance suite:**
Docker is not installed on this workstation. The suite runs on
**podman** (rootless, no daemon, no sudo), and the Makefile prefers
podman with a `CONTAINER` override. The Stalwart image was renamed
upstream: `stalwartlabs/mail-server` stops at v0.11.8, and v0.16.15
lives at `docker.io/stalwartlabs/stalwart:v0.16.15`.

Scope, in dependency order:

1. **The headless runner.** `cmd/poplar` constructs the JMAP source
   from the token, starts the sync worker and the outbox dispatcher,
   and runs until interrupted. No UI; that is pass 2. This is what
   makes `deadcode` a meaningful signal, and `deadcode` is promoted
   from the nightly CI job into the pass-end ritual with it. Carry
   the engine hazard the dispatcher review recorded: the orphan
   sweep is startup-only, so an engine loop that recovers a panic
   and keeps running must re-sweep or exit.
2. **The `jmap` package**, written correctly rather than adopted
   mechanically. go-jmap's RFC-verified types and MIT tests are the
   starting material and the inventory; poplar writes the transport
   itself, because the library's `Listen()` takes no context, never
   reconnects, ignores the SSE `Last-Event-ID` resume header, caps
   server lines at 64KB, and drops `Close()`'s error. Package
   `jmap` at `poplar/jmap`, non-internal so another project can use
   it, its own repo when it earns one. poplar's adapter renames to
   `internal/backend/jmapsource`. The import-boundary analyzer gains
   a carve-out: only that adapter may import `jmap`. MIT attribution
   (Max Mazurov 2019, Tim Culverhouse 2022) survives the rewrite.
   Acceptance criteria are the 46 numbered items in
   `docs/poplar/research/2026-07-28-jmap-test-inventory.md`, ordered
   by data-loss risk. `Last-Event-ID` resumption needs its own ADR:
   RFC 8620 section 7.3 states it in prose with no worked example,
   Apache James does not implement it, and go-jmap never reads an
   SSE `id:` field, so poplar sets the policy unilaterally.
   Abort condition: if any of the seven known defect fixes changes a
   signature that ripples into poplar's callers, stop and
   re-present; the adopt case rests on the defects being local.
3. **Second-server validation, before the library ships.** Stalwart
   v0.16.15 in Docker, session at `/.well-known/jmap`, behind a
   `conformance` build tag beside the existing `live` suite. It is
   the right control because Fastmail's `sessionState` shows a
   Cyrus-derived backend, so Cyrus would have been testing one
   lineage against itself. The 12 divergence tests are in the same
   document.
4. **Typed models at the backend seam**, replacing `Record.Fields
   map[string]any`. Types live in `internal/backend`; `internal/sync`
   translates them into the store's upsert structs. Adoption comes
   first so the wire/domain boundary is placed against poplar's own
   code.
5. **The outbox dispatcher design review**, already done and
   specified in
   `.superpowers/sdd/2026-07-27-pass-1-foundation/` and the saved
   review: replace `failed`/`final`/`landed` with one `disposition`
   enum (delivered, retry, terminal, landed) end to end, which
   closes the currently unenforced `landed implies final` invariant
   by construction; delete the best-effort fallback and have the
   recovery's requeue write only the non-growing columns, dropping
   `failure_detail`; keep the detached-context recovery, which is
   essential. Six variants of a stranded intent came out of this one
   path, and the last three came from fixing the previous ones.
6. **The perf seeder**, given realistic body weight, with QA-2 and
   QA-3 re-measured. QA-5's storage-overhead bound cannot be
   exercised by bodies of a few hundred bytes either.
7. **The SQLite driver decision.** The audit ranks
   `ncruces/go-sqlite3` above the incumbent `modernc.org/sqlite`,
   whose verification apparatus was dismantled rather than merely
   quiet: `TestTclTest` deleted, `internal/mptest` gone, no CI
   configuration, last published Tcl pass rate June 2023 against
   SQLite 3.42, and no `fts5*.test` file ever vendored. ADR-0001's
   disqualifier for ncruces (a Windows WAL corruption bug) was fixed
   2026-07-06 and never affected Unix. The decision needs the
   benchmark the audit designed, run on a quiet machine: whole-
   process peak RSS against QA-5's 250MB with one writer and four
   readers at realistic body sizes, since latency has an order of
   magnitude of headroom on both. Then the QA-6 kill harness under
   both.
8. **The seven Major findings** from the pass-end fan-out that were
   not fixed in pass 1, including the EXPLAIN goldens that pin
   queries nobody runs and `fullResync` holding one unbounded
   transaction against ADR-0003's 50ms cap.

Remaining inherited dependencies still to re-derive, per the
owner's rule that nothing the archived client picked is assumed
correct: the MIME stack (`go-message` plus `enmime`, and why both),
the render stack (`goldmark`, `chroma`, `glamour`, which matters
most since the rendering bet is the differentiator), `go-webdav`
before pass 5 leans on it, and `go-keyring`, which pass 1 does not
yet use. bubbletea is settled by owner intent (the vision names a
bubbletea showcase as a goal) rather than inherited by default.

Starter prompt (paste after /clear in ~/Projects/poplar, in an
Opus 5 session):

```
Resume executing the poplar pass 1b plan at
docs/superpowers/plans/2026-07-28-pass-1b-integration-hardening.md
via superpowers:subagent-driven-development. Tasks 1 through 7b are
complete, reviewed and pushed (master fd722bf, clean). Resume at task 8.

Read .superpowers/sdd/2026-07-28-pass-1b-integration-hardening/progress.md
FIRST, starting with the "READ THIS FIRST" block at the top. It is the
recovery map: every task outcome, every parked and deferred finding, the
routing rulings that bind later tasks, and the defect patterns this pass
has produced. Then read the plan, and the pass 1b scope in
docs/superpowers/specs/poplar-refounding-STATUS.md.

Task briefs live beside the ledger. Task 8's brief carries an ORCHESTRATOR
AMENDMENT from task 7b that changes its outcome and one of its acceptance
criteria: the disposition enum has three members, not four, because 7b
deleted the landed flag once closing the create-replay window left no
dispatch outcome unsafe to replay. Read that amendment before dispatching.

Order: 8 and 11 are independent and run first. Tasks 9 and 10 must run on
a quiet machine with NO implementer dispatched, 10 needs 9's corpus, and
12 closes the pass. Task 11 carries two conformance-suite items routed
from 7b (DV-11's false RFC citation, which pins a Stalwart-only existingId
that Fastmail omits, and a DV row for the sibling-uniqueness normalization
divergence) plus the red -race tests in internal/store.

Binding research is under docs/poplar/research/; requirements are at
revision 4 and ADR revision blocks override ADR bodies. The RFC
obligations map is a standing input, not a work item.

One poplar-implementer per task; poplar-reviewer and
poplar-go-reviewer in parallel on each diff; the main loop reviews
each diff and confirms `make check` between dispatches. Every
reviewer verification proves revert-sensitivity by experiment, not
by reading. Tasks 9 and 10's measurements run on a quiet machine
with no concurrent implementer dispatched.

Expect the fix round, not the clean first pass. Across tasks 1
through 5 every fix round but the last introduced the next defect,
usually in the guard written to close the previous finding, so tell
each reviewer to attack the round's new guards before anything
else. Comment defects outproduced logic defects in task 5: a
justification comment must be literally true clause by clause.

Dispatch hygiene, all learned the expensive way: tell every agent
never to set run_in_background on a Bash call, and to give every
scratch file a unique name. Verify any reported gate result the
local gate does not itself run, especially -race. Measuring lint
findings needs --max-same-issues=0, since the default truncates at
three and makes a load-bearing suppression look dead.

Geoff is at the pass gate only, and that gate is a demonstration:
poplar running against the live Fastmail account with the store
filling.
```

## Superseded: Phase 1, the rendering bet

**In progress (2026-07-19).** Plan:
`docs/superpowers/plans/2026-07-19-rendering-bet.md`. A fresh session
resuming mid-phase executes that plan from the first unchecked task
via superpowers:subagent-driven-development. Settled with Geoff:
standard depth (25 per class, 3 ideals per class), harvest from all
folders including Spam and Trash. Settled 2026-07-19 mid-phase:
poplar's runtime rendering must be a robust deterministic algorithm,
never an LLM in the render path. LLM use is confined to the offline
loop (rule derivation, corpus improvement, grading). The Task 5 LLM
arm is reframed as a comprehension-ceiling benchmark that prices the
deterministic gap, not as a runtime candidate; the verdict judges
viability on the deterministic arm alone. Also settled 2026-07-19:
the production rule system must be structured for extension,
modification, and explanation, so that people and LLMs can improve
the rules over time. Rules carry a name, an observable trigger and
transform, provenance (the corpus messages that motivated them), and
tests, and the renderer can report which rules fired on a given
message. Binds the Phase 4 rendering-pipeline design; the verdict
should assess rule structure, not only rule quality.

Starter prompt (paste after /clear, or say "run Phase 1 of the
re-founding"):

```
Run Phase 1 of the poplar re-founding: the rendering bet. Read the
charter docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md
first, especially the Phase 1 section and the principles.

The phase answers whether messy modern HTML mail can be turned into
prose a user reads and answers in a terminal, and where the
comprehension intelligence must live (deterministic Go vs. an LLM in
the loop, offline vs. runtime). The hard core is comprehension and
judgment, designated Fable-tier: Fable authors the ideal renders,
distills the readability principles, and judges outputs. Plumbing
(corpus harvest via JMAP from the Fastmail account, spike iteration,
bulk conversion runs) dispatches to cheap models.

Start by planning the phase with Geoff before building: corpus scope
and class taxonomy, sample sizes, privacy handling (the corpus stays
local and uncommitted), prior-art candidates for the spike
(HTML-to-markdown libraries, reader-mode extraction algorithms, lynx
and w3m baselines, the legacy renderer as salvage), and the grading
protocol. Then use superpowers:writing-plans and execute.

Deliverables, in order: the readability standard (corpus plus
hand-authored ideal renders plus a principles doc), the throwaway
spike, the per-class grading (excellent / usable / degraded / fail),
and the verdict doc under docs/poplar/research/. End at the gate:
Geoff reads the verdict and sample renders and rules on the bet.
```

## Phase roadmap

(Charter Phases section.) 0 Founding reset [done] -> 1 Rendering bet
[done] -> 2 Vision [done] -> 3 Requirements [done] ->
4 Technical design [done] -> 5 Build machine + build [in progress:
pass 1 foundation done, pass 1b integration and hardening next].
