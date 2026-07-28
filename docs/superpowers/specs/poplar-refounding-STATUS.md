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
2. Replace the `## Next` section with the next phase's starter prompt.
3. Mark the phase `[done]` and the next `[next]` on the roadmap line.
4. Commit the artifacts and this STATUS, then push.
5. Refresh the `project_poplar_refounding_initiative` memory only when
   a load-bearing decision changed. The cursor lives here, not in
   memory.

## Current state (2026-07-27)

Phase 3 (requirements) closed with Geoff's approval the same day it
ran. Phase 4 (technical design) is next and opens with the
exhaustive prior-art and library survey Geoff directed.

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

## Next: Phase 4, technical design

Starter prompt (paste after /clear, or say "run Phase 4 of the
re-founding"):

```
Run Phase 4 of the poplar re-founding: technical design. Read the
charter docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md
(Phase 4 section), the approved requirements
docs/superpowers/specs/2026-07-27-poplar-requirements.md (its
constraints bind every decision), and the grounding survey
docs/poplar/research/2026-07-27-phase3-grounding-survey.md.

Open with the exhaustive prior-art and library survey Geoff directed
(spec C9): for every subsystem (store engine, JMAP sync, CalDAV and
iCalendar, recurrence, MIME, HTML-to-text, markdown rendering, the
bubbletea/Charm ecosystem, full-text search, keyring, notifications,
clipboard), inventory existing libraries, tools, and prior art at
their current releases, verified live; poplar builds nothing a
maintained dependency already does well, and no version inherits a
legacy pin. Survey dispatches go to cheap models.

Then settle the design: data model and store schema (account-keyed
per C4), package boundaries and the backend seam, sync engine and
concurrency model, the rendering pipeline as a first-class subsystem
per the Phase 1 verdict, the calendar subsystem on CalDAV with the
JMAP-calendars upgrade seam, TUI architecture, the unified design
language artifact (UX-3, cairn-cms as the exemplar), the error and
logging model, and the testing strategy. An ADR per major decision
with alternatives considered. Run the spec section 16 probes: CalDAV
RSVP behavior at Fastmail (ask Geoff for a calendar-scoped token),
JMAP draft and Identity semantics, the CO-5 HTML allowlist, and the
measurement spike that replaces the provisional QA numbers (a
blocking input to Phase 5 planning). Judge the design against the
knowable-horizon register and C11's forward-looking lean stance: it
must foreclose nothing on the list and should land where mail,
calendar, and contacts are heading. Adversarially review before the
gate.

End at the gate: Geoff approves the design.
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
4 Technical design [next] -> 5 Build machine + build.
