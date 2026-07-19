# Rendering Bet Implementation Plan (Re-founding Phase 1)

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development to implement this plan
> task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Answer the re-founding's central bet: can messy modern HTML
mail be turned into prose a user reads and answers in a terminal, and
where must the comprehension intelligence live (deterministic Go vs.
an LLM, offline vs. runtime)?

**Architecture:** A local-only corpus harvested from the Fastmail
account feeds a readability standard (hand-authored ideal renders plus
a principles doc), a five-arm throwaway spike (lynx, w3m, the legacy
pipeline, an iterated deterministic arm, an LLM-in-the-loop arm), a
four-grade judging pass, and a verdict doc. The charter's Phase 1
section (`docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md`)
is the spec.

**Tech Stack:** Go (root module, reusing `internal/mailjmap` and
`internal/filter`), `github.com/go-shiori/go-readability`, lynx and
w3m as system binaries, `claude -p` headless for the LLM arm and the
judges.

## Global Constraints

- Privacy: everything under `corpus/` (raw mail, ideal renders, spike
  renders, grades) is local-only and never committed. Task 1 adds
  `corpus/` to `.gitignore`; only spike code, the principles doc, and
  the verdict doc land on `master`.
- All Go passes `make check` (fmt, vet, voice, modern-go, skipcheck,
  vale-comments, test). The `go-conventions` skill is mandatory before
  writing any Go; spike code is throwaway in purpose but not in
  quality.
- Classes (7): `github-ci`, `newsletter`, `transactional`,
  `marketing`, `personal`, `calendar`, `list-patch`.
- Targets: 25 messages per class harvested (accept >=15 with the
  shortfall recorded in the manifest), 3 ideal renders per class.
- Harvest source: all Fastmail folders including Spam and Trash,
  roughly the last 12 months, widening the window when a class runs
  short. Cap 5 messages per sender within a class.
- Executors: "Fable" tasks run in the main loop (comprehension and
  judgment). "Implementer" tasks dispatch to `poplar-implementer`
  (Sonnet). Bulk judging uses headless Sonnet; no Workflow runs
  without Geoff's opt-in.
- Arms (5): `lynx`, `w3m`, `legacy`, `iterated`, `llm`.
- Work lands directly on `master` per project convention.

---

### Task 1: Corpus harvest tool and harvest run

**Executor:** Implementer (Sonnet), then a short orchestrator
classification assist for the ambiguous remainder.

**Files:**
- Create: `cmd/corpusharvest/main.go` (plus any helpers in the same
  package)
- Modify: `.gitignore` (add `corpus/`)
- Output (local): `corpus/raw/<class>/<id>.eml`, `corpus/manifest.json`

**Interfaces:**
- Consumes: `internal/mailjmap` (`New`, `Connect`, `FetchHeaders`,
  `FetchBody`), the existing poplar account config and
  `FASTMAIL_API_TOKEN`.
- Produces: `corpus/manifest.json`, an array of entries with fields
  `id`, `class`, `from`, `subject`, `date`, `folder`, `path`; classes
  from the global list plus `unclassified` for the remainder.

**Requirements:**
- Walk every mailbox (Inbox, Archive, Sent excluded, Spam, Trash
  included), fetch headers for the window, classify deterministically
  from headers: GitHub/CI senders and `List-Id` -> `github-ci`;
  calendar MIME parts or invite headers -> `calendar`; non-GitHub
  `List-Id` or `[PATCH]` subjects -> `list-patch`; receipts, orders,
  confirmations from no-reply transactional senders ->
  `transactional`; `List-Unsubscribe` editorial senders ->
  `newsletter`; `List-Unsubscribe` promotional senders -> `marketing`;
  human senders with none of the above -> `personal`; anything unclear
  -> `unclassified`.
- Select up to 25 per class (sender cap 5, prefer sender diversity),
  download each selected message's raw blob to
  `corpus/raw/<class>/<id>.eml`, write the manifest, print per-class
  counts and the shortfall report.
- No mutation of server state: read-only JMAP calls only.

**Steps:**
- [ ] Write the harvest tool test-first where logic is testable
      (classification rules over synthetic headers; selection caps).
      Live-fetch code is exercised by the run, not unit tests.
- [ ] Run `make check`; green.
- [ ] Run the harvest against the live account; verify per-class
      counts, spot-check three `.eml` files parse as MIME.
- [ ] Verify `git status` shows no `corpus/` files as untracked
      candidates (gitignore effective).
- [ ] Orchestrator: classify the `unclassified` bucket from manifest
      metadata; rerun selection if it changes class counts.
- [ ] Commit the tool and `.gitignore` change.

**Acceptance:** Manifest exists with >=15 messages in every class (or
a recorded shortfall after widening the window); corpus untracked;
`make check` green.

### Task 2: Ideal renders and the readability principles doc

**Executor:** Fable (main loop). This is the phase's hard core.

**Files:**
- Output (local): `corpus/ideals/<class>/<id>.md` (21 files, 3 per
  class)
- Create: `docs/poplar/research/2026-07-19-rendering-readability-principles.md`

**Interfaces:**
- Consumes: `corpus/manifest.json`, `corpus/raw/`.
- Produces: the exemplar set the spike iterates against and the
  principles doc the LLM arm and the judges are prompted with.

**Requirements:**
- Select 3 exemplars per class favoring structural diversity (not
  three variants of the same sender).
- For each, read the raw HTML and hand-author the ideal render: the
  markdown a skilled human would produce transcribing the email for a
  terminal reader. Readability over fidelity; drop what a reader
  would skip.
- Distill the principles doc from the exemplars: what excellent means
  per class, the recurring moves (structure inference, noise
  shedding, link and image policy, table handling, quote handling),
  and the four grade definitions (excellent / usable / degraded /
  fail) in observable terms. The doc must contain no quoted private
  mail content; abstract or synthesize every example.
- The four grades are defined here once and reused verbatim by Task 6.

**Steps:**
- [ ] Select and record the 21 exemplars in the manifest
      (`exemplar: true`).
- [ ] Author the 21 ideal renders.
- [ ] Distill and write the principles doc.
- [ ] Commit the principles doc (docs-only commit).

**Acceptance:** 21 ideal renders on disk; principles doc committed,
free of private content, containing the grade definitions.

### Task 3: Spike runner with baseline and legacy arms

**Executor:** Implementer (Sonnet).

**Files:**
- Create: `cmd/renderspike/main.go` (plus helpers in the same package)
- Output (local): `corpus/renders/<arm>/<class>/<id>.md`,
  `corpus/renders/stats.json`

**Interfaces:**
- Consumes: `corpus/manifest.json`, `corpus/raw/`, `internal/filter`
  (the legacy HTML-to-markdown pipeline entry point, currently
  `filter.CleanHTML`), lynx and w3m binaries.
- Produces: the render tree layout and `stats.json` (per message per
  arm: wall milliseconds, output bytes, error string if the arm
  failed) that Tasks 4, 5, and 6 extend and read.

**Requirements:**
- For each manifest entry: parse the `.eml`, pick the `text/html`
  part (fall back to `text/plain`; record which), run each arm, write
  the render.
- Arms: `lynx` (`lynx -dump -nolist` over the HTML file), `w3m`
  (`w3m -dump -T text/html`), `legacy` (the `internal/filter`
  pipeline output).
- A failing arm records its error in `stats.json` and moves on; the
  run never aborts on one message.
- Verify lynx and w3m are installed before the run; install via apt
  if missing.

**Steps:**
- [ ] Write the runner test-first where testable (MIME part
      selection, render-tree pathing) using
      `internal/filter/testdata/` fixtures.
- [ ] Run `make check`; green.
- [ ] Run all three arms over the full corpus; spot-check one render
      per class per arm opens as plausible text.
- [ ] Commit the tool.

**Acceptance:** Renders exist for every corpus message in all three
arms (or a recorded per-message error); stats.json populated;
`make check` green.

### Task 4: Iterated deterministic arm

**Executor:** Implementer (Sonnet) iterating; Fable reviews each
round against the ideals and steers.

**Files:**
- Create: `internal/spikerender/` (the iterated pipeline: readability
  extraction plus the legacy pipeline plus new rules), wired into
  `cmd/renderspike` as arm `iterated`
- Modify: `go.mod` (add `github.com/go-shiori/go-readability`)
- Output (local): `corpus/renders/iterated/`, iteration notes in
  `corpus/renders/iterated-notes.md`

**Interfaces:**
- Consumes: the Task 3 runner and render tree; the Task 2 ideals as
  the target.
- Produces: arm `iterated` renders and stats, same layout as Task 3.

**Requirements:**
- Round structure: run the arm over the 21 exemplar messages, diff
  against the ideals, propose and apply rule changes (readability
  preprocessing, table flattening, noise shedding, quote handling),
  rerun. Fable reviews each round's diffs and steers or stops.
- Stop after the round where marginal improvement flattens, three
  rounds maximum; then run the final ruleset over the full corpus.
- The arm stays deterministic: no network, no LLM calls at render
  time. Every rule earns its place against the exemplars, recorded in
  the iteration notes with the exemplar that motivated it.

**Steps:**
- [ ] Wire go-readability preprocessing and the `iterated` arm
      skeleton; `make check` green.
- [ ] Round 1: exemplar run, diff review (Fable), rule changes.
- [ ] Round 2 and optionally 3: same loop, or stop early on Fable's
      call.
- [ ] Full-corpus run of the final ruleset.
- [ ] Commit the spike package and go.mod change.

**Acceptance:** `iterated` renders for the full corpus; iteration
notes name each rule and its motivating exemplar; `make check` green.

### Task 5: LLM-in-the-loop arm

**Executor:** Implementer (Sonnet) builds the driver; the renders run
on Haiku.

**Files:**
- Create: `scripts/llm-render.sh` (or a `renderspike` subcommand;
  implementer's call, smallest footprint wins)
- Output (local): `corpus/renders/llm/`, latency and token counts
  merged into `corpus/renders/stats.json`, the prompt at
  `corpus/llm-prompt.md`

**Interfaces:**
- Consumes: the principles doc (committed in Task 2), `corpus/raw/`
  HTML parts, `claude -p` headless with the Haiku model.
- Produces: arm `llm` renders and stats in the Task 3 layout, plus
  per-message wall latency and output token counts (the verdict's
  cost and latency evidence).

**Requirements:**
- Prompt: the principles doc plus the message HTML, instructing a
  markdown render to the standard; the exemplar ideals are not shown
  (that would leak the answer key for exemplar messages).
- One call per message on Haiku over the full corpus; sequential or
  lightly parallel, recording wall latency per call and token usage.
- Failures (refusals, timeouts) record as arm errors, same as Task 3.

**Steps:**
- [ ] Build and smoke-test the driver on 3 messages.
- [ ] Full-corpus run; verify render count and stats fields.
- [ ] Commit the driver (`make check` green if Go; shellcheck-clean
      if shell).

**Acceptance:** `llm` renders for the full corpus with latency and
token stats; prompt file preserved locally.

### Task 6: Grading run and judge audit

**Executor:** Implementer (Sonnet) builds the judge driver; judges
run on headless Sonnet; Fable audits.

**Files:**
- Create: `scripts/grade-renders.sh` (or a `renderspike` subcommand)
- Output (local): `corpus/grades.json`,
  `corpus/grades-summary.md` (the per-class per-arm grade matrix)

**Interfaces:**
- Consumes: all five arms' renders, the principles doc, the ideals
  (as calibration examples only), the grade definitions from Task 2.
- Produces: `grades.json` entries `{id, arm, grade, reason}` and the
  summary matrix the verdict cites.

**Requirements:**
- One judge call per message (Sonnet, fresh context): the judge gets
  the principles doc with grade definitions, one same-class ideal
  render as calibration (never the ideal of the message under
  judgment), the original HTML, and all five candidate renders
  unlabeled and shuffled, and returns a grade plus a one-line reason
  per render. Arm names are withheld to prevent brand bias.
- Aggregate to the matrix: per class per arm, the grade distribution.
- Fable audit: re-grade a stratified sample of ~20 (message, arm)
  pairs blind, compare to the judge grades, record the agreement rate
  and any systematic bias for the verdict.

**Steps:**
- [ ] Build and smoke-test the judge driver on 3 messages.
- [ ] Full grading run (~175 judge calls).
- [ ] Produce the summary matrix.
- [ ] Fable audit on the stratified sample; record agreement.
- [ ] Commit the driver.

**Acceptance:** Grades for every (message, arm) pair; summary matrix
on disk; audit agreement recorded; driver committed.

### Task 7: Verdict doc and phase close

**Executor:** Fable (main loop).

**Files:**
- Create: `docs/poplar/research/2026-07-19-rendering-bet-verdict.md`
- Modify: `docs/superpowers/specs/poplar-refounding-STATUS.md`

**Requirements:**
- The verdict covers, per the charter: per-class feasibility from the
  grade matrix; which techniques carried the weight (with the
  iteration notes as evidence); where the intelligence lives, meaning
  how much comprehension compiles into deterministic Go via offline
  rule-derivation and how much requires an LLM in the loop, with
  cost, latency, and offline implications from the measured stats;
  the fallback story for fail classes (filtered plain text, open in
  browser); and the implications for the Phase 2 vision.
- Sample renders for Geoff's gate read: point at local corpus paths
  per class (best and worst per arm); embed only content-scrubbed
  excerpts in the committed doc.
- No private mail content in the committed doc.

**Steps:**
- [ ] Author the verdict from the matrix, stats, audit, and iteration
      notes.
- [ ] Run the phase-end ritual from the STATUS doc (outcomes block,
      next-phase starter prompt, roadmap cursor, commit, push,
      memory refresh only if a load-bearing decision changed).
- [ ] Present the verdict and curated sample renders to Geoff for the
      gate ruling.

**Acceptance:** Verdict committed; STATUS updated and pushed; Geoff
has the gate packet. The gate ruling itself is Geoff's and ends the
phase.
