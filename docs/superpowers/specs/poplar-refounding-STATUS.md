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

## Current state (2026-07-19)

Phase 1 (rendering bet) executed; the gate ruling is Geoff's and is
the only open item.

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

## Next: Phase 2, vision

Blocked on the Phase 1 gate: Geoff reads the verdict doc and sample
renders and rules on the bet. On a proceed ruling, start Phase 2.

Starter prompt (paste after /clear, or say "run Phase 2 of the
re-founding"):

```
Run Phase 2 of the poplar re-founding: vision. Read the charter
docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md
(Phase 2 section) and the rendering-bet verdict
docs/poplar/research/2026-07-19-rendering-bet-verdict.md first.

This phase is interactive: Geoff's taste drives, the verdict is on
the table. Output is a short, opinionated vision doc: what the MVP
daily driver must do for Geoff to switch full-time; the
differentiators, with the rendering verdict as evidence; the
audience; explicit non-goals; and the knowable-horizon register of
post-MVP improvements the architecture must not foreclose (seed it
from backlog #63's flag loop, the capture-mailbox corpus strategy,
ICS-first calendar rendering, the fact-inventory self-check, and the
nvim companion). Constraints already settled and binding: no LLM in
the render path; rules declarative, explainable, and traceable.

End at the gate: Geoff approves the vision doc.
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
[done, gate ruling pending] -> 2 Vision [next] -> 3 Requirements ->
4 Technical design -> 5 Build machine + build.
