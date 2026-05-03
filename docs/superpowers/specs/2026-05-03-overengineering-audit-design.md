# Pass 8.5 — Overengineering Audit (Design)

**Date:** 2026-05-03
**Phase slot:** Pre-beta. Inserted before Pass 8.4c (Cache III).
**Author:** brainstorming session, 2026-05-03.

## Goal

Pre-1.0 hygiene sweep across the entire poplar codebase (~34k LOC,
98 Go files). Identify and remove:

- speculative abstractions (preemptive interfaces, Lazy Element types,
  Middle Man wrappers);
- dead scaffolding (unreachable functions, unused fields, stale
  TODO/commented blocks);
- defensive code for impossible paths (nil checks on guaranteed-non-
  nil values, error returns no caller branches on);
- one-call-site helpers (extract-method residue);
- zero-value-only config knobs (parameters every call site passes the
  same value to).

The pass is motivated by the LLM-overengineering tendency the project
has already pushed back on during normal passes. This is the catch-up
sweep for what slipped through despite that pushback.

Elm architecture conformance (a related but distinct correctness
concern in `internal/ui/`) is split off into Pass 8.5b — see
"Sibling pass" below.

## Non-goals

- Behavior changes beyond what deletion forces.
- Feature work.
- Performance tuning without prior measurement.
- Audit of vendored snippets in `internal/mailauth/` (provenance-
  marked, license-bound — not ours to simplify).
- Audit of generated files (none currently, but the policy is
  explicit).

## Scope (in)

All Go source under `cmd/poplar/` and `internal/` except
`internal/mailauth/`. Tests included where they encode dead production
code (e.g., a test for a function with no callers). Documentation,
ADRs, and `docs/poplar/invariants.md` are touched only insofar as
they reference deleted symbols.

## Pass position

| Pass  | Goal                                       | Status   |
|-------|--------------------------------------------|----------|
| 8.4b  | Cache II — body cache + CLI                | done     |
| 8.5   | **Overengineering audit (this pass)**      | pending  |
| 8.5b  | Elm architecture conformance audit (sibling) | pending  |
| 8.4c  | Cache III — outbox + offline overlays      | pending  |

Pass 8.4c remains queued. Pass 8.5 runs first because its deletions
may invalidate cache scaffolding that 8.4c would otherwise extend.
Pass 8.5b runs immediately after 8.5 so the Elm-conformance review
operates on the slimmer UI surface 8.5 produces. The 8.4c starter
prompt in STATUS.md does not change; the cache invariants section
may shrink as a side effect of 8.5.

## Phases

### Phase A — Static analysis baseline (mechanical, no LLM judgment)

Adds tooling, runs it, applies the trivially-safe deletions.

**Tooling:**
- Install `golang.org/x/tools/cmd/deadcode` (Go team RTA-based
  reachability from `main`).
- Install `mvdan.cc/unparam` (SSA-based always-same-value parameter
  detection; skips interface-satisfying funcs so signal is high).
- Confirm `staticcheck` and `golangci-lint` are present (already in
  `make lint`); update `.golangci.yml` to enable `unused` with
  `exported-fields-as-used: false`, `unparam`, and `staticcheck`
  checks U1000, SA4006, SA1019.

**Run:**
```
deadcode ./cmd/poplar > docs/poplar/audits/2026-05-03-deadcode.txt
unparam  ./...        > docs/poplar/audits/2026-05-03-unparam.txt
golangci-lint run     > docs/poplar/audits/2026-05-03-golangci.txt
```

**Apply:** Anything `deadcode` flags as unreachable from `main` is
deleted. Anything `unused` flags with no plausible consumer is
deleted. `unparam` findings are deferred to Phase D triage (some are
test fixtures; case-by-case).

**Single commit:** "Pass 8.5 A: static-analysis deletions" with the
three audit files committed alongside.

**Exit criterion:** `make check` passes; all three audit files
checked in; the residual (un-deleted) findings are the input to
Phase B.

### Phase B — Parallel subagent audit, one per package

Eight Explore-style agents dispatched in parallel. Each is scoped to
one package and reads the residual Phase A findings for that package.

**Packages and assignments:**

| Agent | Package | Special lens |
|---|---|---|
| 1 | `cmd/poplar/`     | CLI seam — config touchpoints, command wiring |
| 2 | `internal/cache/` | Cache II/III scaffolding; outbox state machine |
| 3 | `internal/mail/`  | Backend interface shape; classifier surface |
| 4 | `internal/mailjmap/` | JMAP backend; provider preset code paths |
| 5 | `internal/mailimap/` | IMAP backend; capability-negotiation paths |
| 6 | `internal/config/` | Config decoder; provider registry |
| 7 | `internal/theme/` + `internal/term/` + `internal/backoff/` | Leaf packages — combined into one agent for efficiency |
| 8 | `internal/ui/`    | Overengineering lens + UI/bubbletea-specific items (11–17). Elm architecture conformance is deferred to Pass 8.5b. |

The remaining packages (`internal/filter/`, `internal/content/`,
`internal/tidy/`) "await their consumers" per invariants — they have
no current call sites. Phase A's `deadcode` will catch any actually-
dead symbol; Phase B does not assign an agent to them.

**Common checklist (every agent runs this):**

1. Interfaces with exactly one implementation in the same package
   (Lindamood preemptive-interface anti-pattern).
2. Unexported functions with exactly one call site (inline
   candidates).
3. Struct fields never read outside their setter, or never set
   outside their constructor.
4. Function parameters / config fields where every call site passes
   the zero value.
5. Error returns on functions where every caller discards the error
   or routes it identically.
6. `errors.As` / `errors.Is` paths where no caller branches on the
   result.
7. Wrapper types whose only methods delegate verbatim to the wrapped
   type (Middle Man).
8. Types declared but only used as a field of one other type and
   never constructed independently (Lazy Element).
9. Defensive nil / length / range checks on values the call path
   guarantees.
10. Commented-out code, TODO/FIXME blocks older than 2 passes
    (check `git blame`).

**Agent 8 expanded checklist (UI-specific):**

11. `tea.Msg` types defined but never sent (`grep -rn "Msg{"` vs
    declarations).
12. `tea.Cmd`-returning helpers called from exactly one place
    (inline into the Update branch).
13. Defensive width/height clamps in `View()` that duplicate
    `clipPane`'s self-enforcement.
14. `lipgloss.NewStyle()` outside the two permitted files
    (`internal/ui/styles.go`, `internal/theme/palette.go`) — already
    an invariant, but verify.
15. Hex literals outside `internal/theme/themes.go` — same.
16. `len()` used on icon-bearing strings where `displayCells` is
    required (ADR-0083/0084).
17. `lipgloss.JoinHorizontal` / `JoinVertical` used in icon-bearing
    contexts (forbidden when `spuaCellWidth != 1` per ADR-0084).

**Output format per finding (every agent uses this):**

```
- file:line  — <category>  — <one-line description>
  Action: delete | inline | keep | refactor
  Rationale: <one sentence; if "keep," cite the consumer>
```

Categories use the checklist item numbers above (1–10 for common,
11–17 for UI-overengineering).

**Forced-deletion budget (evidence-grounded):**

- **Soft floor: ≥ 3 findings per package.** Anchored in Palomba et
  al. 2018 (EMSE, "On the Diffuseness and Impact on Maintainability
  of Code Smells") — smells affect ~2% of classes across 13 smell
  types in 30 OSS Java projects. For a ~4k LOC Go package with ~40–80
  meaningful units (types/functions at ~50–100 LOC average), expect
  ~1–3 true instances across the six target smell categories. A
  floor of 3 is the upper bound of expected yield; below it, the
  agent must justify with "nothing found because X" — no silent
  passes.
- **Hard ceiling: 10 findings per package.** Same density estimate.
  Above 10, the agent is reporting more smells than the code-property
  data predicts exist, indicating fabrication / false-positive drift.
  Above-ceiling packages stop and escalate to Phase C for human
  triage rather than aggregating to Phase D.

**Recall caveat (LLM-specific):** EASE25 (arxiv 2504.16027) measured
GPT-4.0 recall on these smells:

| Smell | F1 |
|---|---|
| Middle Man | 1.00 |
| Data Class | 1.00 |
| Dead Code | 0.91 |
| Long Parameter List | 0.59 |
| Speculative Generality | 0.09 |

LLM recall on Speculative Generality is essentially zero unprompted.
The floor cannot be met by Speculative Generality findings alone —
the agent must draw the floor primarily from the **high-recall
categories**: dead code, single-implementation interfaces (item 1),
one-call-site helpers (item 2), dead struct fields (item 3), Middle
Man wrappers (item 7), defensive nil/length checks (item 9), and
commented-out code (item 10). Speculative Generality / Lazy Element
findings (items 1, 8) are bonus, not floor-fill — forcing them risks
fabrication.

**Output files:**
`docs/poplar/audits/2026-05-03-pkg-<name>.md`, one per agent, eight
total.

**Exit criterion:** all eight files written; each contains either
≥ 5 findings or an explicit justification.

### Phase C — Cross-cutting re-read

Single-reviewer manual pass. Targets:

1. The 2-3 packages from Phase B with the most findings (highest
   complexity-per-LOC).
2. The seams between `internal/mail/` ↔ `internal/cache/` ↔
   `internal/ui/` for cross-package preemptive interfaces. Specific
   suspects to check:
   - `mail.ChangeTracker` — both backends implement it; is the
     interface earning its place or is it Middle Man over the two
     concrete impls?
   - `mail.Backend` — post-cutover (ADR-0121) it shrunk; verify no
     vestigial methods remain.
   - `cache.OpArgs` sealed sum — `SendArgs`/`AppendArgs` are reserved
     but unused (ADR-0117); is the reservation earning its place or
     is it speculative-generality?
3. `cmd/poplar/` ↔ `internal/config/` boundary: any flags / env vars
   wired up but unused at the consumer.

**Output:** `docs/poplar/audits/2026-05-03-cross-cutting.md` —
findings list in the same format as Phase B.

**Exit criterion:** doc written; ≥ 3 findings or explicit
"nothing found" justification.

### Phase D — Triage and apply

**Triage:** Aggregate Phases B + C into a single
`docs/poplar/audits/2026-05-03-overengineering-audit.md` with one
row per finding and the apply/skip decision.

**Skip-rationale guard** (reused verbatim from `/simplify`; CLAUDE.md
already encodes the matching pre-beta posture):

The only valid skip rationales are:
1. **Speculative future consumer** — and the consumer is a
   *named, scheduled* pass with a starter prompt in STATUS.md, not
   "Pass N might want this."
2. **Upstream-blocked** — requires a third-party change.
3. **Premature optimization without measurement** — efficiency
   findings only.

Forbidden skip rationales (invalidates the skip): "cross-package,"
"schema change," "would require interface change," "churn cost," "out
of scope," "non-trivial refactor."

**Apply order:** leaf packages first (lowest cross-package risk),
working toward `cmd/poplar`:

1. `internal/theme/` + `internal/term/` + `internal/backoff/`
2. `internal/mailjmap/`
3. `internal/mailimap/`
4. `internal/mail/`
5. `internal/cache/`
6. `internal/config/`
7. `internal/ui/`
8. `cmd/poplar/`

One commit per package, message format
`Pass 8.5 D-<n>: <pkg> overengineering deletions`. `make check`
between commits.

**Exit criterion:** all applied findings landed; all skipped
findings have a valid rationale recorded in the audit doc.

## Pass-end ritual (per `poplar-pass` skill)

1. Write ADRs for any architectural deletions (e.g., if
   `mail.ChangeTracker` collapses into the concrete backends, ADR
   it). Numbering continues from 0124 → 0125+.
2. Update `docs/poplar/invariants.md` to reflect deleted types,
   interfaces, fields. Edit-in-place per the file's own rule.
3. Update `.claude/rules/ui-invariants.md` if `internal/ui/`
   deletions changed any binding fact there.
4. Update STATUS.md: Pass 8.5 marked done; 8.5b and 8.4c rows
   confirmed; 8.4c starter prompt reviewed and shrunk if
   invariants reduced its surface.
5. Final pass-end commit, push, `make install`.

## Test plan

- `make check` between every commit (vet + full test suite).
- Live UI verification via tmux at 80×24 and 120×40 after Phase D
  step 7 (the `internal/ui/` apply commit). Captures attached to the
  pass-end commit.
- No new tests required *unless* a deletion exposed coverage gaps in
  surrounding code; in that case, add the test in the same commit as
  the deletion.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Agent-generated weak findings to hit the ≥ 3 floor | Floor is anchored in Palomba 2018 expected-yield data, not picked to maximize pressure; floor is drawable from high-LLM-recall categories per EASE25, so the agent isn't forced to fabricate the categories it can't reliably detect. Phase D triage applies the skip-rationale guard as a second filter. |
| Agent fabrication of low-recall smells (Speculative Generality) to fill the floor | Recall caveat in the spec instructs the agent to draw the floor from high-recall categories only. Speculative Generality findings are bonus, not floor-fill. |
| Deleting a symbol Pass 8.4c needed | Phase B agent for `internal/cache/` re-reads the 8.4c starter prompt before producing findings. Cross-cutting Phase C explicitly checks the cache↔mail↔ui seams. |
| Cross-package deletion cascades break `make check` mid-apply | Apply order is leaf-first. `make check` after every commit. If a commit breaks, fix or revert in-place — no batched recovery. |
| `internal/ui/` overengineering findings spiral into a rewrite | Hard ceiling of 10 findings per package. If exceeded, escalate to Phase C for human triage rather than rolling into 8.5. |
| LLM blind to its own prior overengineering (model reviewing self) | Mitigation 1: ground each agent in static-analysis output before judgment (research shows this raises recall). Mitigation 2: forced-deletion budget pushes past hedging. Mitigation 3: Phase C is human-led, breaks the self-review loop on the highest-risk surfaces. |

## Sibling pass — 8.5b (Elm architecture conformance audit)

Pass 8.5b runs immediately after 8.5 and is scoped exclusively to
`internal/ui/`. It uses the same Phase A → B → C → D shape, but with
a single agent (no fan-out — one package), and the checklist is the
Elm-conformance items previously folded into 8.5:

1. State held outside `tea.Model` structs (package-level vars,
   closures, singletons).
2. Mutations to model state outside `Update`.
3. I/O performed inside `Update` or `View` rather than returned as
   `tea.Cmd`.
4. Child models holding state that the parent also holds (mirroring
   via Msg instead of accessor reads).
5. Parents reading child state without going through an accessor
   (direct field access across the tree boundary).
6. `tea.Msg` used as a state-mirror channel (child sends a Msg to
   inform the parent of state the parent could read directly).
7. `WindowSizeMsg` handlers that fail to both `SetSize` children
   and forward the msg.
8. Keys not declared as `key.Binding` or not dispatched via
   `key.Matches`.
9. Components that don't honor the size contract (`View()` not
   self-guarded by `clipPane`; renderers ignoring `width`).

Pass 8.5b gets its own design spec at
`docs/superpowers/specs/2026-05-03-elm-conformance-audit-design.md`,
written after 8.5 lands so it can incorporate any structural changes
8.5 produces. It is mentioned here only so 8.5's authors and
reviewers know the Elm work is queued and not lost.

## Open questions

None. All design decisions resolved in brainstorming.

## Sources informing this design

- Code smell density: Palomba et al. 2018 (EMSE), "On the
  Diffuseness and Impact on Maintainability of Code Smells" — 30 OSS
  Java projects, 395 releases, ~2% class-level smell prevalence
  across 13 smell types. Anchors the forced-deletion floor.
- LLM smell-detection recall: EASE25 / arxiv:2504.16027,
  "Benchmarking LLM for Code Smells Detection: GPT-4.0 vs DeepSeek-V3"
  — Middle Man F1=1.00, Dead Code F1=0.91, Speculative Generality
  F1=0.09. Anchors the recall caveat.
- LLM context degradation: Chroma "Context Rot" technical report
  (2025) — frontier models degrade with input length; informs the
  ceiling-as-alarm framing.
- Go static analysis: `golang.org/x/tools/cmd/deadcode` blog post
  (Go team, 2024); `mvdan/unparam` README.
- Meta SCARF dead-code program (Engineering at Meta, 2023).
- Lindamood preemptive-interface anti-pattern (Medium).
- Muratori "Semantic Compression" (caseymuratori.com/blog_0015).
- Existing Claude skills: `ksimback/tech-debt-skill`,
  `fastruby/tech-debt-skill`, MCPMarket dead-code cluster.
- hamy.xyz "9 Parallel AI Agents" code-review setup (Feb 2026).
- Anthropic official `code-review` plugin (March 2026).
- Project's own `/simplify` skill (skip-rationale guard borrowed).

## Implementation handoff

After this spec is approved by the user, transition to the
`writing-plans` skill to produce the executable plan at
`docs/superpowers/plans/2026-05-03-overengineering-audit.md`.
