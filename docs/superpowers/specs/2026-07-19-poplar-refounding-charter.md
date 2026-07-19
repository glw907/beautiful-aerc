# Poplar Re-founding Charter

Date 2026-07-19. Status: active. This charter supersedes the rebuild
charter (`2026-05-29-poplar-rebuild-charter.md`) and its spec track.
It holds the frame for the effort and does not change per phase. The
tracker `poplar-refounding-STATUS.md` (created in Phase 0) owns the
phase cursor and the starter prompts; read it first on "continue".

## Why a third track

Poplar has been attempted twice, and each attempt taught something
this charter encodes.

The dogfood client (archived at tag `poplar-legacy`, branch `legacy`)
was built without settled requirements or a settled architecture. It
taught the domain, produced salvageable subsystems, and churned
through enough rework to prove that plan-as-you-go does not scale to a
project this size.

The 2026-05-29 rebuild track answered with a spec-first machine and
produced a 132-scenario functional spec plus a gap analysis. It
stalled, and the diagnosis stands on four counts:

1. The spec is a feature catalogue, not requirements. It lacks
   priorities, non-goals, quality attributes, and a crisp v1 boundary.
2. No technical design exists. That charter deferred architecture to
   the build-plan phase, which repeats the failure it was meant to fix.
3. The build machine (agents, gates, orchestration defaults) was
   designed in Pass I, before the architecture it would build existed,
   so it optimized for the wrong work.
4. The central technical bet was assumed rather than tested. Spec §4
   built a contract-and-corpus edifice on the premise that modern HTML
   mail converts to readable terminal prose. The legacy renderer's
   real output was technically correct markdown that often read badly,
   which is evidence against assuming the premise.

This track fixes all four: vision and requirements with real
priorities, a settled technical design before code, a build machine
designed against that design, and the rendering bet validated first.

## Product frame

Provisional until the Phase 2 vision locks. These are the working
beliefs the early phases test and refine.

- v1 is the MVP daily driver: the smallest client Geoff switches to
  full-time. Everything else stages behind it.
- The architecture is judged against a knowable horizon. Phase 2
  writes an explicit register of post-MVP improvements; Phase 4's
  design must foreclose none of them.
- The rendering thesis: the primary issue for any modern TUI mail
  client is turning messy modern HTML into prose a user can read and
  answer. This is poplar's central bet and hardest problem. It is
  validated in Phase 1, before the vision locks, and the verdict
  shapes the product whichever way it lands.
- Audience: coders, vim-first, modifier-free single keys. Re-examined
  in Phase 2, not assumed away.

## Principles

1. Nothing inherited by default, anything salvageable by choice. Code,
   spec prose, research, agents, and skills from the prior efforts are
   all reusable where they earn their place. None is force-fit.
2. The rendering bet is the first deliverable. An unflattering verdict
   is acceptable; an untested assumption is not.
3. MVP daily-driver v1, with the post-MVP horizon written down so the
   architecture can be judged against it.
4. Design before code, machine before build. Requirements and
   technical design are settled, adversarially reviewed artifacts
   before the first build commit. The build machine is designed
   against the real architecture, not before it exists.
5. Every phase ends at a gate Geoff rules on, with a committed
   artifact. Phases run in fresh sessions; the artifacts are the
   handoff.

## Prior artifacts

| Artifact | Standing under this charter |
| --- | --- |
| Legacy client code (`poplar-legacy`, branch `legacy`) | Salvage source for spikes and the build |
| Rebuild functional spec (2026-05-29, §1–§8) | Research input for Phases 2–4 |
| Spec-hardening Pass A gap analysis | Research input for Phases 2–3 |
| Mail-client field survey, infra-refresh research | Research inputs |
| Spec-hardening Passes B and C | Cancelled; their critique energy spends inside each phase's adversarial review |
| In-tree Claude infra (CLAUDE.md, rules, agents) | Pruned in Phase 0, redesigned in Phase 5 |
| Working code tree on `master` | Stays intact until the build boundary; spikes borrow from it freely |

## Phases

Each phase runs in fresh sessions, ends with a committed artifact, and
closes at a gate where Geoff rules. The STATUS tracker carries each
phase's starter prompt.

### Phase 0: founding reset

Small and quick. It exists so no later session inherits stale context.

- Commit this charter and create `poplar-refounding-STATUS.md`. Mark
  the rebuild STATUS and the spec-hardening STATUS superseded, each
  pointing forward.
- Rewrite the routing layer: the CLAUDE.md banner and
  `.claude/rules/poplar-development.md` route "continue" to the new
  track. Strip CLAUDE.md's legacy-client detail (the invariants
  include describes archived code) down to what still serves: build
  commands, conventions skills, voice rules.
- Refresh workstation memory: the rebuild-initiative memory updates to
  the re-founding; stale entries retire.
- Prune agents and skills only where actively wrong. The full build
  machine is Phase 5's job.

Gate: Geoff confirms the reset; "continue" in a fresh session lands in
the new track cleanly.

### Phase 1: the rendering bet

The make-or-break question, answered honestly. The hard core of this
phase is comprehension, deciding what a messy HTML message actually
says and re-expressing it as prose a human wants to read and answer.
That is frontier-model work: Fable authors the ideal renders, distills
the principles, and judges the outputs. Cheap models run the plumbing
and the iteration loops. Four deliverables, in order:

1. The readability standard. Harvest a corpus of real mail from the
   Fastmail account via JMAP, sorted into classes: GitHub and CI
   notifications, newsletters, transactional receipts, marketing,
   personal, calendar invites, mailing-list and patch mail. For a
   representative sample of each class, write a hand-authored ideal
   render: what a skilled human would produce if transcribing that
   email as clean markdown for a terminal reader. The exemplars plus a
   short principles doc distilled from them define excellent. The
   prior effort graded markdown validity while the product needed
   readability; this standard exists so that mistake cannot recur.
   The corpus stays local and uncommitted; a scrub-and-commit gate may
   follow later.
2. The spike. A throwaway converter, prior art first: existing
   HTML-to-markdown libraries, browser reader-mode extraction
   algorithms, lynx and w3m dumps as baselines, and the legacy
   renderer as salvage. Iterated against the exemplars.
3. The grading. Per-class scoring against the ideal renders, with
   fresh-context judges for scale and Geoff's eyes on samples for
   truth. Four grades: excellent, usable, degraded, fail.
4. The verdict doc. Per-class feasibility; which techniques carried
   the weight; where the intelligence lives, meaning how much of the
   comprehension compiles into deterministic Go via offline
   rule-derivation and how much requires an LLM in the loop, with the
   cost, latency, and offline implications of each; the fallback story
   for the fail classes (filtered plain text, open in browser); and
   the implications for the vision.

Gate: Geoff reads the verdict and sample renders and rules on the bet.
A partial verdict, such as works for coder-relevant mail and degrades
on marketing soup, is a fine outcome that sharpens the product.

### Phase 2: vision

Interactive, Geoff's taste in the driver's seat, the rendering verdict
on the table. Output is a short, opinionated vision doc: what the MVP
daily driver must do for Geoff to switch full-time; the
differentiators, with the rendering verdict as evidence; the audience;
explicit non-goals; and the knowable-horizon register, the post-MVP
improvements the architecture must not foreclose, each named so Phase
4 can be judged against it.

Gate: Geoff approves the vision doc.

### Phase 3: requirements

The vision made testable. MVP scope with real priorities (must,
should, later), quality attributes with numbers where they matter
(startup time, sync latency, offline behavior, large-mailbox
performance), and acceptance criteria per requirement. Lean: the old
spec's 132 scenarios are mined as reference, never inherited as scope.
Adversarially reviewed before the gate.

Gate: Geoff approves the requirements spec.

### Phase 4: technical design

Architecture settled and reviewed before the first build commit: data
model and cache schema, package boundaries and the backend seam, sync
engine, concurrency model, the rendering pipeline as a first-class
subsystem per the Phase 1 verdict, TUI architecture, error and logging
model, and testing strategy. Each major decision gets an ADR with
alternatives considered. Remaining technical risks are spiked here.
The design is judged against the knowable-horizon register: it must
foreclose nothing on the list, or say why the foreclosure is accepted.
Adversarially reviewed before the gate.

Gate: Geoff approves the design.

### Phase 5: build machine, then build

With the design locked, the execution infrastructure is built against
it: implementer and reviewer agents tuned to the actual architecture
and conventions, the quality gate (the `make check` equivalent)
defined from day one, the plan format for TDD build passes, and the
verification harness (tmux-driven UI checks, golden render tests
inherited from Phase 1's corpus). Then the numbered build passes
begin, subagent-driven, with Geoff's involvement at pass gates.

Gate: Geoff approves the machine design; thereafter, per-pass gates.

## Operating model

- Fable conducts every planning phase; this is the judgment work the
  seat exists for. Phase 1's comprehension core is explicitly
  Fable-tier. Volume work (corpus harvesting, spike iteration,
  research fan-outs) dispatches to cheap models per the workstation
  model economy.
- Geoff's attended time concentrates at the gates: taste calls,
  verdict rulings, phase approvals.
- One session per phase where feasible. The charter and STATUS are the
  handoff; anything load-bearing lives in a committed artifact, never
  only in a conversation.
- Artifacts: charter, vision, requirements, and design live under
  `docs/superpowers/specs/`; corpus notes, spike reports, and the
  verdict live under `docs/poplar/research/`.

## Next action

Run Phase 0. Starter prompt (paste after /clear, or say "run Phase 0
of the re-founding"):

```
Run Phase 0 of the poplar re-founding. Read
docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md first.
Execute the Phase 0 checklist: create poplar-refounding-STATUS.md with
the phase cursor and the Phase 1 starter prompt, mark the two old
STATUS docs superseded, rewrite the CLAUDE.md banner and
.claude/rules/poplar-development.md to route "continue" here, strip
stale legacy detail from CLAUDE.md, refresh the rebuild-initiative
memory, and prune actively-wrong agents and skills. End at the gate:
show Geoff what changed and confirm a fresh "continue" lands cleanly.
```
