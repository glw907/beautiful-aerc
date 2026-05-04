# Pass 8.8 / 10.6 — Human-Voice Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Strip AI-fingerprint patterns from poplar's Go code so
the codebase reads as human-authored. Codify the rules forward in
ADR-0141, a research-grounded style guide, `go-conventions` skill
update (including an experienced-Go-developer persona), and
`/simplify` voice lens.

**Architecture:** Six phases across two passes. The audit
(Phases 0–2) and codify-forward (Phase 5) live in **Pass 8.8**.
Apply (Phase 3) splits across both passes by risk profile:

- **Pass 8.8 Phase 3a — string-only fixes:** C1 (comments), C7
  (error phrasing), C4 (uniform prose verbosity). Reviewer-friendly
  diffs, near-zero behavior risk, validates the style guide in
  production before structural risk lands.
- **Pass 8.9 Phase 3b — structural fixes:** C2 (defensive cruft),
  C3 (premature abstraction), C5 (naming/renames), C6 (test
  boilerplate), C8 (structural symmetry). Carries `make check` and
  tmux-render risk; deferred so the style guide soaks one pass
  before applying at scale.

**Phase summary:** **0** produces the research-backed style guide
+ infrastructure updates (skill, /simplify, persona, CLAUDE.md
pointer). **1** dispatches eight parallel review subagents
producing categorized findings; main thread does the cross-cutting
sweep. **2** is single-reviewer triage with apply/keep/taste-call
decisions; spec is frozen at phase end and applies are tagged for
10.5 vs. 10.6. **3a** lands the string-only batches in 10.5; **3b**
lands the structural batches in 10.6, against the already-frozen
triage list. **4** verifies (each pass independently — `make check`,
tmux at 80×24 + 120×40 after structural). **5** codifies forward
via ADR + style guide + skill + persona + `/simplify` voice lens
(in 10.5, since 10.6 inherits the same artifacts).

**Tech Stack:** Go 1.26.1, Claude Code Agent tool for parallel
review dispatch, tmux for live UI verification.

**Working directory:** master branch (per project convention; no
worktree).

**Spec:** `docs/superpowers/specs/2026-05-04-human-voice-audit-design.md`

---

## File map

**New files (created during the pass):**
- `docs/poplar/audits/2026-05-04-pkg-cmd-poplar.md`
- `docs/poplar/audits/2026-05-04-pkg-cache.md`
- `docs/poplar/audits/2026-05-04-pkg-mail.md`
- `docs/poplar/audits/2026-05-04-pkg-mailjmap.md`
- `docs/poplar/audits/2026-05-04-pkg-mailimap.md`
- `docs/poplar/audits/2026-05-04-pkg-config.md`
- `docs/poplar/audits/2026-05-04-pkg-leaves.md` — theme + term + backoff + filter + content + tidy
- `docs/poplar/audits/2026-05-04-pkg-ui.md`
- `docs/poplar/audits/2026-05-04-cross-cutting.md`
- `docs/poplar/audits/2026-05-04-human-voice.md` — final aggregated triage
- `docs/poplar/decisions/0138-human-voice-policy.md`

**Created during Phase 0 (style guide + infrastructure):**
- `docs/poplar/research/2026-05-04-go-comment-voice.md` — the
  research-synthesized style guide; load-bearing artifact for all
  downstream work. (Source files in `research/sources/` and the
  synthesis brief stay in place as provenance.)

**Modified files (during apply phase, scope-dependent):**
- All files under `cmd/poplar/` and `internal/` except `internal/mailauth/`.
- `~/.claude/skills/go-conventions/SKILL.md` — append "Human voice
  & AI tells" section carrying §7 catalogue inline + persona
  preamble (Phase 5 / Pass 8.8).
- `~/.claude/skills/simplify/SKILL.md` — add voice lens that scans
  diff against §7 catalogue by name (Phase 5 / Pass 8.8).
- `CLAUDE.md` (poplar root) — Human voice section gets pointer to
  style guide + persona reference (Phase 5 / Pass 8.8).
- `docs/poplar/STATUS.md` — Pass 8.8 + 10.6 rows.
- `docs/poplar/invariants.md` — decision-index row for ADR-0141
  (10.5) and ADR-0142 if 10.6 produces one.

---

## Phase 0 — Style guide + infrastructure (Pass 8.8)

Goal: produce the research-grounded style guide and propagate it to
the standards before any audit runs. Done in 10.5 so 10.5 *and* 10.6
both apply against the same calibrated guide.

### Task 0.1 — Research (parallel)

- [x] Dispatch four `general-purpose` agents (authoritative docs;
  stdlib exemplars; third-party exemplars; essays/proverbs).
  Outputs land in `docs/poplar/research/sources/`.
- [x] Verify all four files exist and meet length targets.

### Task 0.2 — Synthesize style guide

- [ ] Dispatch synthesis agent against the four source files +
  `docs/poplar/research/2026-05-04-synthesis-brief.md`.
- [ ] Output: `docs/poplar/research/2026-05-04-go-comment-voice.md`
  with §7 AI-tells catalogue as load-bearing section, voice palette
  decision in §4, and propagation plan in §9.
- [ ] Human review pass — verify voice choice, unexported-godoc
  default, and no invented examples.

### Task 0.3 — Update standards

- [ ] `~/.claude/skills/go-conventions/SKILL.md` — append "Human
  voice & AI tells" section: persona preamble + §7 catalogue
  inline + mechanical avoidance rules per tell. Skill is global
  (benefits every Go project on this workstation).
- [ ] `~/.claude/skills/simplify/SKILL.md` — add fourth parallel
  reviewer: voice lens. Scans diff against §7 by name; each
  finding cites tell number + avoidance rule.
- [ ] `CLAUDE.md` (poplar root) — Human voice section: keep short
  rules, add pointer to style guide and to persona section in
  go-conventions.
- [ ] Test the /simplify voice lens on a synthetic diff containing
  one finding from each of the 32+ tells. Confirm voice agent
  flags every category. Iterate the skill prompt until it does.

### Task 0.4 — Calibration spike

- [ ] Run a single `Explore` agent on **one** package (e.g.,
  `internal/cache/`) against the new style guide.
- [ ] Human review of 10–20 findings to confirm calibration.
- [ ] Iterate the audit prompt template (in Phase 1.1) until the
  signal-to-noise ratio is acceptable.

**Commit (per artifact, not bundled):**
- `Pass 8.8: research synthesis — go comment voice style guide`
- `Pass 8.8: go-conventions skill — human voice + AI tells`
- `Pass 8.8: /simplify — voice lens`
- `Pass 8.8: CLAUDE.md — point Human voice section at style guide`

---

## Phase 1 — Audit (read-only)

Goal: produce categorized, line-numbered findings. No edits.
No triage. No recommendations beyond category tags.

### Task 1.1 — Dispatch parallel package audits

- [ ] Dispatch eight `Explore` subagents in a single message
  (parallel). Each agent receives:
  - The C1–C8 category definitions from the spec.
  - The package path it owns.
  - Output path: `docs/poplar/audits/2026-05-04-pkg-<name>.md`.
  - Format: each finding records file:line(s), category tag,
    verbatim excerpt (≤6 lines), one-line diagnosis.
  - Explicit instruction: do NOT propose fixes, do NOT triage.
- [ ] Package assignments:
  - `cmd-poplar` — `cmd/poplar/`
  - `cache` — `internal/cache/`
  - `mail` — `internal/mail/`
  - `mailjmap` — `internal/mailjmap/`
  - `mailimap` — `internal/mailimap/`
  - `config` — `internal/config/`
  - `leaves` — `internal/theme/`, `internal/term/`,
    `internal/backoff/`, `internal/filter/`, `internal/content/`,
    `internal/tidy/`
  - `ui` — `internal/ui/`
- [ ] Wait for all eight to complete. Verify each produced its
  output file.

### Task 1.2 — Cross-cutting sweep

- [ ] Main thread reads each per-package findings file.
- [ ] Identify patterns no single-package agent could catch:
  repeated error-phrasing templates across packages, identical
  comment shapes across files, identical test-case phrasing
  across packages, mirror-imaged file naming.
- [ ] Write `docs/poplar/audits/2026-05-04-cross-cutting.md`.

### Task 1.3 — Aggregate

- [ ] Concatenate package findings + cross-cutting into
  `docs/poplar/audits/2026-05-04-human-voice.md`.
- [ ] Group by category (C1 first, C8 last) for triage.
- [ ] Tally: count of findings per category, per package.
- [ ] Commit Phase 1 output.

**Commit:** `Pass 8.8: Phase 1 — human-voice audit findings`

---

## Phase 2 — Triage

Goal: every finding marked `apply` / `keep` / `taste-call`. Spec
frozen at end of phase.

### Task 2.1 — Triage pass

- [ ] Walk findings in category order. For each:
  - **apply** if the tell is clear and the fix is mechanical.
  - **keep** if surface-level pattern is actually justified
    (real seam, real boundary, load-bearing test). Annotate with
    one-line reason.
  - **taste-call** if reasonable people disagree. Make the call
    here. No taste-calls during Phase 3.
- [ ] Skip-rationale guard: reject any defer framed as
  "cross-package," "non-trivial refactor," "churn cost," "out of
  scope." Pre-beta posture endorses all of those.

### Task 2.2 — Plan-doc updates

- [ ] For each `apply` finding, append a checkbox task to the
  matching Phase 3a (10.5) or Phase 3b (10.6) category section
  below. Routing rule: C1 / C7 / C4-prose → 3a; C2 / C3 / C5 /
  C6 / C8 → 3b. Include file:line reference.
- [ ] Tally per-category apply counts; if any category has zero
  applies, note that — its commit is skipped.

**Commit:** `Pass 8.8: Phase 2 — triage decisions`

---

## Phase 3a — Apply string-only batches (Pass 8.8)

Goal: land C1 + C7 + C4 prose findings in 10.5. One commit per
category, `make check` green between.

> **Append apply-tasks during Phase 2.** Each `apply` finding
> tagged for 10.5 becomes a checkbox task under its category
> below.

### Task 3a.1 — C1 Comments

- [ ] Apply C1 findings (comment rot).
- [ ] `make check`.
- [ ] Commit: `Pass 8.8: C1 — strip comment rot`

### Task 3a.2 — C7 Error phrasing

- [ ] Apply C7 findings. Vary error phrasing site-by-site.
  Match the call site, not a template.
- [ ] `make check`.
- [ ] Commit: `Pass 8.8: C7 — vary error phrasing`

### Task 3a.3 — C4 Uniform verbosity (prose only)

- [ ] Apply C4 findings whose fix is doc-comment-shape variation
  or error-shape variation (most subsumed by C1 + C7).
  Function-length-distribution findings defer to 10.6 if
  structural.
- [ ] `make check`.
- [ ] Commit: `Pass 8.8: C4 — break uniform prose verbosity`
  (skip if no findings remain).

## Phase 3b — Apply structural batches (Pass 8.9)

Goal: land C2 + C6 + C5 + C3 + C8 against the already-frozen Phase
2 triage list. Each commit `make check` green; live tmux render
at 80×24 + 120×40 after C5 and C3.

### Task 3b.1 — C2 Defensive cruft

- [ ] Apply C2 findings. Remove nil checks on internal callers,
  uniform `failed to X: %w` wrapping in code paths C7 didn't
  cover, between-internal-functions validation.
- [ ] `make check`.
- [ ] Commit: `Pass 8.9: C2 — strip defensive cruft`

### Task 3b.2 — C6 Test boilerplate

- [ ] Apply C6 findings. Per-case docstrings, tautological cases,
  identical assertion phrasing.
- [ ] **Coverage guard:** for any test case removed, run the test
  with the case removed AND mutate the production code; if the
  test still passes with the mutation, the case was load-bearing
  — restore it.
- [ ] `make check`.
- [ ] Commit: `Pass 8.9: C6 — strip test boilerplate`

### Task 3b.3 — C5 Naming

- [ ] Apply C5 findings. Renames: package-doubled types,
  `GetX` → `X`, `Manager`/`Helper`/`Util` suffix removal,
  over-descriptive locals.
- [ ] `make check` after each rename batch (renames ripple).
- [ ] Live tmux render at 80×24 and 120×40 — renderers can read
  field names indirectly via reflection-adjacent code paths.
- [ ] Commit: `Pass 8.9: C5 — rename for human voice`

### Task 3b.4 — C3 Premature abstraction

- [ ] Apply C3 findings. Inline single-impl interfaces (where no
  ADR'd seam exists), zero-line wrappers, trivial `New<X>`
  constructors.
- [ ] **Real-seam guard:** before inlining any interface,
  grep for ADR references and test fakes. `mail.Backend` and
  `mail.ChangeTracker` stay (real JMAP+IMAP impls).
- [ ] `make check`.
- [ ] Live tmux render — interface inlining can ripple to
  renderers.
- [ ] Commit: `Pass 8.9: C3 — inline premature abstractions`

### Task 3b.5 — C8 Structural symmetry

- [ ] Apply C8 findings. Reflexive `doc.go` deletion, `errors.go`
  collapse where there's one error, file consolidation.
- [ ] `make check`.
- [ ] Commit: `Pass 8.9: C8 — break structural symmetry`

---

## Phase 4 — Verify (each pass independently)

Both passes run their own Phase 4 before shipping. Pass 8.8's
verify is light (string-only changes); 10.6's verify includes
the tmux render step.

### Task 4.1 — Final make check

- [ ] `make check` from clean state.
- [ ] `make install`; smoke test `poplar` CLI.

### Task 4.2 — Live UI verification (Pass 8.9 only)

- [ ] tmux render at 80×24, capture screenshot.
- [ ] tmux render at 120×40, capture screenshot.
- [ ] Compare against Pass 8.7 screenshots — no visual regression.

### Task 4.3 — Voice spot-check (Pass 8.8 — string-only)

- [ ] Pick three files at random (e.g., `shuf -n3` over the
  Go source list). Read each top-to-bottom.
- [ ] If any reads as machine-uniform on comments/errors, log a
  follow-up issue — do NOT extend Phase 3a. The audit caught what
  it caught; remaining tells are guide-failures, not apply-failures.
  Structural tells will still read AI-shaped after 10.5 — those are
  10.6's job.

### Task 4.4 — `/simplify` over the diff

- [ ] Run `/simplify` per global rule. Apply genuine wins.
- [ ] Commit: `Pass 8.8: /simplify cleanup` (or `Pass 8.9:` per
  pass).

---

## Phase 5 — Codify forward (Pass 8.8)

The skill / `/simplify` / CLAUDE.md updates already landed in
Phase 0 — they had to land before the audit could use them. This
phase writes ADR-0141 (which references the now-shipped
infrastructure) and updates invariants + STATUS.

### Task 5.1 — ADR-0141

- [ ] Write `docs/poplar/decisions/0138-human-voice-policy.md`.
- [ ] Body: rationale (contributor-recruitment threat model);
  pointer to the style guide as the load-bearing artifact;
  pointer to the §7 AI-tells catalogue; pointer to the
  go-conventions persona; explicit naming of the 8.8/8.9 split
  and why. Two or three before/after examples drawn from real
  audit findings (not invented).
- [ ] CLAUDE.md "Human voice" section already points at the guide
  per Phase 0 — no change here.

### Task 5.2 — invariants.md + STATUS.md

- [ ] Append to `docs/poplar/invariants.md` decision index:
  ```
  | Human-voice policy — research-grounded style guide, persona,
  /simplify voice lens, go-conventions update, 8.8/8.9 split | 0138 |
  ```
- [ ] Update `docs/poplar/STATUS.md`: mark Pass 8.8 done; advance
  current-pass marker to 10.6.

### Task 5.3 — Archive (Pass 8.8)

- [ ] **Do NOT archive plan or spec yet** — Pass 8.9 reuses
  both. Archive only happens at the end of 10.6.
- [ ] Audit findings stay in `docs/poplar/audits/` until 10.6
  ships; 10.6's Phase 3b reads from the same triage list.

### Task 5.4 — Ship (Pass 8.8)

- [ ] Final `make check`.
- [ ] Commit Phase 5 changes:
  `Pass 8.8: ADR-0141, invariants, STATUS`.
- [ ] `git push`.
- [ ] `make install`.

## Phase 6 — Pass 8.9 close-out

Runs only after Phase 3b is complete and Pass 8.9's Phase 4
verify is green.

### Task 6.1 — Optional ADR-0142

- [ ] If 10.6 surfaced new policy decisions beyond ADR-0141's
  scope (e.g., a real-seam exception list, a renaming convention
  for compound types), write ADR-0142.
- [ ] Otherwise skip — 10.6 is implementation against ADR-0141.

### Task 6.2 — Archive plan + spec + audit

- [ ] Move spec → `docs/superpowers/archive/specs/`.
- [ ] Move plan → `docs/superpowers/archive/plans/`.
- [ ] Move audit findings → `docs/poplar/audits/archive/`.

### Task 6.3 — Ship (Pass 8.9)

- [ ] Final `make check` + tmux render verification.
- [ ] Commit + push + `make install`.
- [ ] Update STATUS.md: Pass 8.9 done; advance to Pass 11.

---

## Acceptance

**Pass 8.8:**
- Style guide written and reviewed; AI-tells catalogue load-bearing.
- `go-conventions` skill carries persona + §7 catalogue inline.
- `/simplify` voice lens flags every tell category on a synthetic
  diff.
- CLAUDE.md "Human voice" points at the guide.
- Calibration spike confirms audit prompt produces actionable
  signal.
- Phase 1 audit committed; Phase 2 triage frozen with each apply
  tagged 10.5 or 10.6.
- C1 + C7 + C4-prose batches committed; `make check` green.
- ADR-0141 written; invariants index updated.
- Voice spot-check on three random files: comment + error voice
  reads as human (structural tells acceptably remaining for 10.6).

**Pass 8.9:**
- C2 + C6 + C5 + C3 + C8 batches committed; `make check` green at
  every commit; tmux render at 80×24 + 120×40 unchanged.
- Plan + spec + audit findings archived.
- Voice spot-check on three random files reads as fully human.
