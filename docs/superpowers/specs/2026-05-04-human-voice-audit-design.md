# Pass 8.8 — Human-Voice Audit (Design)

**Date:** 2026-05-04
**Phase slot:** Pre-beta. Inserted between Pass 10 (Polish II) and
Pass 11 (v0.9.0 prep) — the final hygiene sweep before feature
freeze.
**Author:** brainstorming session, 2026-05-04.

## Goal

Make poplar's Go code read as if a single experienced human Go
developer wrote it. Strip the rhythmic and structural fingerprints
that mark code as AI-generated, so external contributors don't
recognize the codebase as model output and disengage before reading
it.

The threat model is contributor recruitment. AI-generated Go has a
recognizable shape — uniform error wrapping, defensive nil checks
on internal callers, single-impl interfaces, godoc on every
unexported symbol, identical comment density across files. Readers
who have seen enough of it pattern-match instantly and treat the
code as low-trust. Poplar wants to invite contribution at v0.9.0;
this pass removes the surface markers that would prevent that.

The forward-enforcement counterpart — `CLAUDE.md`'s "Human voice"
section — landed in the design session that produced this spec.
This pass is the catch-up sweep for what slipped through before
those rules existed.

## Non-goals

- Behavior changes beyond what edits force. No feature work.
- Performance tuning. The audit's lens is voice, not speed.
- Vendored snippets in `internal/mailauth/`. Provenance-marked,
  license-bound, not ours to restyle.
- Generated files (none currently; policy is explicit).
- Re-litigating ADR'd decisions. If a single-impl interface exists
  because an ADR named a real seam, it stays.
- Test-coverage reduction. Tautological cases go; substantive
  cases stay even if their phrasing is uniform.

## Scope (in)

All Go source under `cmd/poplar/` and `internal/` except
`internal/mailauth/`. Tests included where they encode AI tells
(per-case docstrings, tautological cases, identical assertion
phrasing across files).

Documentation, ADRs, and `docs/poplar/invariants.md` are touched
only insofar as they reference renamed symbols.

## Pass position

| Pass  | Goal                                         | Status   |
|-------|----------------------------------------------|----------|
| 8.7   | Attachments II — viewer (#24)                | pending  |
| 9     | Compose framing — Editor interface, SMTP     | pending  |
| 9.5   | Compose enhancements                         | pending  |
| 9.6   | First-run wizard + config template           | pending  |
| 10    | Polish II — popover dim (#14)                | pending  |
| 8.8  | **Human-voice audit I — string-only**        | active   |
| 8.9  | **Human-voice audit II — structural**        | pending  |
| 11    | v0.9.0 prep — feature freeze, docs, tag      | pending  |

Position rationale: running last in the feature window means the
audit covers all code Passes 9–10 introduce. Running before Pass 11
means the freeze tag captures clean code. CLAUDE.md's "Human voice"
section already enforces forward during 8.7–10, so the volume the
8.8 sweep has to address should shrink as those passes ship.

## Categories of "tell"

Each finding in Phase 1 is tagged with one of these categories.
Triage in Phase 2 uses the category to decide apply / keep /
taste-call.

### C1 — Comment rot

- WHAT-comments (restating what the next line obviously does).
- Godoc on unexported symbols where the doc adds no information
  beyond the name.
- Multi-line comment blocks above functions where one line, or
  none, would do.
- Hedges: `// for now`, `// TODO`, `// note:`.
- Task framing: `// added for the X flow`, `// used by Y`,
  `// fixes issue #N`.

### C2 — Defensive cruft

- Nil checks on values returned by internal constructors (a nil
  return would be a bug, and the receiver would panic anyway).
- `if err != nil { return fmt.Errorf("failed to X: %w", err) }`
  where the caller already has full context — bare `return err`
  reads more naturally and matches Go idiom.
- Validation between two functions in the same package, neither
  of which is a system boundary.
- Length checks before indexing where Go's runtime panic would be
  the better signal.

### C3 — Premature abstraction

- Single-implementation interfaces with no test fake, no DI seam,
  no ADR. (Real seams are kept and noted in code.)
- Helper wrappers that save zero lines.
- `Manager` / `Helper` / `Util` / `Service` types that wrap a
  single field.
- Builder patterns where a struct literal would suffice.
- `New<X>` constructors that only set fields a literal would set.

### C4 — Uniform verbosity

- Every error in a file wrapped with the same template.
- Every function carrying the same shape of doc comment.
- Every package laid out as `doc.go` / `errors.go` / `types.go`
  reflexively.
- Identical length distribution across functions in a file (real
  human files are uneven).

### C5 — Naming tells

- Overly descriptive locals in tight scopes (`messageList ml`
  three lines from a `range` whose loop variable is the obvious
  name).
- `GetX` getter prefix (Go convention is `X()`).
- Package-doubled types: `mail.MailMessage`, `cache.CacheEntry`.
- `Manager` / `Helper` / `Util` / `Service` suffixes.
- Exported names that read like docstrings:
  `ProcessIncomingMessageWithRetries`.

### C6 — Test boilerplate

- Per-case docstrings explaining what each table case tests.
- Tautological cases: empty input → empty output against a
  function that returns its argument unchanged.
- Identical assertion phrasing copy-pasted across files.
- Case `name` fields written as full sentences when a 2-3-word
  label would do.
- Subtests for trivial scalar functions where one assertion in
  the parent test would suffice.

### C7 — Error phrasing

- `fmt.Errorf("failed to X: %w", err)` everywhere — the loudest
  AI signature in Go. Real Go varies: `"open db"`, `"resolve %q"`,
  bare nouns, sometimes bare `err`.
- Adjacent error sites that read identically when they handle
  genuinely different failures.
- Sentinel-wrapping at points where the caller doesn't branch on
  the sentinel.

### C8 — Structural symmetry

- Reflexive `doc.go` files with no real package doc.
- Every package having `errors.go` even when it has one error.
- Identical file naming patterns across packages with very
  different responsibilities.
- Mirror-imaged subdirectory structure that doesn't reflect
  actual coupling.

## Phases

### Phase 1 — Audit (read-only)

Walk `cmd/poplar/` + `internal/` (minus `internal/mailauth/`)
package-by-package. Dispatch parallel `Explore` subagents — one per
package — each returning a categorized findings file under
`docs/poplar/audits/2026-05-04-pkg-<name>.md`. Each finding records:

- File and line number(s).
- Category (C1–C8).
- Verbatim excerpt of the code (≤6 lines).
- One-line diagnosis.

Subagents return findings only — no edits, no triage, no
recommendations beyond the category tag. Main thread aggregates
into `docs/poplar/audits/2026-05-04-human-voice.md`.

The main thread also performs a **cross-cutting pass** that no
single-package agent can do: scanning for repeated error-phrasing
templates across packages, identical comment shapes across files,
identical test-case phrasing across packages. Cross-cutting findings
live in `docs/poplar/audits/2026-05-04-cross-cutting.md`.

### Phase 2 — Triage

Single-reviewer pass over the aggregated findings. Each finding
gets `apply` / `keep` / `taste-call`.

- **apply** — clear AI tell, fix is obvious.
- **keep** — surface-level tell that's actually justified (real
  seam, real boundary validation, real test coverage). Annotate
  with reason.
- **taste-call** — reasonable people disagree. Resolved here, not
  during Phase 3.

Spec is **frozen** at end of Phase 2. Plan doc carries the
applied findings as ordered task lists, grouped by category not
by file.

Skip-rationale guard mirrors the `/simplify` rule: never defer a
finding with framings like "cross-package," "non-trivial refactor,"
"churn cost," "out of scope." Pre-beta posture endorses all of
those.

### Phase 3 — Apply in category batches (split across 8.8 and 8.9)

The apply phase splits across two passes by risk profile. Triage
in Phase 2 tags each `apply` finding with its target pass:

**Pass 8.8 — string-only (Phase 3a):**
1. **C1 — Comments.** Lowest-risk; deletions and one-line edits.
2. **C7 — Error phrasing.** String-only edits, no signature
   changes; reviewer-friendly diff.
3. **C4 — Uniform verbosity (prose only).** Doc-comment-shape
   variation, error-shape variation. Often subsumed by C1 + C7.
   Function-length-distribution findings defer to 8.9.

**Pass 8.9 — structural (Phase 3b):**
4. **C2 — Defensive cruft.** Removes code; small risk that a check
   was load-bearing — `make check` catches.
5. **C6 — Test boilerplate.** Test-only; doesn't touch production.
   Coverage-guard rule applies.
6. **C5 — Naming.** Renames; ripple-prone, run `make check`
   between each rename batch. Live tmux render after.
7. **C3 — Premature abstraction.** Inlines and deletes;
   structural. Real-seam guard before any interface inline.
8. **C8 — Structural symmetry.** Most invasive; file moves and
   deletions. Last so prior commits are reviewable in isolation.

Rationale for the split:

- **Risk profile differs sharply.** String-only edits validate
  with `make check` alone; structural edits need tmux render and
  benefit from soak time on the style guide.
- **Style-guide validation.** 8.8's narrow scope is the lowest-
  risk way to discover whether the guide actually says what we
  meant. If 8.9's tells reveal a guide gap, we revise the guide
  before applying renames at scale.
- **Reviewer pace.** Bundling forces lowest-common-denominator
  review; splitting lets each pass merge promptly.
- **Acceptable cost.** Pre-beta posture allows two passes — no
  shipping cost, just sequencing. Pass-end ritual overhead doubles
  (two ADRs at most, two STATUS rows), bounded.

`make check` between each commit. `/simplify` before the final
commit per global rule.

### Phase 4 — Verify

- `make check` passes on every commit (gate).
- Live UI verification (tmux, 80×24 + 120×40) after C5 and C3 —
  any rename/inline that touches a renderer-visible field could
  break rendering.
- Manual read-through of three randomly-selected files to confirm
  the voice changed. If a file still feels machine-uniform after
  the pass, log a follow-up; don't extend Phase 3.

### Phase 5 — Codify forward

- **ADR-0141** — Human-voice policy. Concrete examples drawn from
  real audit findings, not abstract rules. Referenced from
  CLAUDE.md and `go-conventions` skill.
- **`go-conventions` skill update** — append a "Human voice"
  section with the same C1–C8 categories, each with one
  before/after example from the codebase. Skill is global, so this
  benefits other Go projects too.
- **`/simplify` integration** — the skill currently runs reuse /
  quality / efficiency lenses. Add a fourth lens: voice. Same
  parallel-agent shape; one agent dedicated to scanning the diff
  for C1–C8 patterns.
- **invariants.md decision-index update** — new row for Pass 8.8
  / ADR-0141.
- **Optional: `/human-voice` slash command.** Standalone audit on
  a diff or path. Defer to Phase 5 only if `/simplify` integration
  proves insufficient. Likely unnecessary.

## Risks and tradeoffs

**Diff size.** Will touch most files. Acceptable pre-beta. Cannot
land post-1.0; this is a hard deadline against Pass 11.

**Subjective findings.** ~30% of findings won't have a clean
answer. Triage in Phase 2 forces a decision before Phase 3 begins;
no taste-calls during apply.

**Test coverage erosion.** Stripping "obvious" cases is dangerous
when the case is actually load-bearing. Default to keeping unless
the case is literally tautological. C6 reviewers re-run tests with
the case removed AND with it kept; if behavior diverges, the case
stays.

**UI regression risk.** Renames in C5 and inlines in C3 can break
renderers that read field names indirectly. Live tmux verification
after both batches catches this; `make check` doesn't.

**Forward-enforcement gap.** If ADR-0141 + skill update + simplify
integration don't land, the next AI-assisted pass re-introduces
the tells. Phase 5 is mandatory, not optional polish.

**False-positive on real seams.** Some single-impl interfaces are
correct (`mail.Backend`, `mail.ChangeTracker` — both have JMAP and
IMAP impls; cache uses them through the interface). The audit
must not strip these. Phase 2 triage gates this.

## Acceptance

- All eight category batches committed, `make check` green.
- `docs/poplar/audits/2026-05-04-human-voice.md` archived.
- ADR-0141 written.
- `go-conventions` skill updated with concrete examples.
- `/simplify` voice lens added.
- `invariants.md` decision index updated.
- A read-through of three randomly-selected files reads as
  recognizably human.
