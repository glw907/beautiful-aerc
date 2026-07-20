# Rendering bet verdict

**Date:** 2026-07-19
**Phase:** Re-founding Phase 1. Charter:
`docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md`.
**Question:** Can messy modern HTML mail be turned into prose a user
reads and answers in a terminal, and where must the comprehension
intelligence live?
**Verdict in one line:** Yes for the mail poplar's audience lives in,
with a measured boundary: a deterministic rule pipeline reaches 87%
usable-or-better on the coder-relevant core of a real mailbox, the
weak classes fail in diagnosable and improvable ways, and the
intelligence compiles offline into Go rules with no LLM at render
time.

The corpus, ideal renders, per-render grades, and spike renders are
local artifacts under `corpus/` (private mail, never committed). The
committed evidence chain is this document, the readability standard
(`2026-07-19-rendering-readability-principles.md`), and the spike code
(`cmd/renderspike`, `cmd/llm-render`, `internal/spikerender`,
`cmd/corpusharvest`).

## Method

169 real messages were harvested from the Fastmail account across
seven classes (25 per class; marketing reached 19, an honest
shortfall). For 21 of them (3 per class), hand-authored ideal renders
defined excellent; a principles doc distilled from those ideals
defines the four grades (excellent, usable, degraded, fail) in
observable output terms. Five arms rendered every message:

- `lynx` and `w3m` dumps, the terminal status quo.
- `legacy`, the archived dogfood client's pipeline
  (html-to-markdown/v2 plus normalization).
- `iterated`, the product candidate: the legacy pipeline plus targeted
  readability extraction plus 29 named, provenance-tracked rules
  derived across four steering rounds and one grading-driven
  corrective round.
- `llm`, a comprehension-ceiling benchmark: Haiku rendering from the
  principles doc. Benchmark only; a settled product constraint
  excludes any LLM from the render path.

Grading was blinded: one fresh judge per message received the grade
definitions, a same-class calibration ideal (never the judged
message's own), the original body, and all five renders shuffled under
letter names, with arm identities withheld. 840 grades resulted. A
blind audit re-graded a 20-pair stratified sample: 60% exact
agreement, 95% within one grade, no directional bias. After the
corrective rule round, the 63 degraded-or-fail iterated renders were
re-graded single-candidate by fresh judges.

## Results

Usable-or-better across the full corpus (168 graded; one
attachment-only DMARC report errors in every arm, correctly):

| Arm | Excellent | Usable | Degraded | Fail | Usable+ |
|---|---|---|---|---|---|
| iterated (post-correction) | 32 | 82 | 38 | 16 | 68% |
| llm (ceiling benchmark) | 96 | 50 | 5 | 17 | 87% |
| legacy | 7 | 63 | 88 | 10 | 42% |
| w3m | 5 | 8 | 53 | 102 | 8% |
| lynx | 2 | 6 | 57 | 103 | 5% |

The iterated arm per class, post-correction (usable-or-better, with
pre-correction in parentheses):

| Class | Usable+ | Fails | Movement |
|---|---|---|---|
| github-ci | 100% (92%) | 0 | Both degraded cleared |
| personal | 88% (79%) | 0 | Last fail cleared |
| list-patch | 84% (76%) | 2 | |
| transactional | 76% (76%) | 1 | Fail count 2 to 1 |
| newsletter | 48% (44%) | 2 | Fail count 4 to 2 |
| calendar | 44% (36%) | 5 | Fail count 13 to 5 |
| marketing | 26% (26%) | 6 | Judged borderline both rounds |

The coder-relevant core (github-ci, personal, transactional,
list-patch: 99 messages) sits at **87% usable-or-better with 3
fails**. The commercial-and-calendar tail (69 messages) sits at 41%.

The corrective round is itself a result. Blinded grading diagnosed
30 fails to two named rules plus one noise source; four precision
guards (R26 to R28 plus one pattern extension) written against the
judges' reasons halved the fails in one round, cleared every calendar
truncation, and restored every dropped CTA that the guards targeted,
without regressing previously excellent renders. Failures in, rules
out, measurably: that is the improvement loop the product bets on,
demonstrated end to end inside the phase.

## Where the intelligence lives

Settled by decision during the phase and confirmed by measurement:
at render time, nowhere. The renderer is deterministic Go.

- The rules were authored by frontier-model comprehension working
  offline: reading failures, naming the transformation, motivating it
  with specific messages. Every one compiled into ordinary Go with
  tests. Nothing about the surviving gap suggests a different compiler
  of comprehension into rules; it suggests more corpus.
- The LLM benchmark prices what render-time comprehension would buy:
  19 points of usable-or-better overall, concentrated in marketing
  and newsletter. It would cost a median 25.6 seconds per message
  (p90 47s, measured over 82 fresh calls; 86 resumed calls lost
  timing), token spend on every open, and an online dependency in an
  offline-capable client. Disqualifying on latency alone.
- The offline loop is where LLM effort keeps paying: rule derivation
  from flagged failures, corpus grading, and regression judging. The
  phase ran that loop five times at small scale.

## What carried the weight

In rough order of contribution: the html-to-markdown base conversion
(the legacy salvage); structure inference and noise-shedding rules
derived from exemplars; precision guards derived from graded failures;
targeted readability extraction (a scalpel that amputates forwarded
mail when applied blindly); and duplicate-link collapse with
subsumption checks. The single strongest lesson inverts the
rule-writing instinct: every serious failure across 840 grades was a
lost actionable fact, never surviving clutter. Shedding noise is easy;
the discipline that matters is proving facts survive.

Two structural lessons bind the Phase 4 design:

1. **Rules as declarative, explainable units.** The spike's rules are
   imperative functions, but their names, provenance, and observable
   trigger-and-transform statements are what made blinded failure
   diagnosis nearly free. The product rule engine should make that
   structural: each rule carries name, trigger, transform, motivating
   corpus references, and tests, and the renderer reports which rules
   fired per message. That trace plus a flagged message is a
   self-contained repair prompt for any human or LLM improving the
   rules.
2. **A fact-inventory self-check.** Deriving the set of actionable
   facts from the source (links, amounts, dates, codes) and verifying
   the output covers them is deterministic and cheap, and would have
   caught every serious failure in this corpus before a user saw it.

## The weak classes and the fallback story

**Calendar (44%)** is a false weak class. The spike rendered invites
from their HTML part; real invites carry a `text/calendar` part, and
the product should render the event from that structured data (title,
time, location, RSVP state) and skip the HTML entirely. The remaining
calendar fails are template-scraping artifacts of an approach the
product should not use. Expect this class to lead, not trail.

**Marketing (26%) and newsletter (48%)** are the honest boundary. The
residual failures are offer-structure inference in image-heavy mail:
the deal, deadline, and threshold live in preheaders, alt text, and
image pixels, and assembling them into a headline-first render is the
most comprehension-shaped work in the corpus. The ceiling benchmark
confirms room (87% there overall). Some of the gap will yield to rules
with corpus breadth; some may not. The fallback stack for a degraded
render: the reader's existing source-cycling (filtered plain text, raw
HTML part) and open-in-browser. A mail client's marketing mail is also
the mail a reader most tolerates skimming in degraded form.

**A partial verdict was pre-declared acceptable** by the charter, and
this is one: strong on coder-relevant mail, degraded on commercial
soup, with the boundary now measured instead of suspected.

## Limitations

- Single-mailbox distribution, and the owner doubts it generalizes.
  The list-patch class here is community discussion lists, not patch
  flow; hostile marketing is undersampled. Numbers are valid for the
  daily-driver decision and indicative beyond it.
- The post-correction re-grade was single-candidate rather than
  five-way comparative; 5 of 63 messages slipped a grade, consistent
  with judge variance at the usable/degraded boundary. Treat
  per-class movements under ~5 points as noise.
- Judges and standard-authors share a model family; the audit bounds
  but does not eliminate correlated taste.
- Rule overfit to this corpus remains the standing risk; the phase
  demonstrated the corrective loop, not immunity.

## Future work already scoped

- Public corpus: no modern public HTML-email corpus exists (survey:
  `2026-07-19-public-email-corpus-survey.md`). lore.kernel.org solves
  list-patch; a dedicated capture mailbox is the realistic path for
  the rest and would de-bias the single-mailbox limitation.
- User flagging (backlog #63): one-key flag-a-bad-render with a local
  problem corpus and an opt-in hosted collection, feeding the same
  offline loop this phase ran.
- Golden regression tests for Phase 5 from license-clean specimens.

## Gate packet

Sample renders for the ruling, all local paths; compare each render
(`corpus/renders/iterated/...`) against its source
(`corpus/bodies/...`) and, where present, its ideal
(`corpus/ideals/...`):

- Strong: `github-ci/StnkuHiv_UDN`, `transactional/StnnoWc8HTyc`,
  `newsletter/StnoH1-dPdyV` (degraded to excellent through the
  corrective round), `personal/StnqEkH-ZBVV` (fail to usable).
- The honest boundary: `marketing/StnmGAwtpO7N` (link soup, still
  fail), `newsletter/StnnJJAIVw4g` (lead story lost, still fail),
  `calendar/Stss8oJBvzTZ` (location lost; the class the ICS part
  makes moot).
- The ceiling: the same ids under `corpus/renders/llm/`.

## Phase economics

Roughly 17.5M subagent tokens across four workflows (readability
standard 0.6M, spike arms 1.4M, blinded grading 11.0M, corrective
re-grade 3.8M) plus implementer, reviewer, and research dispatches,
with main-loop spend on top (see `/cost`). Geoff interaction points:
two batched planning answers, one workflow authorization, one secrets
re-anchor approval, one infrastructure defect he had to report (the
1Password prompt loop, now hook-blocked), and voluntary
product-direction notes. The defect is the one that counts against
the process.

## Recommendation

Proceed to Phase 2 with the bet ruled viable inside the measured
boundary. Carry into the vision: rendering as poplar's differentiator
on coder-relevant mail; the rule engine, offline loop, and fact-check
as first-class Phase 4 subsystems; ICS-first calendar rendering;
honest degraded-mode fallbacks for commercial soup; and the capture
mailbox plus flag loop as the corpus strategy.

Gate: Geoff reads this document and the sample renders and rules on
the bet.
