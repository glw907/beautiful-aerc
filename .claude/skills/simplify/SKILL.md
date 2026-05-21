---
name: simplify
description: Review poplar's changed Go code for reuse, quality, efficiency, and human voice, then fix any issues found. The Go-aware simplify; the generic version lives in user skills.
---

# Simplify: Code Review and Cleanup

Review all changed files for reuse, quality, efficiency, and — when the diff includes Go files — human voice. Fix any issues found.

## Phase 1: Identify Changes

Run `git diff` (or `git diff HEAD` if there are staged changes) to see what changed. If the diff doesn't include untracked files, run `git add -N` on them first so they appear in the diff. If there are no git changes, review the most recently modified files that the user mentioned or that you edited earlier in this conversation.

Note whether the diff includes any Go files. If yes, dispatch four agents (reuse, quality, efficiency, voice). If no Go files, dispatch three (skip voice).

## Phase 2: Launch Review Agents in Parallel

Use the Agent tool to launch all agents concurrently in a single message. Pass each agent the full diff so it has the complete context.

### Agent 1: Code Reuse Review

For each change:

1. **Search for existing utilities and helpers** that could replace newly written code. Look for similar patterns elsewhere in the codebase — common locations are utility directories, shared modules, and files adjacent to the changed ones.
2. **Flag any new function that duplicates existing functionality.** Suggest the existing function to use instead.
3. **Flag any inline logic that could use an existing utility** — hand-rolled string manipulation, manual path handling, custom environment checks, ad-hoc type guards, and similar patterns are common candidates.

### Agent 2: Code Quality Review

Review the same changes for hacky patterns:

1. **Redundant state**: state that duplicates existing state, cached values that could be derived, observers/effects that could be direct calls
2. **Parameter sprawl**: adding new parameters to a function instead of generalizing or restructuring existing ones
3. **Copy-paste with slight variation**: near-duplicate code blocks that should be unified with a shared abstraction
4. **Leaky abstractions**: exposing internal details that should be encapsulated, or breaking existing abstraction boundaries
5. **Stringly-typed code**: using raw strings where constants, enums (string unions), or branded types already exist in the codebase
6. **Unnecessary JSX nesting**: wrapper Boxes/elements that add no layout value — check if inner component props (flexShrink, alignItems, etc.) already provide the needed behavior
7. **Nested conditionals**: ternary chains (`a ? x : b ? y : ...`), nested if/else, or nested switch 3+ levels deep — flatten with early returns, guard clauses, a lookup table, or an if/else-if cascade
8. **Unnecessary comments**: comments explaining WHAT the code does (well-named identifiers already do that), narrating the change, or referencing the task/caller — delete; keep only non-obvious WHY (hidden constraints, subtle invariants, workarounds)

### Agent 3: Efficiency Review

Review the same changes for efficiency:

1. **Unnecessary work**: redundant computations, repeated file reads, duplicate network/API calls, N+1 patterns
2. **Missed concurrency**: independent operations run sequentially when they could run in parallel
3. **Hot-path bloat**: new blocking work added to startup or per-request/per-render hot paths
4. **Recurring no-op updates**: state/store updates inside polling loops, intervals, or event handlers that fire unconditionally — add a change-detection guard so downstream consumers aren't notified when nothing changed. Also: if a wrapper function takes an updater/reducer callback, verify it honors same-reference returns (or whatever the "no change" signal is) — otherwise callers' early-return no-ops are silently defeated
5. **Unnecessary existence checks**: pre-checking file/resource existence before operating (TOCTOU anti-pattern) — operate directly and handle the error
6. **Memory**: unbounded data structures, missing cleanup, event listener leaks
7. **Overly broad operations**: reading entire files when only a portion is needed, loading all items when filtering for one
8. **Pre-modern Go stdlib idioms** (Go diffs only, when `go.mod` is `>= 1.21`): surface as findings, do not auto-fix. Mirror the precision-over-recall bias of the voice agent — false positives waste apply-phase time.
   - `sync.Once` + package-level result var that `sync.OnceValue[T]` / `sync.OnceFunc` would collapse to one declaration.
   - Push-callback iterators (`func(emit func(T))` shapes, hand-rolled `Next`/`Stop` pairs) with ≥ 2 call sites — candidates for `iter.Seq[T]` / `iter.Seq2[K,V]`. Single-caller helpers stay.
   - Raw `fmt.Fprintf(os.Stderr, ...)` for log-shaped messages inside `internal/` packages — `log/slog` belongs here. `cmd/` stays on stderr for user-facing startup errors.
   - Hand-rolled multi-error accumulation (`var errs []error; … errs = append(errs, err); …`) where `errors.Join(errs...)` fits.
   - Nil/zero-coalescing chains (`if a != "" { return a }; if b != "" { ... }; return c`) where `cmp.Or(a, b, c)` fits.
   - Leftover `x := x` loop-variable shadow lines (pre-1.22 workaround; every loop variable is per-iteration since 1.22).

**Grep-tier idioms already covered by tooling — do not re-scan.** If the project ships `scripts/modern-go-check.sh` (poplar: yes, run by `make check`), it catches `sort.SliceStable` / `sort.Slice`, `sort.Strings` / `sort.Ints`, `for i := 0; i < N` with unused `i`, and package-scope `sync.Once` mechanically with zero false-positives. Skip those — they'll surface on the next `make check`. Spend attention on the semantic findings above.

### Agent 4: Voice Review (Go diffs only)

Skip this agent if the diff has no Go files.

The agent must first read `~/.claude/docs/go-comment-voice.md` to load the §0 write-time gate, the §1 placement rubric, and the full T1–T40 catalogue (§7 plus the §9b calibrated pairs). Then scan the Go portion of the diff for tells. Each finding cites the tell number, quotes the offending lines, and quotes the avoidance rule. Bias toward precision over recall — false positives on voice waste apply-phase time.

**Primary check: the §0 paraphrase test.** Run this *before* the catalogue scans. For every in-function comment in the diff, ask: *does this comment paraphrase the next ≤5 lines?* If yes, flag it under T39 with the paraphrase as the avoidance rule. This is the highest-value scan in the agent — the audit that motivated T38–T40 found 1,227 in-function comments clustered at structural seams paraphrasing the lines below. Calibration: the bar is *paraphrasing the next ≤5 lines*, not *summarizing a package*. Don't fire on legitimate package summaries or contract-bearing godocs.

**Grep-tier tells already covered by tooling — do not re-scan.** If the project ships a voice-check script (poplar: `scripts/voice-check.sh`, run by `make check`), it catches T4 (`for now` / stub TODO), T10 (`"failed to"` chorus), T14 (`Get*` getter prefix), T16 (Manager/Helper/Util/Service suffix), T27 (apologetic phrases), T28 (over-explained Go idioms), T33 (em dash in comment), T34 (semicolon clause-joiner in comment), T35 (documentation labels in comments), the narrow label-colon godoc form of T39, T40a (`NOTE:` / `IMPORTANT:` / `TODO:` prefixes), and T41 (SPDX header) mechanically with zero false-positives. Skip those tells in this agent — they're either already clean or the apply-phase will surface them on the next `make check`. Spend attention on the semantic tells below.

Semantic tells to scan for, by category:

1. **Comment tells (T1–T3, T5–T9):** WHAT-comments restating obvious code (T1); godoc on unexported symbols where the name suffices (T2); uniform comment density across functions of different complexity (T3); task-framing comments ("added for X flow", "used by Y", "fixes #N") (T5); first-person plural in unexported docs (T6); every doc beginning "Foo does X" (T7); multi-paragraph docstrings on self-describing functions (T8); per-case docstrings in table tests (T9). T4 is grep-tier — skip.

2. **Error-phrasing tells (T10b, T11–T13):** cross-function error chorus in one file (T10b); adjacent error sites reading identically within one function (T11); function name embedded in its own error string (T12); bare `%w` wrapping where no caller branches on the sentinel (T13). T10 is grep-tier — skip.

3. **Naming tells (T15, T17, T18):** package-doubled types (`mail.MailMessage`) (T15); over-descriptive locals in tight scopes (T17); exported names that read like docstrings (T18). T14 and T16 are grep-tier — skip.

4. **Structural tells (T19–T23):** reflexive `doc.go` / `errors.go` / `types.go` skeleton (T19); single-impl interfaces with no test fake or DI seam (T20); `New<X>` constructors that only set fields a struct literal would set (T21); defensive nil checks between same-package functions (T22); length checks before indexing on internal callers (T23).

5. **Test tells (T24–T26):** identical assertion phrasing copy-pasted across files (T24); tautological cases (T25); subtests for trivial scalar functions (T26).

6. **Voice tells (T29–T32):** uniform sentence length across a file (T29); identical paragraph rhythm (T30); uniform verbosity (identical doc shape and length) (T31); `Builder` patterns where a struct literal would suffice (T32). T27 and T28 are grep-tier — skip.

7. **Structural tells (T38–T40):** comment frequency tracking library density (~15%) on application code rather than the ~7–9% application-code norm (T38); section-boundary commenting at structural seams that paraphrases the next 3–5 lines (T39 — see the primary check above; the broader paraphrase form sits here, the narrow `^// Foo: ` label-colon godoc is grep-tier); markdown shape leaking into godoc — closing aphoristic summary sentences, reference-stuffing with multiple ADR/RFC cites per godoc (T40 — the `NOTE:`/`IMPORTANT:`/`TODO:` prefix form is grep-tier, the rest stays here). Calibrated examples for all three live in §9b of the voice doc.

Output format per finding:

```
T<n> at path/to/file.go:LINE
  <quoted offending line(s)>
  Avoidance: <one-line rule from the catalogue>
```

## Phase 3: Triage

Wait for all three agents to complete. Aggregate their findings before deciding which to apply.

### Triage anti-pattern guard

**Before marking any finding as "skip," verify the rationale is not one of:**

- "Cross-package" / "cross-3-files" / "non-trivial refactor"
- "Schema change required"
- "Would require interface change" / "would require backend change"
- "Churn cost" / "out of scope"

**These are forbidden skip rationales when the project's `CLAUDE.md` (or analogous file) establishes a refactor-friendly posture** — pre-beta, alpha, "code quality outweighs stability," "refactor freely," "breaking changes first-class," or similar. In those projects the rationales above describe *exactly* the work the project posture endorses, so they cannot be used to defer.

**The only valid skip rationales are:**

1. **"Speculative future consumer"** — the finding adds a field, type, or hook for a consumer that doesn't yet exist and isn't immediately needed. Read the code: if no current call site benefits, skip is fair.
2. **"Upstream-blocked"** — the finding requires a change to a third-party dependency that the project doesn't control or vendor.
3. **"Premature optimization without measurement"** — for efficiency findings only, where the hot path hasn't been profiled and the current shape is bounded.

If none of those three apply, the finding must be applied in this pass.

When you've decided what to apply, state the apply/skip split explicitly (one line per skip with its rationale tagged) and then proceed to Phase 4.

## Phase 4: Fix Issues

Apply each finding directly. When done, briefly summarize what was fixed (or confirm the code was already clean).
