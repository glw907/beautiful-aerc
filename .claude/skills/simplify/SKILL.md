---
name: simplify
description: Review poplar's changed Go code for reuse, quality, efficiency, and human voice, then fix any issues found. The Go-aware simplify; the generic version lives in user skills.
---

# Simplify: Code Review and Cleanup

Review all changed files for reuse, quality, and efficiency. When the diff includes Go files, also review for human voice. Fix any issues found.

## Phase 1: Identify Changes

Run `git diff` (or `git diff HEAD` if there are staged changes) to see what changed. If the diff doesn't include untracked files, run `git add -N` on them first so they appear in the diff. If there are no git changes, review the most recently modified files that the user mentioned or that you edited earlier in this conversation.

Note whether the diff includes any Go files. If yes, dispatch four agents (reuse, quality, efficiency, voice). If no Go files, dispatch three (skip voice).

## Phase 2: Launch Review Agents in Parallel

Use the Agent tool to launch all agents concurrently in a single message. Pass each agent the full diff so it has the complete context.

### Agent 1: Code Reuse Review

For each change:

1. **Search for existing utilities and helpers** that could replace newly written code. Look for similar patterns elsewhere in the codebase; common locations are utility directories, shared modules, and files adjacent to the changed ones.
2. **Flag any new function that duplicates existing functionality.** Suggest the existing function to use instead.
3. **Flag any inline logic that could use an existing utility.** Hand-rolled string manipulation, manual path handling, custom environment checks, and ad-hoc type assertions are common candidates.

### Agent 2: Code Quality Review

Review the same changes for hacky patterns:

1. **Redundant state**: state that duplicates existing state, cached values that could be derived, observers/callbacks that could be direct calls
2. **Parameter sprawl**: adding new parameters to a function instead of generalizing or restructuring existing ones
3. **Copy-paste with slight variation**: near-duplicate code blocks that should be unified with a shared abstraction
4. **Leaky abstractions**: exposing internal details that should be encapsulated, or breaking existing abstraction boundaries
5. **Stringly-typed code**: using raw strings where the codebase already defines named constants or a string/iota-based type
6. **Unnecessary wrapper layers**: lipgloss style wrappers or `JoinHorizontal`/`JoinVertical` nesting that add no layout value; check whether the inner component's own size contract already provides the behavior
7. **Nested conditionals**: nested if/else or nested switch 3+ levels deep; flatten with early returns, guard clauses, a lookup table, or an if/else-if cascade
8. **Unnecessary comments**: comments explaining WHAT the code does (well-named identifiers already do that), narrating the change, or referencing the task/caller; delete these, and keep only non-obvious WHY comments (hidden constraints, subtle invariants, workarounds)

### Agent 3: Efficiency Review

Review the same changes for efficiency:

1. **Unnecessary work**: redundant computations, repeated file reads, duplicate network/API calls, N+1 patterns
2. **Missed concurrency**: independent operations run sequentially when they could run in parallel
3. **Hot-path bloat**: new blocking work added to startup or per-request/per-render hot paths
4. **Recurring no-op updates**: state/store updates inside polling loops, intervals, or event handlers that fire unconditionally; add a change-detection guard so downstream consumers aren't notified when nothing changed. Also, if a wrapper function takes an update/merge callback, verify it honors a no-change signal (a false ok, or a returned value equal to the input); otherwise callers' early-return no-ops are silently defeated.
5. **Unnecessary existence checks**: pre-checking file/resource existence before operating (TOCTOU anti-pattern); operate directly and handle the error
6. **Memory**: unbounded data structures, missing cleanup, goroutine leaks
7. **Overly broad operations**: reading entire files when only a portion is needed, loading all items when filtering for one
8. **Pre-modern Go stdlib idioms not yet caught by `modernize`** (Go diffs only, when `go.mod` is `>= 1.21`): `make check`'s `lint` step runs golangci-lint with the `modernize` linter enabled, which catches most stdlib-modernization candidates mechanically. Spend this pass on the semantic idioms `modernize` does not reach:
   - `sync.Once` + package-level result var that `sync.OnceValue[T]` / `sync.OnceFunc` would collapse to one declaration, where the shape is non-trivial enough that the linter misses it.
   - Push-callback iterators (`func(emit func(T))` shapes, hand-rolled `Next`/`Stop` pairs) with 2 or more call sites: candidates for `iter.Seq[T]` / `iter.Seq2[K,V]`. Single-caller helpers stay.
   - Hand-rolled multi-error accumulation (`var errs []error; … errs = append(errs, err); …`) where `errors.Join(errs...)` fits.
   - Nil/zero-coalescing chains (`if a != "" { return a }; if b != "" { ... }; return c`) where `cmp.Or(a, b, c)` fits.

Surface findings, do not auto-fix. Mirror the precision-over-recall bias of the voice agent: false positives waste apply-phase time.

### Agent 4: Voice Review (Go diffs only)

Skip this agent if the diff has no Go files.

The voice system is Vale plus the `go-conventions` skill: `make check` already runs the Vale
comment gate over Go comment prose, so a mechanical style-guide finding (banned em dash,
labeled comment prefixes, apologetic phrasing, and every other rule Vale's `glw907` overlay
encodes) will surface there and does not need re-scanning here. This agent's job is the
pattern-matching Vale cannot do: whether a comment is worth having at all, and whether the
code around it reads as one experienced Go developer's work rather than a generated one.

The agent must first invoke the `go-conventions` skill and read its comment-standard section
(Go Doc Comments, Effective Go, the write-time comment-or-not gate). Then scan the Go portion
of the diff.

**Primary check: the paraphrase test.** For every in-function comment in the diff, ask: does
this comment paraphrase the next five lines or fewer? If yes, flag it. The code already
states what it does, so a paraphrase comment adds nothing and ages badly. Don't fire on a
legitimate package summary or a contract-bearing godoc that documents behavior the signature
doesn't already convey.

Beyond the paraphrase test, scan for:

1. **Comment tells**: godoc on an unexported symbol where the name already suffices; uniform
   comment density across functions of very different complexity; task-framing comments
   ("added for X flow", "fixes #N"); first-person plural in unexported docs; multi-paragraph
   docstrings on a self-describing function; per-case docstrings in a table test.
2. **Error-phrasing tells**: an error-message chorus that reads identically across adjacent
   sites in one function or file; the function's own name embedded in its error string; a bare
   `%w` wrap where no caller branches on the sentinel.
3. **Naming tells**: package-doubled types (`mail.MailMessage`); over-descriptive locals in a
   tight scope; an exported name that reads like a docstring instead of an identifier.
4. **Structural tells**: a reflexive `doc.go`/`errors.go`/`types.go` skeleton; a single-impl
   interface with no named test-fake or DI seam; a `New<X>` constructor that only sets fields a
   struct literal would set; a defensive nil check between two functions in the same package; a
   length check before an index on an internal caller.
5. **Test tells**: identical assertion phrasing copy-pasted across test files; a tautological
   test case; a subtest wrapping a trivial scalar function that gains nothing from the wrapper.
6. **Voice tells**: uniform sentence length across a whole file; identical paragraph rhythm
   across docs; a `Builder` pattern where a struct literal would suffice.

Output format per finding:

```
<tell name> at path/to/file.go:LINE
  <quoted offending line(s)>
  Fix: <one-line rule from go-conventions>
```

Bias toward precision over recall. False positives on voice waste apply-phase time.

## Phase 3: Triage

Wait for all four agents to complete. Aggregate their findings before deciding which to apply.

### Triage anti-pattern guard

**Before marking any finding as "skip," verify the rationale is not one of:**

- "Cross-package" / "cross-3-files" / "non-trivial refactor"
- "Schema change required"
- "Would require interface change" / "would require backend change"
- "Churn cost" / "out of scope"

**These are forbidden skip rationales when the project's `CLAUDE.md` (or analogous file) establishes a refactor-friendly posture**: pre-beta, alpha, "code quality outweighs stability," "refactor freely," "breaking changes first-class," or similar. In those projects the rationales above describe *exactly* the work the project posture endorses, so they cannot be used to defer.

**The only valid skip rationales are:**

1. **"Speculative future consumer"**: the finding adds a field, type, or hook for a consumer that doesn't yet exist and isn't immediately needed. Read the code: if no current call site benefits, skip is fair.
2. **"Upstream-blocked"**: the finding requires a change to a third-party dependency that the project doesn't control or vendor.
3. **"Premature optimization without measurement"**: for efficiency findings only, where the hot path hasn't been profiled and the current shape is bounded.

If none of those three apply, the finding must be applied in this pass.

When you've decided what to apply, state the apply/skip split explicitly (one line per skip with its rationale tagged) and then proceed to Phase 4.

## Phase 4: Fix Issues

Apply each finding directly. When done, briefly summarize what was fixed (or confirm the code was already clean).
